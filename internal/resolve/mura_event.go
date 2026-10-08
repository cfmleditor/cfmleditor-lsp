package resolve

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
)

// muraEventComponent is what Mura hands its code as `event`: a servletEvent
// on the front end, an event elsewhere. They share an API without sharing a
// base, so the receiver is both, and a method either declares is found.
const muraEventComponent = "mura.servletEvent|mura.event"

// muraEvent is muraEventComponent when variable is `event` (bare,
// arguments. or variables.) in code Mura itself calls with one, else "":
//
//   - Mura's own source, the directory holding mura/event.cfc and
//     mura/servletEvent.cfc: its handlers, renderer and API;
//   - a display object, a .cfm the mura preset runs inside the content
//     renderer (ImplicitExtends);
//   - a plugin's event handler, a component extending
//     pluginGenericEventHandler.
//
// It is not a rule about the name. A project's own file holding a variable
// called event is left as it was; only these files are Mura's to say what
// event is. comp is what a function-scoped ref already gave: a local Mura
// event is widened to both classes, and anything else is kept.
func (r *Resolver) muraEvent(variable, comp string, line uint32, caller string, pr *parser.ParseResult) string {
	if !strings.EqualFold(parser.StripReceiverScope(variable), "event") || strings.Count(variable, ".") > 1 {
		return ""
	}

	if comp != "" && !isMuraEvent(comp) {
		return ""
	}

	if pr == nil || !r.handsMuraEvent(cfpath.FromURI(string(pr.URI)), pr) {
		return ""
	}

	// A local the function declares for itself is its own value, unless it is
	// assigned the event: contentRendererUtility takes
	// `var event = renderer.getEvent()`, while contentIntervalManager loops
	// with `var event = events.next()`.
	if comp == "" && !strings.Contains(variable, ".") {
		if start, ok := declaresLocal(pr, line, caller, "event"); ok {
			rhs, _ := localAssignment(pr.Content, "event", start, int(line)+1)
			if !muraEventSourceRe.MatchString(rhs) {
				return ""
			}
		}
	}

	return muraEventComponent
}

// isMuraEvent reports whether comp names Mura's event or servletEvent, by
// dot-path or by file.
func isMuraEvent(comp string) bool {
	for alt := range strings.SplitSeq(comp, "|") {
		name := strings.TrimSuffix(filepath.Base(filepath.ToSlash(alt)), filepath.Ext(alt))
		if i := strings.LastIndexByte(alt, '.'); !strings.HasSuffix(strings.ToLower(alt), ".cfc") && i >= 0 {
			name = alt[i+1:]
		}

		if !strings.EqualFold(name, "event") && !strings.EqualFold(name, "servletEvent") {
			return false
		}
	}

	return true
}

// handsMuraEvent reports whether Mura calls the code in path with an event;
// see muraEvent. Remembered per file.
func (r *Resolver) handsMuraEvent(path string, pr *parser.ParseResult) bool {
	if path == "" {
		return false
	}

	o := r.owner()
	key := pathKey(path)

	o.mu.RLock()
	ok, seen := o.muraEventFiles[key]
	o.mu.RUnlock()

	if seen {
		return ok
	}

	ok = r.inMuraSource(path) || r.extendsFor("", path) == "mura.content.contentRenderer" && strings.EqualFold(filepath.Ext(path), ".cfm") ||
		strings.HasSuffix(strings.ToLower(pr.Extends), "plugingenericeventhandler")

	o.mu.Lock()
	if o.muraEventFiles == nil {
		o.muraEventFiles = map[string]bool{}
	}

	o.muraEventFiles[key] = ok
	o.mu.Unlock()

	return ok
}

// inMuraSource reports whether path is under the directory holding Mura's
// event.cfc and servletEvent.cfc, as mura.event resolves from path.
func (r *Resolver) inMuraSource(path string) bool {
	dir := filepath.Dir(path)

	ev := r.ComponentPath("mura.event", dir)
	if ev == "" || !strings.EqualFold(filepath.Base(ev), "event.cfc") {
		return false
	}

	se := r.ComponentPath("mura.servletEvent", dir)
	if se == "" || filepath.Dir(se) != filepath.Dir(ev) {
		return false
	}

	root := filepath.Dir(ev) + string(filepath.Separator)

	return strings.HasPrefix(strings.ToLower(path), strings.ToLower(root))
}

// muraEventSourceRe is an expression that hands back Mura's event: a
// renderer's getEvent(), a MuraScope's event(), or a new one.
var muraEventSourceRe = regexp.MustCompile(`(?i)(?:^|\.)(?:getEvent|event)\(\s*\)$|mura\.(?:servlet)?event["']`)

// declaresLocal reports whether the function holding line declares name as a
// local of its own, not an argument, and the line that function starts on.
func declaresLocal(pr *parser.ParseResult, line uint32, caller, name string) (int, bool) {
	if argumentOf(pr, caller, name) != nil {
		return 0, false
	}

	scope := parser.FindFuncScopeAt(int(line), pr.Scopes)
	if scope.Start == -1 {
		return 0, false
	}

	for _, v := range pr.FuncVars(scope.Start, scope.End) {
		if strings.EqualFold(v, name) {
			return scope.Start, true
		}
	}

	return 0, false
}
