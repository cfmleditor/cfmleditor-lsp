package resolve

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

// Wheels' controller( name ) returns the controller class for name, found the
// way $createControllerClass finds it: the first path in the controllerPath
// list holding <name>.cfc, or, when none does, the last path's own
// Controller.cfc. A controller with no file of its own is the base class, so
// controller( "dummy" ) in a test is the test assets' Controller.cfc.
//
// Both methods are checked against the pinned source before anything is read
// from the call, and the list comes from a literal write of controllerPath:
// the nearest one above the calling file (a test runner's
// set( controllerPath = AssetPath & "controllers" ) for its specs), or the
// framework's default when there is none. A computed write in that place, a
// computed name, or a candidate with no file withholds the type.

// wheelsCreateControllerClass is $createControllerClass's body at the pinned
// commit, comments removed.
const wheelsCreateControllerClass = `local.controllerPathsArray = ListToArray(arguments.controllerPaths);
local.iEnd = ArrayLen(local.controllerPathsArray);
for (local.i = 1; local.i <= local.iEnd; local.i++) {
	local.controllerPath = local.controllerPathsArray[local.i];
	local.fileName = $objectFileName(name = arguments.name, objectPath = local.controllerPath, type = arguments.type);
	if (local.fileName != "Controller" || local.i == ArrayLen(local.controllerPathsArray)) {
		application.wheels.controllers[arguments.name] = $createObjectFromRoot(
			path = local.controllerPath,
			fileName = local.fileName,
			method = "$initControllerClass",
			name = arguments.name
		);
		local.rv = application.wheels.controllers[arguments.name];
		break;
	}
}
return local.rv;`

func (r *Resolver) wheelsControllerReturn(global string, controller *parser.FunctionDef, args [][]parser.Token, baseDir string) string {
	name := beanArgument(args, "name", 0)
	if name == "" || strings.ContainsAny(name, "/\\ ") {
		return ""
	}

	source := r.wheelsSource(controller.URI.Path()).methods
	if !strings.Contains(source["controller"].body, wheelsTokens(`execute = "$createControllerClass"`)) ||
		!strings.Contains(source["controller"].body, wheelsTokens(`local.rv = local.rv.$createControllerObject(arguments.params);`)) {
		return ""
	}

	create := r.wheelsPlanFunc(global, "$createControllerClass")
	if create == nil || r.wheelsSource(create.URI.Path()).methods["$createcontrollerclass"].body != wheelsTokens(wheelsCreateControllerClass) {
		return ""
	}

	configs, ok := r.wheelsControllerPaths(baseDir, filepath.Dir(global))
	if !ok {
		return ""
	}

	var targets []string

	for _, config := range configs {
		target := r.wheelsControllerClass(name, config)
		if target == "" {
			return ""
		}

		if !slices.Contains(targets, target) {
			targets = append(targets, target)
		}
	}

	return strings.Join(targets, "|")
}

// wheelsControllerClass is the file $createControllerClass instantiates for
// name with this controllerPath list.
func (r *Resolver) wheelsControllerClass(name, config string) string {
	paths := strings.Split(config, ",")
	for i, path := range paths {
		dir := strings.ReplaceAll(strings.Trim(strings.TrimSpace(path), "/"), "/", ".")
		if dir == "" {
			return ""
		}

		if file := r.ComponentPath(dir+"."+name, ""); file != "" {
			return file
		}

		if i == len(paths)-1 {
			return r.ComponentPath(dir+".Controller", "")
		}
	}

	return ""
}

// wheelsControllerPaths is every controllerPath value that governs a call made
// from dir: the literal writes in the nearest directory above it that makes
// any, else the framework's own default. ok is false when a write there is
// computed.
func (r *Resolver) wheelsControllerPaths(dir, framework string) ([]string, bool) {
	for d := dir; ; d = filepath.Dir(d) {
		if !r.inWorkspace(d) {
			break
		}

		values, found, ok := r.controllerPathWrites(d)
		if !ok {
			return nil, false
		}

		if found {
			return values, true
		}

		if filepath.Dir(d) == d {
			break
		}
	}

	values, found, ok := r.controllerPathWrites(filepath.Join(framework, "events", "init"))
	if !ok || !found {
		return nil, false
	}

	return values, true
}

