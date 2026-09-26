package parser

import (
	"strings"
)

// tagParser extracts definitions from CFML tag-based source.
type tagParser struct {
	src           string
	fileURI       string
	funcs         []FunctionDef
	vars          []VarDef
	componentRefs []ComponentRef
	funcRefs      map[string][]ComponentRef // keyed by "start:end"
	funcLinks     map[string][]DocumentLink // keyed by "start:end"
	funcCalls     map[string][]CallSite     // keyed by "start:end"
	links         []DocumentLink            // global scope links
	calls         []CallSite                // global scope call sites
	scopes        []FuncScope
	pendingCalls  []pendingCall
	properties    []propertyDef
	extends       string
	persistent    bool
	lineIndex     []int32 // byte offset of each line start
	resolvers     []Resolver
	resolverSet   *ResolverSet
	extractLinks  bool // whether to extract document links
	extractCalls  bool // whether to extract all call sites
	// gated limits #...# scanning of text to outputSpans, the ranges of the
	// file ColdFusion evaluates; srcOffset is this region's offset in it. The
	// attributes of CF tags and of importPrefixes custom tags are scanned
	// wherever they stand. See outputContext.
	gated               bool
	outputSpans         [][2]int
	importPrefixes      []string
	srcOffset           int
	builtinReturnLookup func(string) string
	inFunc              string // current function scope key ("start:end"), empty if global
	// localVars holds the var'd/local. names declared in the function being
	// parsed. A slice scanned with EqualFold rather than a map of lowercased
	// keys: a function declares a handful of locals, so the scan is short, and
	// the map cost both its buckets and a strings.ToLower per insert and per
	// lookup — together 7% of everything a tag parse allocated.
	localVars   []string
	forceGlobal bool // when true, addRef routes to componentRefs regardless of inFunc

	// baseLine is this region's absolute start line (0 if parsing the whole
	// file as one region). knownScopes, when set by the caller, gives the
	// true absolute (Start, End) of every tag function in the WHOLE file —
	// computed once up front via findTagFuncScopes(pr.Content, 0), which is
	// unaffected by region splitting. Used so a function whose body is
	// interrupted by a nested <cfscript> block (splitting the file into
	// separate regions here) still resolves its real end line, instead of
	// only whatever "/cffunction" this region's own limited text happens to
	// contain.
	baseLine    int
	knownScopes []FuncScope
}

func newTagParser(src, fileURI string) *tagParser {
	p := &tagParser{src: src, fileURI: fileURI}
	p.buildLineIndex()

	return p
}

func (p *tagParser) buildLineIndex() {
	p.lineIndex = buildLineIdx(p.src)
}

// lineAt returns the 0-based line number for a byte offset using binary search.
func (p *tagParser) lineAt(offset int) int {
	lo, hi := 0, len(p.lineIndex)
	for lo < hi {
		mid := (lo + hi) / 2
		if int(p.lineIndex[mid]) <= offset {
			lo = mid + 1
		} else {
			hi = mid
		}
	}

	return lo - 1
}

// looksLikeCallSpan reports whether the text between two hashes is worth
// treating as opaque: a CFML expression holding a call, rather than two
// unrelated hashes that happened to pair.
//
// Two tests, and each is here for a shape the corpus produced:
//
//   - a `(`, because a call is the only thing a span contributes to the tag
//     walk and scanInterpolatedText rejects the rest on the same byte scan;
//   - balanced quotes, because a lone `#` inside a quoted string is how an
//     embedded page writes a jQuery id selector — `$( '#search' ).typeahead(`
//     has both a quote and a paren, and pairing its hash with the next one in
//     the file swallowed four tags of Lucee's doc pages. A CSS `#01798A` needs
//     no test of its own: it has no paren.
func looksLikeCallSpan(body string) bool {
	if strings.IndexByte(body, '(') < 0 {
		return false
	}

	return strings.Count(body, `"`)%2 == 0 && strings.Count(body, "'")%2 == 0
}

// tagEndIndex finds the '>' that closes a tag, skipping over quoted strings.
//
// A bare IndexByte stops at the first '>' anywhere, and a CF tag holds an
// expression that may contain one:
//
//	<cfset fields = array( field( "Host:Port&lt;new line><br>" ), field( "b" ) )>
//
// cut there, the second `field` is in no tag and in no text the walk scans, so
// it is recorded nowhere. Lucee's cache-driver components are written this way.
//
// It falls back to the plain scan when a quote never closes, so a malformed tag
// cannot swallow the rest of the file — the tag is read exactly as it was
// before, which is the behaviour this replaces.
func tagEndIndex(s string) int {
	// The bulk of the scan stays an IndexByte: a quote before the next '>' is
	// what makes a false end possible, and only then is there anything to skip.
	// A byte-at-a-time version of this cost 9% of a tag parse — it ran over
	// every tag in the file, most of which hold no quote at all.
	gt := strings.IndexByte(s, '>')
	if gt < 0 {
		return -1
	}

	// A '>' with every quote before it already closed is the tag's end, and
	// two counts settle that for the whole attribute list at once. The walk
	// below costs three calls per attribute instead, on short strings where
	// the call is most of the cost — and a tag whose attributes are all
	// ordinarily quoted is nearly every tag in a file. Parity is exact rather
	// than a heuristic: CFML escapes a quote by doubling it, which adds two.
	if strings.Count(s[:gt], `"`)%2 == 0 && strings.Count(s[:gt], "'")%2 == 0 {
		return gt
	}

	return tagEndWalk(s)
}

// tagEndWalk is the quote-by-quote scan tagEndIndex falls back to. It is a
// function of its own so the fast path above can be compared against it
// directly — restating it in a test would be the parallel list this codebase
// keeps warning about.
func tagEndWalk(s string) int {
	for pos := 0; ; {
		gt := strings.IndexByte(s[pos:], '>')
		if gt < 0 {
			return -1
		}

		end := pos + gt

		q := indexQuote(s[pos:end])
		if q < 0 {
			return end
		}

		j := skipQuotedIn(s, pos+q)
		if j < 0 {
			return strings.IndexByte(s, '>')
		}

		pos = j + 1
	}
}

