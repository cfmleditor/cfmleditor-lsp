package server

import (
	"context"
	"strings"
	"time"

	"encoding/json/v2"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/textdiff"
	"go.lsp.dev/protocol"
)

// handleRangeFormatting answers textDocument/rangeFormatting — the editor's
// "Format Selection".
//
// It formats the *whole* document and then returns only the edits that fall
// inside the requested lines. Formatting the selected text on its own would be
// the obvious approach and is the wrong one twice over: a selection rarely
// parses standalone (half a function, a tag whose close is outside it), and
// even when it does, its indentation depends on everything enclosing it. Going
// through the whole document instead makes the result identical, line for line,
// to what format-on-save would have produced — so formatting a selection can
// never disagree with formatting the file, and repeated selections converge
// rather than fight.
//
// The cost is that a document the grammar cannot parse is refused even when the
// selection itself is clean. That is the same refusal whole-document formatting
// makes, and for the same reason: the formatter has no rendering for an ERROR
// node.
func (s *Server) handleRangeFormatting(ctx context.Context, rawParams []byte) (any, error) {
	// Two gates, not one: formatting.enabled governs the formatter as a whole,
	// and features.rangeFormatting only this half of it. Without the second,
	// stopping a range-formatting defect means giving up format-on-save too.
	if !s.Formatting.Enabled || !s.Features.RangeFormatting {
		return nil, nil
	}

	var params protocol.DocumentRangeFormattingParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	docURI := params.TextDocument.URI

	content, ok := s.getDocument(docURI)
	if !ok {
		return nil, nil
	}

	first, last := lineSpan(params.Range)
	s.log.Info("formatting range", cflog.String("uri", string(docURI)), cflog.Int("firstLine", first), cflog.Int("lastLine", last))

	// As handleFormatting: the text and settings are all it needs.
	cfg := s.Formatting

	releaseReadLoop(ctx)

	start := time.Now()
	formatted, err := formatDocument(content, params.Options, cfg)
	elapsed := time.Since(start)

	if err != nil {
		// Deliberately quieter than the whole-document handler, which pops a
		// warning. "Format Selection" is often reached by keybinding while the
		// file is mid-edit and does not parse, and a modal-ish warning on every
		// keystroke-adjacent invocation is worse than doing nothing visible.
		s.log.Warn("range formatting failed", cflog.String("uri", string(docURI)), cflog.Duration("elapsed", elapsed), cflog.Err(err))

		return []protocol.TextEdit{}, nil
	}

	edits := rangeEdits(content, formatted, first, last)
	s.log.Debug("range formatting complete",
		cflog.String("uri", string(docURI)),
		cflog.Int("edits", len(edits)),
		cflog.Duration("elapsed", elapsed))

	return edits, nil
}

// lineSpan converts a requested range into the inclusive line numbers it
// covers. Formatting is line-oriented — the formatter rebuilds whole lines — so
// a selection that starts or ends mid-line takes those lines whole, which is
// what the specification tells a server to do.
//
// The one subtlety is an end at character 0: selecting three whole lines in an
// editor produces an end position at the start of the *fourth*, and taking that
// line whole would reformat a line the user did not select.
func lineSpan(r protocol.Range) (first, last int) {
	first = int(r.Start.Line)
	last = int(r.End.Line)

	if r.End.Character == 0 && last > first {
		last--
	}

	return first, last
}

// rangeEdits returns the edits that bring lines [first,last] of content into
// line with formatted, and no others.
//
// Lines are matched on their trimmed text. Matching raw lines would be simpler
// and much worse: re-indenting a block changes every raw line in it, leaving no
// common line for the diff to anchor on, and the whole block would come back as
// one hunk that a partial selection could only over-apply. Trimmed, a
// re-indented line still matches itself, the diff sees only the lines the
// formatter genuinely added or removed, and the indentation change is emitted
// per line.
func rangeEdits(content, formatted string, first, last int) []protocol.TextEdit {
	if formatted == content {
		return []protocol.TextEdit{}
	}

	src := strings.Split(content, "\n")
	dst := strings.Split(formatted, "\n")

	srcKeys := trimmedLines(src)
	dstKeys := trimmedLines(dst)

	var candidates []lineEdit

	// Structural changes first: lines the formatter added or removed. Between
	// two hunks the lines are key-equal and line up one to one, so the gaps are
	// walked in step and any raw difference there — the reindentation, the
	// requoting, the tag casing — becomes a single-line edit.
	srcPos, dstPos := 0, 0

	emitGap := func(srcEnd, dstEnd int) {
		for srcPos < srcEnd && dstPos < dstEnd {
			if src[srcPos] != dst[dstPos] {
				candidates = append(candidates, lineEdit{aStart: srcPos, aEnd: srcPos + 1, lines: dst[dstPos : dstPos+1]})
			}

			srcPos++
			dstPos++
		}
	}

	for _, h := range textdiff.Hunks(srcKeys, dstKeys) {
		emitGap(h.AStart, h.BStart)
		candidates = append(candidates, lineEdit{aStart: h.AStart, aEnd: h.AEnd, lines: dst[h.BStart:h.BEnd]})
		srcPos, dstPos = h.AEnd, h.BEnd
	}

	emitGap(len(src), len(dst))

	return textEdits(coalesce(inRange(candidates, first, last, len(src))))
}