func (r *Resolver) inWorkspace(dir string) bool {
	if len(r.WorkspaceFolders) == 0 {
		return true
	}

	for _, root := range r.WorkspaceFolders {
		if cfpath.SamePath(dir, root) || strings.HasPrefix(dir, strings.TrimSuffix(root, string(filepath.Separator))+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

type controllerPaths struct {
	values    []string
	found, ok bool
}

// controllerPathWrites reads the CFML files directly in dir for writes of
// controllerPath: `x.controllerPath = …` and `set( controllerPath = … )`.
// found says whether there were any; ok is false when one is not a literal.
// Every call from a directory walks the same ancestors, so the answer is kept
// per directory until InvalidatePaths.
func (r *Resolver) controllerPathWrites(dir string) (values []string, found, ok bool) {
	key := pathKey(dir)

	r.mu.RLock()
	cached, hit := r.ctlPathCache[key]
	r.mu.RUnlock()

	if hit {
		return cached.values, cached.found, cached.ok
	}

	values, found, ok = r.readControllerPathWrites(dir)

	r.mu.Lock()
	if r.ctlPathCache == nil {
		r.ctlPathCache = map[string]controllerPaths{}
	}

	r.ctlPathCache[key] = controllerPaths{values: values, found: found, ok: ok}
	r.mu.Unlock()

	return values, found, ok
}

func (r *Resolver) readControllerPathWrites(dir string) (values []string, found, ok bool) {
	entries, err := r.fs().ReadDir(dir)
	if err != nil {
		return nil, false, true
	}

	for _, entry := range entries {
		if entry.IsDir() || !cfpath.IsCFMLFile(entry.Name()) {
			continue
		}

		data, err := r.fs().ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil || !strings.Contains(strings.ToLower(string(data)), "controllerpath") {
			continue
		}

		tokens := producerTokens(string(data))
		if tokens == nil {
			return nil, true, false
		}

		for i := range tokens {
			value, write := controllerPathWrite(tokens, i)
			if !write {
				continue
			}

			found = true

			literal, known := literalConcat(tokens, value)
			if !known {
				return nil, true, false
			}

			if !slices.Contains(values, literal) {
				values = append(values, literal)
			}
		}
	}

	return values, found, true
}

// controllerPathWrite reports whether tokens[i] names controllerPath as the
// target of a write, and the tokens of the value written.
func controllerPathWrite(tokens []parser.Token, i int) ([]parser.Token, bool) {
	if tokens[i].Kind != parser.TokIdent || !strings.EqualFold(tokens[i].Value, "controllerPath") || i+1 >= len(tokens) || tokens[i+1].Kind != parser.TokEquals || i+2 < len(tokens) && tokens[i+2].Kind == parser.TokEquals {
		return nil, false
	}

	member := i > 0 && tokens[i-1].Kind == parser.TokDot
	named := i > 1 && (tokens[i-1].Kind == parser.TokLParen || tokens[i-1].Kind == parser.TokComma)

	if !member && !named {
		return nil, false
	}

	end := i + 2
	for depth := 0; end < len(tokens); end++ {
		switch tokens[end].Kind {
		case parser.TokLParen, parser.TokLBracket, parser.TokLBrace:
			depth++
		case parser.TokRParen, parser.TokRBracket, parser.TokRBrace:
			if depth == 0 {
				return tokens[i+2 : end], true
			}

			depth--
		case parser.TokComma, parser.TokSemicolon:
			if depth == 0 {
				return tokens[i+2 : end], true
			}
		default:
		}

		if depth == 0 && end > i+2 && tokens[end].Line != tokens[end-1].Line && tokens[end-1].Kind != parser.TokAmpersand && tokens[end].Kind != parser.TokAmpersand {
			return tokens[i+2 : end], true
		}
	}

	return tokens[i+2 : end], true
}

// literalConcat evaluates value as string literals joined by `&`, where a name
// is a variable the file assigns exactly one literal.
func literalConcat(tokens, value []parser.Token) (string, bool) {
	if len(value) == 0 {
		return "", false
	}

	var out strings.Builder

	for i := 0; i < len(value); {
		part, next, ok := literalPart(tokens, value, i)
		if !ok {
			return "", false
		}

		out.WriteString(part)

		if next == len(value) {
			return out.String(), true
		}

		if value[next].Kind != parser.TokAmpersand {
			return "", false
		}

		i = next + 1
	}

	return "", false
}

func literalPart(tokens, value []parser.Token, i int) (string, int, bool) {
	if value[i].Kind == parser.TokString {
		text := value[i].Value
		if len(text) < 2 || strings.Contains(text, "#") {
			return "", 0, false
		}

		return text[1 : len(text)-1], i + 1, true
	}

	if value[i].Kind != parser.TokIdent {
		return "", 0, false
	}

	name := value[i].Value
	next := i + 1

	if strings.EqualFold(name, "local") && next+1 < len(value) && value[next].Kind == parser.TokDot && value[next+1].Kind == parser.TokIdent {
		name = value[next+1].Value
		next += 2
	}

	literal, ok := soleLiteralAssignment(tokens, name)

	return literal, next, ok
}

// soleLiteralAssignment is the one string literal the file assigns to name
// (bare, var-declared or local.-scoped), and false if it assigns it anything
// else or more than one value.
func soleLiteralAssignment(tokens []parser.Token, name string) (string, bool) {
	found := ""
	seen := false

	for i := range tokens {
		if tokens[i].Kind != parser.TokIdent || !strings.EqualFold(tokens[i].Value, name) || i+1 >= len(tokens) || !producerWrites(tokens, i+1) {
			continue
		}

		if i > 0 && tokens[i-1].Kind == parser.TokDot && (i < 2 || !strings.EqualFold(tokens[i-2].Value, "local")) {
			continue
		}

		if tokens[i+1].Kind != parser.TokEquals || i+2 >= len(tokens) || tokens[i+2].Kind != parser.TokString || i+3 < len(tokens) && tokens[i+3].Kind == parser.TokAmpersand {
			return "", false
		}

		text := tokens[i+2].Value
		if len(text) < 2 || strings.Contains(text, "#") {
			return "", false
		}

		literal := text[1 : len(text)-1]
		if seen && literal != found {
			return "", false
		}

		found, seen = literal, true
	}

	return found, seen
}

// producerWrites reports whether tokens[j:] begins with an assignment: `=`
// but not `==`, a compound `+=`, `&=` and the like, or `++`/`--`.
func producerWrites(tokens []parser.Token, j int) bool {
	if j >= len(tokens) {
		return false
	}

	next := parser.TokEOF
	if j+1 < len(tokens) {
		next = tokens[j+1].Kind
	}

	switch tokens[j].Kind {
	case parser.TokEquals:
		return next != parser.TokEquals
	case parser.TokPlus, parser.TokMinus:
		return next == parser.TokEquals || next == tokens[j].Kind
	case parser.TokStar, parser.TokSlash, parser.TokAmpersand, parser.TokPercent, parser.TokCaret:
		return next == parser.TokEquals
	default:
		return false
	}
}