// nextTagStart finds the next '<' that could open a tag, stepping over a
// `#...#` span rather than through it.
//
// Markup written inside a span is an argument, not a tag:
//
//	#ETH.author( content = "<strong>@name@</strong>" )#
//
// Splitting there hands scanInterpolatedText a chunk with an unterminated `#`,
// so the call is lost — and the hashes left over mis-pair with the next span,
// so the call after it is lost too.
//
// Two rules, and getting either wrong loses tags rather than finding calls:
//
//   - **A span's hashes pair whether or not its contents are stepped over.** A
//     first version advanced only past the *opening* hash of a span it declined,
//     so the closing hash became the next opening one and everything after was
//     inverted: `#local.iconType#` left its second hash to pair with the `#`
//     three lines later, and the `</i><cfif isSimpleValue( … )>` between them
//     was swallowed. That alone was 141 files.
//   - **A span holding no `(` does not hide a tag.** Hashes interpolate inside
//     <cfoutput> and in a tag's attributes and this walk tracks neither, so in
//     prose a `#` is just a character and `item #1 <b>x</b> #2` must keep its
//     markup. A call is the only thing a span contributes here —
//     scanInterpolatedText rejects the rest on the same byte scan — so a span
//     with no `(` still pairs, but a `<` inside it is returned as a tag.
func nextTagStart(s string) int {
	// The bulk of the scan stays an IndexByte. A '#' before the next '<' is
	// what makes a span possible, and only then is there anything to resolve —
	// on a file with no interpolation this is the same two byte scans the walk
	// always did.
	//
	// Memoising the '#' scan across calls — one scan per hash rather than one
	// per tag — is the obvious next step and was measured: it changed nothing
	// on the tag benchmarks, because what this costs is the '<' scan's call
	// overhead on short strings, not the hash scan.
	for pos := 0; ; {
		lt := strings.IndexByte(s[pos:], '<')

		end := len(s)
		if lt >= 0 {
			end = pos + lt
		}

		h := strings.IndexByte(s[pos:end], '#')
		if h < 0 {
			if lt < 0 {
				return -1
			}

			return pos + lt
		}

		at := pos + h

		// `##` is CFML's escaped hash and opens nothing.
		if at+1 < len(s) && s[at+1] == '#' {
			pos = at + 2

			continue
		}

		// The next '#', not matchingHash: the two scans legitimately differ,
		// because they optimise different errors. A wrong span here costs a
		// *tag* — everything between the hashes stops being markup — so this
		// one stays conservative and pairs hashes as they come. A wrong span
		// in interpolatedSpans costs at most a call, so that one follows CFML's
		// nesting through quoted strings. Making this share that rule fixed one
		// corpus site and broke thirty-six: skipping quoted strings runs a span
		// much further in markup, and Lucee's doc pages lost their tags to it.
		rel := strings.IndexByte(s[at+1:], '#')
		if rel < 0 {
			// Unterminated: the chunk is a fragment of some tag's attribute
			// list, and the '#' is just a byte.
			pos = at + 1

			continue
		}

		spanEnd := at + 1 + rel

		body := s[at+1 : spanEnd]
		if !looksLikeCallSpan(body) {
			if i := strings.IndexByte(body, '<'); i >= 0 {
				return at + 1 + i
			}
		}

		pos = spanEnd + 1
	}
}

// parse scans through tag-based CFML extracting definitions.
func (p *tagParser) parse() {
	pos := 0

	for pos < len(p.src) {
		// Find next < that could be a CF tag
		idx := p.nextTag(pos)
		if idx < 0 {
			// Trailing text, which is also where the attribute list of the last
			// declined tag in the file ends up.
			if p.textEvaluated(pos) {
				p.scanInterpolatedText(p.src[pos:], pos)
			}

			break
		}

		idx += pos

		// Everything between the previous tag and this one is text, and in a
		// tag file that is where #...# interpolation lives: a <cfoutput> body,
		// and the attribute list of any tag the fast skip below declines (which
		// becomes text before the *next* tag). Scanning it here rather than
		// after the walk is what keeps p.inFunc live, so a call inside a
		// function's <cfoutput> is filed against that function.
		//
		// Gated, it is scanned only where ColdFusion evaluates text. Output
		// contexts open and close at tags, so a gap lies wholly inside one or
		// wholly outside; and the attributes of a declined CF or custom tag
		// are scanned below and stepped over, so a gap holds only text and
		// HTML attributes, which are evaluated only inside an output context.
		if p.textEvaluated(pos) {
			p.scanInterpolatedText(p.src[pos:idx], pos)
		}

		// Skip CFML comments (nested)
		if idx+4 < len(p.src) && p.src[idx:idx+5] == "<!---" {
			depth := 1
			j := idx + 5

			for j < len(p.src) && depth > 0 {
				switch {
				case j+4 < len(p.src) && p.src[j:j+5] == "<!---":
					depth++
					j += 5
				case j+3 < len(p.src) && p.src[j:j+4] == "--->":
					depth--
					j += 4
				default:
					j++
				}
			}

			pos = j

			continue
		}

		// Check for cfscript block
		if idx+10 <= len(p.src) && strings.EqualFold(p.src[idx:idx+10], "<cfscript>") {
			bodyStart := idx + 10
			closeIdx := indexCFTag(p.src[bodyStart:], "/cfscript>")

			var bodyEnd int

			if closeIdx < 0 {
				bodyEnd = len(p.src)
			} else {
				bodyEnd = bodyStart + closeIdx
			}

			baseLine := p.lineAt(bodyStart)
			sp := newScriptParser(p.src[bodyStart:bodyEnd], p.fileURI, baseLine, p.resolvers).asCFScript()
			sp.resolverSet = p.resolverSet
			sp.extractCalls = p.extractCalls
			sp.parse()
			p.funcs = append(p.funcs, sp.funcs...)
			p.vars = append(p.vars, sp.vars...)

			// Merge calls from cfscript sub-parser
			if p.extractCalls {
				for i := range sp.calls {
					c := &sp.calls[i]

					p.addCall(c)
				}

				for _, calls := range sp.funcCalls {
					for i := range calls {
						c := &calls[i]

						p.addCall(c)
					}
				}
			}

			if p.inFunc != "" {
				for i := range sp.componentRefs {
					ref := &sp.componentRefs[i]

					p.addRef(ref)
				}
			} else {
				p.componentRefs = append(p.componentRefs, sp.componentRefs...)
			}

			// Merge pending calls (unresolved "x = y.method()" assignments) from
			// the cfscript sub-parser. Without this, chained factory calls inside
			// a nested <cfscript> block (e.g. "conn = uri.openConnection();")
			// never reach resolvePendingCalls, so they can never fall back to
			// baseVar's own component (or FuncLookup's declared return type).
			for i := range sp.pendingCalls {
				c := sp.pendingCalls[i] // a copy: edited below, and kept

				if c.funcKey == "" {
					c.funcKey = p.inFunc
				}

				p.pendingCalls = append(p.pendingCalls, c)
			}

			if closeIdx < 0 {
				pos = len(p.src)
			} else {
				pos = bodyEnd + 11 // len("</cfscript>")
			}

			continue
		}

		// Check for CF tags we care about
		switch {
		case idx+3 < len(p.src) && toLowerByte(p.src[idx+1]) == 'c' && toLowerByte(p.src[idx+2]) == 'f':
			// Fast skip on the fourth byte: only tags starting cfc/cff/cfl/cfp/cfs/
			// cfo/cfi/cfr/cfe reach the switch below (cfcomponent, cffunction,
			// cfloop, cfparam/cfproperty, cfset, cfobject, cfif/cfinvoke,
			// cfreturn, cfelseif).
			//
			// 'l' and 'e' are the two admitted for a single tag each, and they are
			// the ones to re-measure if this loop ever shows up in a profile.
			// <cfloop index="x"> declares x, so go-to-definition on a loop index
			// has nowhere to land without 'l'; the cost is that every <cflock>,
			// <cflog>, <cflocation> and <cfldap> now pays an IndexByte for its '>'
			// and a lineAt, and <cfloop> is among the most common tags in CFML.
			// 'e' is admitted for <cfelseif>, whose condition holds calls like any
			// other expression, and charges the same to <cfelse>, <cfexit> and
			// <cferror>. Both measured within noise on the parser benchmarks,
			// because that work is a byte scan over a tag that was going to be
			// scanned past anyway.
			ch := toLowerByte(p.src[idx+3])
			if ch != 'c' && ch != 'f' && ch != 'l' && ch != 'p' && ch != 's' && ch != 'o' && ch != 'i' && ch != 'r' && ch != 'e' && ch != '/' {
				pos = p.stepOverEvaluatedTag(idx)

				continue
			}

			tagEnd := tagEndIndex(p.src[idx:])
			if tagEnd < 0 {
				pos = idx + 1

				continue
			}

			tagEnd += idx + 1 // past the >
			tag := p.src[idx:tagEnd]
			line := p.lineAt(idx)

			// A handled tag is stepped over whole, so its own attributes are in
			// no text gap: `<cfloop array="#svc.list()#">` is scanned here.
			p.scanInterpolatedText(tag, idx)

			// Detect </cffunction> to exit function scope.
			//
			// This was dead code twice over: "</cffunction>" is exactly 13
			// bytes so `len(tag) > 13` was false, and tag[2:13] is
			// "cffunction>" — 11 bytes — which never equals the 10-byte
			// "cffunction". Function scope was therefore never exited.
			if isCloseTagFor(tag, "cffunction") {
				p.inFunc = ""
				p.localVars = p.localVars[:0]
				p.forceGlobal = false
				pos = tagEnd

				continue
			}

			// Dispatch on the fourth byte the fast skip already read, rather than
			// testing each prefix in turn. The chain this replaces ran up to nine
			// EqualFold calls for every tag that got this far; the byte narrows it
			// to one or two, which is what pays for <cfloop> now being admitted to
			// the scan at all.
			//
			// The letters here must stay in step with the fast skip above: a tag
			// admitted there with no case here is scanned and then dropped, and a
			// case here whose letter the skip rejects is unreachable.
			// TestTagDispatchMatchesTheFastSkip fails on either.
			switch ch {
			case 'c':
				if hasCFTagPrefix(tag, "<cfcomponent") {
					p.extends = getAttr(tag, "extends")
					if isTruthy(getAttr(tag, "persistent")) {
						p.persistent = true
					}
				}
			case 'f':
				if hasCFTagPrefix(tag, "<cffunction") {
					p.parseCFFunction(tag, idx, tagEnd, line)
					p.enterFunctionScope(tagEnd, line)
				}
			case 'l':
				if hasCFTagPrefix(tag, "<cfloop") {
					p.parseScopedAttrVar(tag, "index", line)
				}
			case 'p':
				switch {
				case hasCFTagPrefix(tag, "<cfproperty"):
					p.parseCFProperty(tag, line)
				case hasCFTagPrefix(tag, "<cfparam"):
					p.parseScopedAttrVar(tag, "name", line)
				}
			case 's':
				if hasCFTagPrefix(tag, "<cfset") {
					p.parseCFSet(tag, line)
				}
			case 'o':
				if hasCFTagPrefix(tag, "<cfobject") {
					p.parseCFObject(tag, line)
				}
			case 'i':
				switch {
				case hasCFTagPrefix(tag, "<cfinvoke"):
					p.parseCFInvoke(tag, line)
				case hasCFTagPrefix(tag, "<cfif"):
					// A condition is an expression, and `<cfif svc.isValid(x)>`
					// recorded nothing while the same test in script syntax
					// recorded the call.
					p.scanExpressionCalls(tagBody(tag), line)
				}
			case 'e':
				// 'e' is admitted for this one tag, the way 'l' is for <cfloop>.
				// The cost is an IndexByte and a lineAt on every <cfelse>,
				// <cfexit> and <cferror>; the alternative is that the second
				// half of every if/else chain in tag syntax has no calls in it.
				if hasCFTagPrefix(tag, "<cfelseif") {
					p.scanExpressionCalls(tagBody(tag), line)
				}
			case 'r':
				if hasCFTagPrefix(tag, "<cfreturn") {
					p.parseCFReturn(tag, line)
				}
			}

			pos = tagEnd
		case p.gated && customPrefixTag(p.src[idx:], p.importPrefixes):
			pos = p.stepOverEvaluatedTag(idx)
		default:
			pos = idx + 1
		}
	}

	if p.extractLinks {
		p.extractAllLinks()
	}
}