// lineEdit replaces source lines [aStart,aEnd) with lines. An empty source
// range is an insertion before aStart.
//
// lines usually points into the formatted document rather than owning storage,
// which is what owned records: coalesce may not append to a borrowed slice,
// since doing so writes over the formatted lines that follow.
type lineEdit struct {
	aStart, aEnd int
	lines        []string
	owned        bool
}

// inRange keeps the edits that belong to the selected lines.
//
// An edit is kept only when it lies *entirely* within the selection. Overlap is
// not enough, because a formatter change is not always divisible: joining a
// five-line `<cfif ... >` header into one line is a single hunk covering all
// five, and there is no half of it to apply. Keeping such a hunk because it
// happens to reach into the selection would rewrite lines the user did not
// select — which is the one thing "Format Selection" must not do. Corpus
// measurement found 1,745 files where the overlap rule did exactly that.
//
// So the guarantee is: every change that fits inside the selection is applied,
// and a change straddling its edge is left alone. Widening the selection, or
// formatting the document, picks the straddling one up.
//
// An insertion modifies no existing line, so it only has to sit at a position
// inside the selection. The exception is the position one past the last line:
// that counts only at the end of the document, so a selection running to EOF
// still receives the trailing lines the formatter appends, with no special case
// for the whole-document request.
func inRange(candidates []lineEdit, first, last, nLines int) []lineEdit {
	kept := make([]lineEdit, 0, len(candidates))

	for _, c := range candidates {
		var keep bool
		if c.aStart == c.aEnd {
			keep = c.aStart >= first && (c.aStart <= last || (c.aStart == last+1 && c.aStart >= nLines))
		} else {
			keep = c.aStart >= first && c.aEnd <= last+1
		}

		if keep {
			kept = append(kept, c)
		}
	}

	return kept
}

// coalesce merges edits that touch.
//
// Not cosmetic: the walk above naturally produces an insertion and a
// replacement at the same source position — a blank line added before a line
// that is also being reindented — and the protocol leaves the order of two
// edits sharing a position undefined, so a client may apply them either way
// round. Merging touching edits removes the ambiguity, and there is nothing
// between them to preserve.
func coalesce(edits []lineEdit) []lineEdit {
	merged := make([]lineEdit, 0, len(edits))

	for _, e := range edits {
		n := len(merged)
		if n == 0 || merged[n-1].aEnd != e.aStart {
			merged = append(merged, e)

			continue
		}

		prev := &merged[n-1]

		// Copy once, on the first merge into this edit, then append in place.
		// Re-copying per merge is the obvious spelling and is quadratic: a
		// whole-document format merges every changed line into one edit, so a
		// large file copied the whole accumulated slice thousands of times.
		// Measured on the corpus, that reached 13.9 GB and was killed.
		if !prev.owned {
			prev.lines = append(make([]string, 0, len(prev.lines)+len(e.lines)), prev.lines...)
			prev.owned = true
		}

		prev.lines = append(prev.lines, e.lines...)
		prev.aEnd = e.aEnd
	}

	return merged
}

func textEdits(edits []lineEdit) []protocol.TextEdit {
	out := make([]protocol.TextEdit, 0, len(edits))

	for _, e := range edits {
		text := ""
		if len(e.lines) > 0 {
			text = strings.Join(e.lines, "\n") + "\n"
		}

		out = append(out, protocol.TextEdit{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(e.aStart), Character: 0},
				End:   protocol.Position{Line: uint32(e.aEnd), Character: 0},
			},
			NewText: text,
		})
	}

	return out
}

func trimmedLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.TrimSpace(l)
	}

	return out
}
