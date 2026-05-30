package parser

import (
	"fmt"
	"strings"
	"time"
)

// EditKind describes what part of the file was affected by an edit.
type EditKind int

// EditKind classifies where an edit occurred.
const (
	EditInFunc EditKind = iota // Edit was inside a function body
	EditGlobal                 // Edit was outside all function bodies (requires re-parse of signatures)
	EditFull                   // Full document replacement
)

// ApplyEdit updates the ParseResult incrementally after a text edit.
// It updates the content, shifts line numbers, and re-parses only what's needed.
// Returns the EditKind indicating what was affected.
func (pr *ParseResult) ApplyEdit(startLine, startChar, endLine, endChar int, newText string) (kind EditKind) {
	defer func() {
		if r := recover(); r != nil {
			pr.logWarn("parse panic in ApplyEdit", "uri", string(pr.URI), "error", fmt.Sprint(r))
			pr.reparseShallow()

			kind = EditGlobal
		}
	}()

	start := time.Now()

	oldLines := endLine - startLine
	newLines := strings.Count(newText, "\n")
	delta := newLines - oldLines

	// Determine if edit is inside a function body (before modifying content)
	funcIdx := pr.funcContaining(startLine, endLine, startChar)

	// Apply the text edit
	startOff := posOffset(pr.Content, startLine, startChar)
	endOff := posOffset(pr.Content, endLine, endChar)
	pr.Content = pr.Content[:startOff] + newText + pr.Content[endOff:]

	if funcIdx >= 0 {
		// Edit is inside a function body — just shift and invalidate
		pr.shiftAfter(startLine, delta)
		pr.InvalidateFunc(pr.Scopes[funcIdx].Start, pr.Scopes[funcIdx].End)
		pr.resetGlobalCaches()
		pr.logDebug("applyEdit", "uri", string(pr.URI), "kind", "inFunc", "line", startLine, "funcStart", pr.Scopes[funcIdx].Start, "funcEnd", pr.Scopes[funcIdx].End, "dur", time.Since(start))

		return EditInFunc
	}

	// Edit is in global scope — need to re-parse signatures
	pr.reparseShallow()
	pr.logDebug("applyEdit", "uri", string(pr.URI), "kind", "global", "dur", time.Since(start))

	return EditGlobal
}

// ApplyFullReplace replaces the entire content and re-parses shallowly.
func (pr *ParseResult) ApplyFullReplace(content string) {
	pr.Content = content
	pr.reparseShallow()
}

// funcContaining returns the index into pr.Scopes of the function whose body
// contains the edit range, or -1 if the edit spans outside all functions.
func (pr *ParseResult) funcContaining(startLine, endLine, startChar int) int {
	for i, s := range pr.Scopes {
		if startLine > s.Start && endLine < s.End {
			return i
		}
		// Edit is on the closing line — check if it's before the closing tag/brace
		if startLine > s.Start && endLine == s.End {
			closeCol := pr.closingTokenCol(s.End)
			if closeCol >= 0 && startChar <= closeCol {
				return i
			}
		}
	}

	return -1
}

// closingTokenCol returns the column of the closing token (} or </cffunction>)
// on the given line, or -1 if not found.
func (pr *ParseResult) closingTokenCol(line int) int {
	lineStart := 0
	for range line {
		idx := strings.IndexByte(pr.Content[lineStart:], '\n')
		if idx < 0 {
			return -1
		}

		lineStart += idx + 1
	}

	lineEnd := strings.IndexByte(pr.Content[lineStart:], '\n')
	if lineEnd < 0 {
		lineEnd = len(pr.Content) - lineStart
	}

	lineText := pr.Content[lineStart : lineStart+lineEnd]

	// Check for </cffunction (tag-based)
	if idx := indexCFTag(lineText, "/cffunction"); idx >= 0 {
		return idx
	}
	// Check for closing brace (script-based)
	if idx := strings.LastIndex(lineText, "}"); idx >= 0 {
		return idx
	}

	return -1
}

// shiftAfter adjusts line numbers for scopes, funcs, and refs after editLine.
func (pr *ParseResult) shiftAfter(editLine, delta int) {
	if delta == 0 {
		return
	}

	for i := range pr.Scopes {
		if pr.Scopes[i].Start > editLine {
			pr.Scopes[i].Start += delta
			pr.Scopes[i].End += delta
		} else if pr.Scopes[i].End >= editLine {
			pr.Scopes[i].End += delta
		}
	}

	for i := range pr.Funcs {
		if int(pr.Funcs[i].Line) > editLine {
			pr.Funcs[i].Line = uint32(int(pr.Funcs[i].Line) + delta)
		}
	}

	for i := range pr.Refs {
		if int(pr.Refs[i].Line) > editLine {
			pr.Refs[i].Line = uint32(int(pr.Refs[i].Line) + delta)
		}
	}
}

// reparseShallow re-runs the shallow parse on the current content,
// replacing Funcs, Refs, Scopes, and Regions. Clears all caches.
func (pr *ParseResult) reparseShallow() {
	start := time.Now()
	pr.Regions = ClassifyRegions(pr.Content)
	pr.Funcs = pr.Funcs[:0]
	pr.Refs = pr.Refs[:0]
	pr.Scopes = pr.Scopes[:0]
	pr.extractSignatures()
	pr.resetGlobalCaches()
	pr.funcVarsMu.Lock()
	pr.funcVars = make(map[string][]string)
	pr.funcVarsMu.Unlock()
	pr.logDebug("reparseShallow", "uri", string(pr.URI), "funcs", len(pr.Funcs), "dur", time.Since(start))
}

// resetGlobalCaches resets the lazily-computed global/variables/this var caches.
func (pr *ParseResult) resetGlobalCaches() {
	pr.mu.Lock()
	pr.globalDone = false
	pr.varsDone = false
	pr.thisDone = false
	pr.globalVars = nil
	pr.variablesVars = nil
	pr.thisVars = nil
	pr.mu.Unlock()
}

// posOffset converts line/char to byte offset.
func posOffset(content string, line, char int) int {
	off := 0
	for range line {
		idx := strings.IndexByte(content[off:], '\n')
		if idx < 0 {
			return len(content)
		}

		off += idx + 1
	}

	off += char
	if off > len(content) {
		off = len(content)
	}

	return off
}