// nextTag is nextTagStart from pos, or, gated and outside an output context, a
// plain search for the next '<'.
//
// nextTagStart steps over a #...# span because markup inside one is an
// argument, not a tag. That holds only where hashes are evaluated. Outside an
// output context a hash is a literal character, and taking one as a span's
// opening hid real tags: a Handlebars "{{#associated_rolls}}" in a kiosk
// template paired with a hash far below, the walk stepped over every tag
// between them, and the <cfif> conditions there lost their calls — which the
// old reading only recovered by scanning the whole run as one expression.
func (p *tagParser) nextTag(pos int) int {
	if p.gated && !p.textEvaluated(pos) {
		return strings.IndexByte(p.src[pos:], '<')
	}

	return nextTagStart(p.src[pos:])
}

// textEvaluated reports whether the text at pos is where ColdFusion evaluates
// #...#. Ungated, all of it is, as the parser has always read it.
func (p *tagParser) textEvaluated(pos int) bool {
	return !p.gated || inSpan(p.outputSpans, p.srcOffset+pos)
}

// stepOverEvaluatedTag handles a tag the walk has no handler for but whose
// attributes ColdFusion evaluates in any context: a <cf...> tag the fast skip
// declines (<cfquery>, <cfmail>, <cfmodule>, <cf_custom>) or a cfimport-prefixed
// custom tag. Gated, it scans the attributes and returns the offset past the
// tag, so they are not left in the next text gap for the gate to drop. Ungated,
// it keeps the old walk, which advances one byte and scans the attributes as
// part of that gap.
func (p *tagParser) stepOverEvaluatedTag(idx int) int {
	if !p.gated {
		return idx + 1
	}

	end := tagEndIndex(p.src[idx:])
	if end < 0 {
		return idx + 1
	}

	tagEnd := idx + end + 1
	p.scanInterpolatedText(p.src[idx:tagEnd], idx)

	return tagEnd
}

// enterFunctionScope records the <cffunction> the parser has just entered, so
// assignments inside it are attributed to the function rather than the file.
func (p *tagParser) enterFunctionScope(tagEnd, line int) {
	endLine := -1

	if closeIdx := indexCFTag(p.src[tagEnd:], "/cffunction"); closeIdx >= 0 {
		endLine = p.lineAt(tagEnd + closeIdx)
	} else {
		// Not found within this region's text — the function body is interrupted
		// by a nested <cfscript> region split. Fall back to the whole-file
		// pre-scan for the real end.
		absStart := p.baseLine + line

		for _, s := range p.knownScopes {
			if s.Start == absStart {
				endLine = s.End - p.baseLine

				break
			}
		}
	}

	if endLine < 0 {
		return
	}

	p.inFunc = funcKey(line, endLine)
	p.scopes = append(p.scopes, FuncScope{Start: line, End: endLine})

	// The set starts nil and is built on first write. It was a map per
	// <cffunction> whether or not the function declared anything local, which on
	// the benchmark component was 13% of everything the tag parser allocated —
	// and a map is an expensive way to record nothing.
	p.localVars = p.localVars[:0]

	if len(p.funcs) > 0 {
		for _, arg := range p.funcs[len(p.funcs)-1].Arguments {
			p.markVarLocal(arg.Name)
		}
	}
}

// markVarLocal records name as declared local to the function being parsed,
// allocating the set on first use.
func (p *tagParser) markVarLocal(name string) {
	if p.isVarDeclaredLocal(name) {
		return
	}

	p.localVars = append(p.localVars, name)
}

// parseCFFunction extracts a function def from <cffunction> and its <cfargument> children.
func (p *tagParser) parseCFFunction(tag string, idx, tagEnd, line int) {
	name := getAttr(tag, "name")
	if name == "" {
		return
	}

	// Check for JSDoc comment preceding this function tag
	docComment := p.precedingComment(idx)

	// Find arguments between this tag and </cffunction> or next <cffunction
	rest := p.src[tagEnd:]

	end := len(rest)
	if ci := indexCFTag(rest, "/cffunction"); ci >= 0 && ci < end {
		end = ci
	}

	if ci := indexCFTag(rest, "cffunction"); ci >= 0 && ci < end {
		end = ci
	}

	block := rest[:end]

	args := p.parseCFArguments(block, tagEnd)

	// Apply JSDoc @param {type} annotations
	if docComment != "" {
		applyJSDocParams(docComment, args)
	}

	// Create component refs for arguments with component-like types.
	// p.inFunc is not yet set at this call site — the main loop sets it after
	// parseCFFunction returns. Using p.addRef would route refs to componentRefs
	// (global), which causes cachedFuncRefs to miss them when the funcRefsMap
	// entry already exists from resolver-derived refs in the same scope.
	// Instead, compute the funcKey directly and write to funcRefs[key].
	for _, a := range args {
		if isComponentType(a.Type) {
			ref := ComponentRef{
				Variable:  a.Name,
				Component: a.Type,
				URI:       uriFromString(p.fileURI),
				Line:      uint32(line),
			}

			endLine := -1

			if end < len(rest) {
				endLine = p.lineAt(tagEnd + end)
			} else {
				// Not found within this region's text — the function body is
				// interrupted by a nested <cfscript> region split. Fall back
				// to the whole-file pre-scan for the real end (see baseLine).
				absStart := p.baseLine + line

				for _, s := range p.knownScopes {
					if s.Start == absStart {
						endLine = s.End - p.baseLine

						break
					}
				}
			}

			if endLine >= 0 {
				key := funcKey(line, endLine)

				if p.funcRefs == nil {
					p.funcRefs = make(map[string][]ComponentRef)
				}

				p.funcRefs[key] = append(p.funcRefs[key], ref)
			} else {
				p.componentRefs = append(p.componentRefs, ref)
			}
		}
	}

	returnType := getAttr(tag, "returntype")

	// Promote hint to returntype when returntype is generic and hint looks
	// like a component path — same safe mechanism as cfargument's Type/Hint
	// above (hint is documentation-only, so this carries no runtime
	// type-coercion risk the way editing returntype directly would).
	if returnHint := getAttr(tag, "hint"); isComponentType(returnHint) && !isComponentType(returnType) {
		returnType = returnHint
	}

	p.funcs = append(p.funcs, FunctionDef{
		Name:       name,
		URI:        uriFromString(p.fileURI),
		Line:       uint32(line),
		Arguments:  args,
		ReturnType: returnType,
	})
}

// parseCFArguments extracts <cfargument> tags from a block of source.
// precedingComment extracts the CFML comment (<!--- ... --->) immediately before position idx.
func (p *tagParser) precedingComment(idx int) string {
	// Walk backward from idx skipping whitespace to find --->
	i := idx - 1
	for i >= 0 && (p.src[i] == ' ' || p.src[i] == '\t' || p.src[i] == '\n' || p.src[i] == '\r') {
		i--
	}

	// Check for ---> ending (i points to '>')
	if i < 3 || p.src[i-3:i+1] != "--->" {
		return ""
	}

	// Find matching <!---
	start := strings.LastIndex(p.src[:i-3], "<!---")
	if start < 0 {
		return ""
	}

	return p.src[start+5 : i-3]
}

// blockStart is the offset of block within p.src, so each <cfargument> can be
// recorded at the line it is written on. An argument is a declaration — it is
// what `ref = argumentVariable` refers to — and without a line there is nowhere
// for go-to-definition to land.
func (p *tagParser) parseCFArguments(block string, blockStart int) []Argument {
	// Most functions take a few arguments, and a nil slice reaching four costs
	// three copies to get there.
	args := make([]Argument, 0, 4)

	pos := 0

	for {
		idx := indexCFTag(block[pos:], "cfargument")
		if idx < 0 {
			break
		}

		idx += pos

		end := strings.IndexByte(block[idx:], '>')
		if end < 0 {
			break
		}

		tag := block[idx : idx+end+1]

		name := getAttr(tag, "name")
		if name != "" {
			a := Argument{Name: name}
			a.Type = getAttr(tag, "type")
			a.Hint = getAttr(tag, "hint")
			req := getAttr(tag, "required")
			a.Required = strings.EqualFold(req, "true") || strings.EqualFold(req, "yes")
			// Promote hint to type when type is generic and hint looks like a component path
			if isComponentType(a.Hint) && !isComponentType(a.Type) {
				a.Type = a.Hint
			}

			args = append(args, a)

			p.vars = append(p.vars, VarDef{
				Name: name, Scope: ScopeArguments,
				Line: uint32(p.lineAt(blockStart + idx)),
			})
		}

		pos = idx + end + 1
	}

	return args
}

// parseReadOnlyScopeSet records `<cfset application.x = ...>` and its siblings,
// returning whether it matched so parseCFSet's switch can use it as a case.
//
// One helper over a case per scope: the scopes behave identically here, and nine
// near-identical branches is the shape that acquires an inconsistency nobody
// notices.
func (p *tagParser) parseReadOnlyScopeSet(inner string, line int) bool {
	dot := strings.IndexByte(inner, '.')
	if dot <= 0 {
		return false
	}

	scope, ok := readOnlyScopePrefixes[strings.ToLower(strings.TrimSpace(inner[:dot]))]
	if !ok {
		return false
	}

	name, _ := splitAssign(inner[dot+1:])
	if name == "" {
		return false
	}

	p.vars = append(p.vars, VarDef{Name: name, Scope: scope, Line: uint32(line)})

	return true
}

// parseScopedAttrVar records the variable a tag attribute declares —
// `<cfparam name="url.x">` and `<cfloop index="variables.i">`. Both write a
// scope-qualified name into an attribute, and both create the variable rather
// than referring to one, which is what makes them definition targets.
//
// An unqualified value (`<cfloop index="i">`) lands in the variables scope,
// which is where CFML puts it.
func (p *tagParser) parseScopedAttrVar(tag, attr string, line int) {
	value := getAttr(tag, attr)
	if value == "" {
		return
	}

	scope := ScopeVariables
	name := value

	if dot := strings.IndexByte(value, '.'); dot > 0 {
		if sc, ok := ScopeForPrefix(value[:dot]); ok {
			scope = sc
			name = value[dot+1:]
		}
	}

	if name == "" || strings.ContainsAny(name, ".[#") {
		return
	}

	p.vars = append(p.vars, VarDef{Name: name, Scope: scope, Line: uint32(line)})
}

// parseCFSet handles <cfset var x = ...>, <cfset local.x = ...>, etc.
func (p *tagParser) parseCFSet(tag string, line int) {
	// Strip <cfset and trailing >
	inner := tag
	if i := strings.IndexByte(inner, ' '); i >= 0 {
		inner = inner[i+1:]
	}

	inner = strings.TrimSuffix(inner, ">")
	inner = strings.TrimSuffix(strings.TrimSpace(inner), "/")
	inner = strings.TrimSpace(inner)

	// A <cfset> holds an expression, and the shapes matched below are a few of
	// the ways one can hold a call. Measured against six open-source projects,
	// they missed about 3,100 call sites: a bare `<cfset arrayAppend(a, b)>`, a
	// call after a concatenation, a nested call in an argument list, and any
	// assignment whose left-hand side is scope-prefixed. The expression goes to
	// the script parser afterwards for the rest, which is what <cfif> and
	// <cfreturn> already do; the string paths stay because they carry the refs
	// and pending calls that decide what a variable now holds.
	defer p.scanSetExpressionCalls(inner, line)

	switch {
	case hasPrefixFold(inner, "var "):
		rest := strings.TrimSpace(inner[4:])

		name := extractIdent(rest)
		if name != "" {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeLocal, Line: uint32(line)})

			if p.inFunc != "" {
				p.markVarLocal(name)
			}

			p.checkSetRHS(rest, name, line)
		}
	case hasPrefixFold(inner, "local."):
		rest := inner[6:]

		name, rhs := splitAssign(rest)
		if name != "" {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeLocal, Line: uint32(line)})

			if p.inFunc != "" {
				p.markVarLocal(name)
			}

			p.checkSetRHSStr(rhs, name, line)
		}
	case hasPrefixFold(inner, "arguments."):
		rest := inner[10:]

		name, rhs := splitAssign(rest)
		if name != "" {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeArguments, Line: uint32(line)})
			p.checkSetRHSStr(rhs, name, line)
		}
	case hasPrefixFold(inner, "this."):
		rest := inner[5:]

		name, rhs := splitAssign(rest)
		if name != "" {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeThis, Line: uint32(line)})
			p.forceGlobal = true
			p.checkSetRHSStr(rhs, name, line)
			p.forceGlobal = false
		}
	case hasPrefixFold(inner, "variables."):
		rest := inner[10:]

		name, rhs := splitAssign(rest)
		if name != "" {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeVariables, Line: uint32(line)})
			p.forceGlobal = true
			p.checkSetRHSStr(rhs, name, line)
			p.forceGlobal = false
		}
	case p.parseReadOnlyScopeSet(inner, line):
		// Handled: an assignment into url./application./request./session./etc.
		// These carry no component type, so there is no RHS to follow — the
		// point is only that the name was declared here.
	default:
		name, rhs := splitAssign(inner)
		if name != "" && !isKeyword(name) {
			if rhs == "" && p.extractCalls {
				// No assignment — check for bare call: obj.method(...)
				p.checkBareCallStr(inner, line)
			} else if rhs != "" {
				// If var was previously declared local in this function, keep it local
				isLocal := p.inFunc != "" && p.isVarDeclaredLocal(name)
				if !isLocal {
					p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeVariables, Line: uint32(line)})
				}

				p.forceGlobal = !isLocal
				p.checkSetRHSStr(rhs, name, line)
				p.forceGlobal = false
			}
		}
	}
}

// parseCFObject handles <cfobject component="path" name="var">
func (p *tagParser) parseCFObject(tag string, line int) {
	component := getAttr(tag, "component")
	name := getAttr(tag, "name")

	if component != "" && name != "" {
		p.addRef(&ComponentRef{
			Variable:  name,
			Component: component,
			URI:       uriFromString(p.fileURI),
			Line:      uint32(line),
		})
	}
}

// parseCFInvoke handles <cfinvoke component="path" returnvariable="var">
func (p *tagParser) parseCFInvoke(tag string, line int) {
	component := getAttr(tag, "component")
	variable := getAttr(tag, "returnvariable")

	if component != "" && variable != "" {
		p.addRef(&ComponentRef{
			Variable:  variable,
			Component: component,
			URI:       uriFromString(p.fileURI),
			Line:      uint32(line),
		})
	}
}

// parseCFReturn handles <cfreturn expr /> to infer function return component.
func (p *tagParser) parseCFReturn(tag string, line int) {
	inner := tagBody(tag)

	// A returned expression holds calls whether or not this is the return that
	// sets the component, and whether or not we are inside a function at all —
	// so the scan happens before every early exit below. `<cfreturn
	// svc.value()>` used to record nothing.
	p.scanExpressionCalls(inner, line)

	if p.inFunc == "" || len(p.funcs) == 0 {
		return
	}

	f := &p.funcs[len(p.funcs)-1]
	if f.ReturnComponent != "" || f.returnVar != "" {
		return // already set from an earlier return
	}

	if inner == "" {
		return
	}
	// Check for direct new/createObject
	switch {
	case hasPrefixFold(inner, "new "):
		if comp := extractComponentPath(inner[4:]); comp != "" {
			f.ReturnComponent = comp
		}
	case hasPrefixFold(inner, "createobject("):
		if comp := extractCreateObjectArg(inner[13:]); comp != "" {
			f.ReturnComponent = comp
		}
	default:
		// return varName — store for deferred resolution
		varName := extractIdent(inner)
		if varName != "" && !strings.Contains(inner, "(") {
			f.returnVar = varName
		}
	}
}

// parseCFProperty handles <cfproperty name="x" type="y" />
func (p *tagParser) parseCFProperty(tag string, line int) {
	name := getAttr(tag, "name")
	if name == "" {
		return
	}

	typeName := getAttr(tag, "type")
	attrs := extractAllAttrs(tag)
	p.properties = append(p.properties, propertyDef{name: name, typeName: typeName, line: uint32(line), attrs: attrs})
}

func (p *tagParser) checkSetRHS(rest, varName string, line int) {
	// Find = and check what's after it
	if _, after, ok := strings.Cut(rest, "="); ok {
		p.checkSetRHSStr(strings.TrimSpace(after), varName, line)
	}
}

func (p *tagParser) checkSetRHSStr(rhs, varName string, line int) {
	rhs = strings.TrimSpace(rhs)

	switch {
	case strings.EqualFold(rhs, "this"):
		if selfPath := strings.TrimPrefix(p.fileURI, "file://"); selfPath != "" {
			p.addRef(&ComponentRef{
				Variable: varName, Component: selfPath,
				URI: uriFromString(p.fileURI), Line: uint32(line),
			})
		}
	case hasPrefixFold(rhs, "new "):
		comp := extractComponentPath(rhs[4:])
		if comp != "" {
			p.addRef(&ComponentRef{
				Variable: varName, Component: comp, ChainRest: trailingCalls(rhs),
				URI: uriFromString(p.fileURI), Line: uint32(line),
			})
		}

	case hasPrefixFold(rhs, "createobject("):
		comp := extractCreateObjectArg(rhs[13:])
		if comp != "" {
			p.addRef(&ComponentRef{
				Variable: varName, Component: comp, ChainRest: trailingCalls(rhs),
				URI: uriFromString(p.fileURI), Line: uint32(line),
			})
		} else if len(p.resolvers) > 0 {
			if comp := p.resolveCall(rhs); comp != "" {
				p.addRef(&ComponentRef{
					Variable: varName, Component: comp, ChainRest: trailingCalls(rhs),
					URI: uriFromString(p.fileURI), Line: uint32(line),
				})
			}
		}

	case hasPrefixFold(rhs, "entitynew("):
		comp := extractEntityNewArg(rhs[10:])
		if comp != "" {
			p.addRef(&ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(line),
			})
		}
	case hasPrefixFold(rhs, "entityload("):
		comp := extractEntityNewArg(rhs[11:])
		if comp != "" {
			p.addRef(&ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(line),
			})
		}
	default:
		// Try generic resolver match on the RHS expression
		if len(p.resolvers) > 0 {
			if comp := p.resolveCall(rhs); comp != "" {
				p.addRef(&ComponentRef{
					Variable: varName, Component: comp,
					URI: uriFromString(p.fileURI), Line: uint32(line),
				})

				return
			}
			// Try bare function name for exact-match resolvers
			if funcName := extractIdent(rhs); funcName != "" {
				for i := range p.resolvers {
					r := &p.resolvers[i]
					if r.Prefix != "" && prefixEqualFold(funcName, r.Prefix) {
						if comp := matchResolverWithCache(funcName, r); comp != "" {
							p.resolverSet.noteSoft(r, comp)

							p.addRef(&ComponentRef{
								Variable: varName, Component: comp,
								URI: uriFromString(p.fileURI), Line: uint32(line),
							})

							return
						}
					}
				}
			}
		}
		// Detect x = someVar.method(...) or x = funcName(...) pattern
		if baseVar := extractMethodCallBase(rhs); baseVar != "" {
			// Extract method name for pendingCall
			var methodForPending string

			before, _, _ := strings.Cut(rhs, "(")
			if dot := strings.LastIndexByte(before, '.'); dot >= 0 {
				methodForPending = before[dot+1:]
			}

			if p.extractCalls {
				// Record full call: extract method name from rhs
				if dot := strings.LastIndexByte(before, '.'); dot >= 0 {
					methodName := before[dot+1:]

					varChain := before[:dot]
					if isIdentifier(methodName) && isValidVarChain(varChain) {
						caller := ""
						if p.inFunc != "" && len(p.funcs) > 0 {
							caller = p.funcs[len(p.funcs)-1].Name
						}

						comp := p.lookupComponentRef(varChain, line)

						p.addCall(&CallSite{
							FuncName:  methodName,
							Variable:  varChain,
							Component: comp,
							Resolved:  comp != "",
							Line:      uint32(line),
							Caller:    caller,
						})
					}
				}
			}

			p.pendingCalls = append(p.pendingCalls, pendingCall{
				varName:  varName,
				funcName: methodForPending,
				baseVar:  baseVar,
				line:     uint32(line),
				funcKey:  p.inFunc,
				rest:     trailingCalls(rhs),
			})
		} else if paren := strings.IndexByte(rhs, '('); paren > 0 {
			funcName := extractIdent(rhs)
			// Confirm the parens actually belong to funcName (only whitespace between
			// them) — otherwise rhs is a larger expression like `temp & "~" & DateFormat(...)`
			// where extractIdent grabbed the leading token ("temp"), not the identifier the
			// paren is attached to, and this isn't a `x = funcName(...)` shape at all.
			if funcName != "" && !isKeyword(funcName) && strings.TrimSpace(rhs[len(funcName):paren]) == "" {
				if p.builtinReturnLookup != nil {
					if comp := p.builtinReturnLookup(funcName); comp != "" {
						p.addRef(&ComponentRef{
							Variable: varName, Component: comp,
							URI: uriFromString(p.fileURI), Line: uint32(line),
						})

						return
					}
				}

				if p.extractCalls {
					caller := ""
					if p.inFunc != "" && len(p.funcs) > 0 {
						caller = p.funcs[len(p.funcs)-1].Name
					}

					p.addCall(&CallSite{
						FuncName: funcName,
						Line:     uint32(line),
						Caller:   caller,
					})
				}

				p.pendingCalls = append(p.pendingCalls, pendingCall{
					varName:  varName,
					funcName: funcName,
					line:     uint32(line),
					funcKey:  p.inFunc,
					rest:     trailingCalls(rhs),
				})
			}
		}
	}
}

// checkBareCallStr detects bare obj.method(...) patterns in a <cfset> without assignment.
func (p *tagParser) checkBareCallStr(expr string, line int) {
	before, _, ok := strings.Cut(expr, "(")
	if !ok {
		return
	}

	// If there's an = before the (, it's an assignment, not a bare call
	if strings.IndexByte(before, '=') >= 0 {
		return
	}

	dot := strings.LastIndexByte(before, '.')
	if dot < 0 {
		return
	}

	varName := strings.TrimSpace(before[:dot])
	methodName := strings.TrimSpace(before[dot+1:])

	if methodName == "" || varName == "" || !isIdentifier(methodName) {
		return
	}

	if !isValidVarChain(varName) {
		return
	}

	caller := ""
	if p.inFunc != "" && len(p.funcs) > 0 {
		caller = p.funcs[len(p.funcs)-1].Name
	}

	comp := p.lookupComponentRef(varName, line)

	p.addCall(&CallSite{
		FuncName:  methodName,
		Variable:  varName,
		Component: comp,
		Resolved:  comp != "",
		Line:      uint32(line),
		Caller:    caller,
	})
}

// getAttr extracts an attribute value from a tag string (case-insensitive).
func getAttr(tag, attr string) string {
	// Scanned case-insensitively in place rather than against a lowercased copy
	// of the tag.
	//
	// Two reasons, and the second is a correctness one. A <cffunction> tag is
	// asked for five or six attributes, so the copy was made five or six times
	// for one tag — strings.ToLower was 10% of tag parsing in a profile, the
	// largest single cost in it, and getAttr 13% including it. And the offsets
	// from that copy were then used to index `tag`, which only holds while
	// folding preserves length: it does not universally (U+0130 lowercases to
	// two runes), so a tag carrying one desynchronised every offset after it.
	//
	// Find the attribute name followed by optional whitespace and =
	idx := -1
	searchFrom := 0

	for {
		i := indexFoldFrom(tag, attr, searchFrom)
		if i < 0 {
			break
		}

		// The name has to start a word. Only the text *after* the match used to
		// be checked, so a plain substring hit inside a longer attribute name
		// counted: "name" matched the tail of "displayname=", and
		// <cffunction displayname="Donor Lookup" name="getDonor"> was indexed
		// as a function literally called "Donor Lookup". Same for "type"
		// inside "returntype=" and any other attribute ending in the one being
		// looked up.
		if i > 0 && isAttrNameByte(tag[i-1]) {
			searchFrom = i + 1

			continue
		}

		j := i + len(attr)
		for j < len(tag) && isWhitespace(tag[j]) {
			j++
		}

		if j < len(tag) && tag[j] == '=' {
			idx = i

			break
		}

		searchFrom = i + 1
	}

	if idx < 0 {
		return ""
	}

	// Skip past attr name, whitespace, =, whitespace to reach the value
	valStart := idx + len(attr)
	for valStart < len(tag) && isWhitespace(tag[valStart]) {
		valStart++
	}

	valStart++ // skip =

	for valStart < len(tag) && isWhitespace(tag[valStart]) {
		valStart++
	}

	if valStart >= len(tag) {
		return ""
	}

	ch := tag[valStart]
	if ch == '"' || ch == '\'' {
		valStart++

		end := strings.IndexByte(tag[valStart:], ch)
		if end < 0 {
			return ""
		}

		return tag[valStart : valStart+end]
	}
	// Unquoted value — read until space or >
	end := valStart
	for end < len(tag) && tag[end] != ' ' && tag[end] != '>' && tag[end] != '\t' {
		end++
	}

	return tag[valStart:end]
}

func extractIdent(s string) string {
	i := 0
	for i < len(s) && isIdentPart(s[i]) {
		i++
	}

	if i == 0 {
		return ""
	}

	return s[:i]
}

// extractAllAttrs extracts all attribute key=value pairs from a tag string.
// Keys are lowercased.
func extractAllAttrs(tag string) map[string]string {
	attrs := make(map[string]string)
	// Skip tag name
	i := 1 // past '<'
	for i < len(tag) && tag[i] != ' ' && tag[i] != '\t' && tag[i] != '>' {
		i++
	}

	for i < len(tag) {
		// Skip whitespace
		for i < len(tag) && isWhitespace(tag[i]) {
			i++
		}

		if i >= len(tag) || tag[i] == '>' || tag[i] == '/' {
			break
		}
		// Read attribute name
		start := i

		for i < len(tag) && tag[i] != '=' && tag[i] != ' ' && tag[i] != '>' && tag[i] != '/' {
			i++
		}

		key := strings.ToLower(tag[start:i])

		if i >= len(tag) || tag[i] != '=' {
			continue
		}

		i++ // past '='
		if i >= len(tag) {
			break
		}

		if tag[i] == '"' || tag[i] == '\'' {
			q := tag[i]
			i++
			valStart := i

			for i < len(tag) && tag[i] != q {
				i++
			}

			attrs[key] = tag[valStart:i]

			if i < len(tag) {
				i++ // past closing quote
			}
		} else {
			valStart := i

			for i < len(tag) && tag[i] != ' ' && tag[i] != '>' {
				i++
			}

			attrs[key] = tag[valStart:i]
		}
	}

	return attrs
}

func splitAssign(s string) (name, rhs string) {
	name = extractIdent(s)
	if name == "" {
		return "", ""
	}

	rest := strings.TrimSpace(s[len(name):])
	if rest != "" && rest[0] == '=' {
		return name, strings.TrimSpace(rest[1:])
	}

	return name, ""
}

func extractComponentPath(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	if s[0] == '"' || s[0] == '\'' {
		q := s[0]

		end := strings.IndexByte(s[1:], q)
		if end < 0 {
			return ""
		}

		return s[1 : 1+end]
	}
	// Unquoted dotted path
	i := 0
	for i < len(s) && (isIdentPart(s[i]) || s[i] == '.') {
		i++
	}

	return s[:i]
}

func extractCreateObjectArg(s string) string {
	// Expects: "component", "path") — we're past the opening (
	s = strings.TrimSpace(s)
	if s == "" || (s[0] != '"' && s[0] != '\'') {
		return ""
	}

	q := s[0]

	end := strings.IndexByte(s[1:], q)
	if end < 0 {
		return ""
	}

	first := s[1 : 1+end]
	if !strings.EqualFold(first, "component") {
		return ""
	}

	rest := s[2+end:]

	ci := strings.IndexByte(rest, ',')
	if ci < 0 {
		return ""
	}

	rest = strings.TrimSpace(rest[ci+1:])
	if rest == "" || (rest[0] != '"' && rest[0] != '\'') {
		return ""
	}

	q2 := rest[0]

	end2 := strings.IndexByte(rest[1:], q2)
	if end2 < 0 {
		return ""
	}

	return rest[1 : 1+end2]
}

func extractEntityNewArg(s string) string {
	// Expects: "EntityName") — we're past the opening (
	s = strings.TrimSpace(s)
	if s == "" || (s[0] != '"' && s[0] != '\'') {
		return ""
	}

	q := s[0]

	end := strings.IndexByte(s[1:], q)
	if end < 0 {
		return ""
	}

	return s[1 : 1+end]
}

// extractMethodCallBase returns the base variable name from a "baseVar.method(" pattern.
// For "variables.jss.getInstance(..." returns "jss". Returns "" if not a method call.
func extractMethodCallBase(rhs string) string {
	before, _, ok := strings.Cut(rhs, "(")
	if !ok {
		return ""
	}

	prefix := before

	dot := strings.LastIndexByte(prefix, '.')
	if dot < 0 {
		return ""
	}

	base := prefix[:dot]
	if lastDot := strings.LastIndexByte(base, '.'); lastDot >= 0 {
		return base[lastDot+1:]
	}

	return base
}

func toLowerByte(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}

	return b
}

// hasPrefixFold checks if s starts with prefix (case-insensitive, ASCII only).
func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}

	return strings.EqualFold(s[:len(prefix)], prefix)
}

// addRef appends a component ref to the correct bucket (global or per-function).
// isVarDeclaredLocal returns true if the variable was declared with var or local.
// in the current function.
func (p *tagParser) isVarDeclaredLocal(name string) bool {
	for _, v := range p.localVars {
		if strings.EqualFold(v, name) {
			return true
		}
	}

	return false
}

func (p *tagParser) resolveCall(expr string) string {
	if p.resolverSet != nil {
		return p.resolverSet.Resolve(expr)
	}

	return ResolveFromCall(expr, p.resolvers)
}

// Refs assigned to VARIABLES. or this. scopes are always global.
func (p *tagParser) addRef(ref *ComponentRef) {
	if p.inFunc == "" || p.forceGlobal {
		p.componentRefs = append(p.componentRefs, *ref)
	} else {
		if p.funcRefs == nil {
			p.funcRefs = make(map[string][]ComponentRef)
		}

		p.funcRefs[p.inFunc] = append(p.funcRefs[p.inFunc], *ref)
	}
}

// lookupComponentRef finds a previously-recorded ComponentRef for varName, so
// a call site like `psService.updatePerson(...)` can carry the Component that
// an earlier `psService = getService("person")` assignment already resolved
// (mirrors resolve.CanResolveCall's function-scope-then-global precedence).
// Only sees refs recorded earlier in this single forward pass — same
// limitation goto-definition has for forward references.
//
// Prefers the nearest-preceding assignment at or before atLine over the first
// match in parse order: a scratch variable can be reassigned multiple times in
// the same file (e.g. once per <cfswitch>/<cfcase> branch), and the first ref
// in parse order may belong to an earlier, unrelated branch. Falls back to the
// first match in parse order for a genuine forward reference, where no
// preceding ref exists.
func (p *tagParser) lookupComponentRef(varName string, atLine int) string {
	lookupVar := varName
	if dot := strings.LastIndexByte(lookupVar, '.'); dot >= 0 {
		lookupVar = lookupVar[dot+1:]
	}

	if p.inFunc != "" {
		if comp := nearestComponentRef(p.funcRefs[p.inFunc], lookupVar, atLine); comp != "" {
			return comp
		}
	}

	return nearestComponentRef(p.componentRefs, lookupVar, atLine)
}

// nearestComponentRef returns the Component of the ref matching lookupVar whose
// Line is the largest value <= atLine, or the first matching ref in slice order
// if none precede atLine.
func nearestComponentRef(refs []ComponentRef, lookupVar string, atLine int) string {
	var best *ComponentRef

	for i := range refs {
		ref := &refs[i]
		if !strings.EqualFold(ref.Variable, lookupVar) || chainPending(ref) {
			continue
		}

		if int(ref.Line) > atLine {
			continue
		}

		if best == nil || ref.Line > best.Line {
			best = ref
		}
	}

	if best != nil {
		return best.Component
	}

	for i := range refs {
		if strings.EqualFold(refs[i].Variable, lookupVar) && !chainPending(&refs[i]) {
			return refs[i].Component
		}
	}

	return ""
}

func (p *tagParser) addCall(call *CallSite) {
	if p.inFunc == "" {
		p.calls = append(p.calls, *call)
	} else {
		if p.funcCalls == nil {
			p.funcCalls = make(map[string][]CallSite)
		}

		p.funcCalls[p.inFunc] = append(p.funcCalls[p.inFunc], *call)
	}
}

// extractAllLinks scans source lines for document links, routing them to
// global links or funcLinks based on which scope the line falls in.
func (p *tagParser) extractAllLinks() {
	src := p.src
	lineNum := 0
	scopeIdx := 0

	for src != "" {
		nl := strings.IndexByte(src, '\n')

		var line string
		if nl < 0 {
			line = src
			src = ""
		} else {
			line = src[:nl]
			src = src[nl+1:]
		}

		// Advance past finished scopes
		for scopeIdx < len(p.scopes) && lineNum > p.scopes[scopeIdx].End {
			scopeIdx++
		}

		if scopeIdx < len(p.scopes) && lineNum > p.scopes[scopeIdx].Start && lineNum < p.scopes[scopeIdx].End {
			key := funcKey(p.scopes[scopeIdx].Start, p.scopes[scopeIdx].End)
			if p.funcLinks == nil {
				p.funcLinks = make(map[string][]DocumentLink)
			}

			links := p.funcLinks[key]
			extractLinksFromLine(line, lineNum, &links)
			p.funcLinks[key] = links
		} else {
			extractLinksFromLine(line, lineNum, &p.links)
		}

		lineNum++
	}
}

// isWhitespace returns true if the byte is any whitespace character.
// isCloseTagFor reports whether tag is the closing tag for name, e.g.
// "</cffunction>" or "</cffunction >" for "cffunction". The name must be
// followed by whitespace or ">", so "</cffunctionfoo>" does not match.
func isCloseTagFor(tag, name string) bool {
	if len(tag) < len(name)+3 || tag[0] != '<' || tag[1] != '/' {
		return false
	}

	if !strings.EqualFold(tag[2:2+len(name)], name) {
		return false
	}

	rest := tag[2+len(name):]

	return rest[0] == '>' || isWhitespace(rest[0])
}

// isAttrNameByte reports whether c can appear inside a tag attribute name.
// Used to require that a name match starts a word rather than landing in the
// middle of a longer attribute.
func isAttrNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' ||
		c >= 'A' && c <= 'Z' ||
		c >= '0' && c <= '9' ||
		c == '_' || c == '-' || c == ':' || c == '.'
}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// hasCFTagPrefix checks if tag starts with prefix (case-insensitive) followed by whitespace.
func hasCFTagPrefix(tag, prefix string) bool {
	n := len(prefix)
	if len(tag) <= n {
		return false
	}

	if !strings.EqualFold(tag[:n], prefix) {
		return false
	}

	c := tag[n]

	return isWhitespace(c)
}

// scanInterpolatedText records the calls written inside #...# spans of a piece
// of tag-region source, at byte offset `offset` within p.src.
//
// `<cfoutput>#svc.getName()#</cfoutput>` is how tag-syntax CFML emits a
// computed value, and none of it was visible: the tag parser matches tags and
// never tokenises text. The cost was the argument-list gap's — no edge into the
// callee, so the code map read it as unreachable and `unresolved` never checked
// it.
//
// Each span is handed to a scriptParser, the same way a <cfscript> body already
// is, so one implementation of "what is a call" serves both. A span that is not
// an expression yields no calls and is dropped, which is what keeps a stray
// pair of hashes in prose or a CSS colour from inventing one.
func (p *tagParser) scanInterpolatedText(text string, offset int) {
	// Only calls are merged below, and a call needs a '('. Most interpolation in
	// a tag file is `#user.name#` or `#i#`, so rejecting a chunk on one byte
	// scan is what keeps this from costing a fifth of a tag parse.
	if !p.extractCalls || strings.IndexByte(text, '#') < 0 || strings.IndexByte(text, '(') < 0 {
		return
	}

	for _, span := range interpolatedSpans(text) {
		spanText := text[span.start:span.end]
		if strings.IndexByte(spanText, '(') < 0 {
			continue
		}

		p.scanExpressionCalls(spanText, p.lineAt(offset+span.start))
	}
}

// scanExpressionCalls records the calls written in one CFML expression.
//
// The tag parser matches tags and pulls attributes out with string searches; it
// has no expression parser and should not grow one. Where a tag *holds* an
// expression — a <cfif> condition, a <cfreturn>, a #...# span — it hands the
// text to a scriptParser and keeps the calls, which is the same thing a
// <cfscript> body already does. One implementation of "what is a call" then
// serves both syntaxes, and a fix to it reaches tag files for free.
//
// Only calls are merged. The refs and vars a sub-parse also produces belong to
// whatever the tag handler already recorded for them, and taking them here
// would record each one twice.
func (p *tagParser) scanExpressionCalls(expr string, line int) {
	p.mergeExpressionCalls(expr, line, false)
}

// scanSetExpressionCalls is scanExpressionCalls for a <cfset>, whose own
// string-matched shapes have already been recorded by the time it runs.
//
// Only <cfset> needs this. A #...# span and a <cfif> condition have nothing
// recorded before them, and deduping there is actively wrong: two
// interpolations of the same function on one line —
// `#getColdBoxSetting("a")# #getColdBoxSetting("b")#` — are two calls, and
// running the shared merge with the tally on cost the second one. The corpus
// caught that; a unit test on one span could not have.
func (p *tagParser) scanSetExpressionCalls(expr string, line int) {
	p.mergeExpressionCalls(expr, line, true)
}

// mergeExpressionCalls parses one CFML expression and records the calls in it.
//
// The tag parser matches tags and pulls attributes out with string searches; it
// has no expression parser and should not grow one. Where a tag *holds* an
// expression — a <cfif> condition, a <cfreturn>, a <cfset>, a #...# span — it
// hands the text to a scriptParser and keeps the calls, which is the same thing
// a <cfscript> body already does. One implementation of "what is a call" then
// serves both syntaxes, and a fix to it reaches tag files for free.
//
// Only calls are merged. The refs and vars a sub-parse also produces belong to
// whatever the tag handler already recorded for them, and taking them here
// would record each one twice.
//
// With topUp, the count already recorded on the starting line for each name is
// subtracted and only the excess is added, so a <cfset> keeps the component its
// own string path resolved against this file's refs while still gaining the
// calls that path never matched. Counting rather than testing presence is what
// keeps `<cfset x = f() + f()>` at two.
func (p *tagParser) mergeExpressionCalls(expr string, line int, topUp bool) {
	if !p.extractCalls || strings.IndexByte(expr, '(') < 0 {
		return
	}

	sub := newScriptParser(expr, p.fileURI, line, p.resolvers).asCFScript()
	sub.resolverSet = p.resolverSet
	sub.extractCalls = true
	sub.parse()

	have := map[string]int{}

	if topUp {
		cs := p.callsOnLine(uint32(line))

		for i := range cs {
			c := &cs[i]

			have[strings.ToLower(c.FuncName)]++
		}
	}

	emit := func(c CallSite) {
		k := strings.ToLower(c.FuncName)
		if have[k] > 0 {
			have[k]--

			return
		}

		p.addCall(&c)
	}

	for i := range sub.calls {
		c := &sub.calls[i]

		emit(*c)
	}

	for _, calls := range sub.funcCalls {
		for i := range calls {
			c := &calls[i]

			emit(*c)
		}
	}
}

// callsOnLine returns the calls already recorded in the current bucket for a
// line. Calls are appended in source order, so the run at the end of the bucket
// is the whole answer and this costs what it finds rather than what the file
// holds.
func (p *tagParser) callsOnLine(line uint32) []CallSite {
	bucket := p.calls
	if p.inFunc != "" {
		bucket = p.funcCalls[p.inFunc]
	}

	i := len(bucket)
	for i > 0 && bucket[i-1].Line == line {
		i--
	}

	return bucket[i:]
}

// tagBody returns what a tag holds between its name and its closing '>': the
// condition of a <cfif>, the expression of a <cfreturn>.
func tagBody(tag string) string {
	inner := tag

	if i := strings.IndexByte(inner, ' '); i >= 0 {
		inner = inner[i+1:]
	} else {
		return ""
	}

	inner = strings.TrimSuffix(inner, ">")
	inner = strings.TrimSuffix(inner, "/")

	return strings.TrimSpace(inner)
}

// trailingCalls returns the names of the method calls chained after the first
// call in expr, in order: for "b.width(1).height(2).build()" it is
// ["height", "build"], for createObject("java", "x").builder().build()
// ["builder", "build"]. It stops at anything that is not another .name(...)
// hop, and returns nil when expr has no complete first call.
func trailingCalls(expr string) []string {
	i := strings.IndexByte(expr, '(')
	if i < 0 {
		return nil
	}

	i = skipCallGroup(expr, i)

	var hops []string

	for i >= 0 {
		j := skipSpace(expr, i)
		if j >= len(expr) || expr[j] != '.' {
			break
		}

		j = skipSpace(expr, j+1)
		start := j

		for j < len(expr) && isIdentByte(expr[j]) {
			j++
		}

		name := expr[start:j]

		j = skipSpace(expr, j)
		if name == "" || j >= len(expr) || expr[j] != '(' {
			break
		}

		hops = append(hops, name)
		i = skipCallGroup(expr, j)
	}

	return hops
}

// skipCallGroup returns the offset just past the ( … ) group opening at open,
// stepping over quoted strings, or -1 when the group does not close.
func skipCallGroup(expr string, open int) int {
	depth := 0

	for i := open; i < len(expr); i++ {
		switch c := expr[i]; c {
		case '"', '\'':
			end := strings.IndexByte(expr[i+1:], c)
			if end < 0 {
				return -1
			}

			i += end + 1
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}

	return -1
}

func skipSpace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}

	return i
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
