package resolve

import (
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

type fw1Scope struct {
	managed, services, subsystems bool
}

// FW/1 constructs controllers/services outside its bean factory's search roots,
// then autowires their setters using that factory. Recognize the source-backed
// convention, not every folder named controllers or every Application setter.
func (r *Resolver) fw1SetterManaged(file string) bool {
	kind := filepath.Base(filepath.Dir(file))
	if !strings.EqualFold(kind, "controllers") && !strings.EqualFold(kind, "services") {
		return false
	}

	appDir := r.FindApplicationRoot(filepath.Dir(file))
	if appDir == "" {
		return false
	}

	r.mu.RLock()
	scope, found := r.fw1Scopes[appDir]
	r.mu.RUnlock()

	if !found {
		scope = r.readFW1Scope(appDir)
		r.mu.Lock()
		if r.fw1Scopes == nil {
			r.fw1Scopes = map[string]fw1Scope{}
		}

		r.fw1Scopes[appDir] = scope
		r.mu.Unlock()
	}

	if !scope.managed {
		return false
	}

	if strings.EqualFold(kind, "services") && !scope.services {
		return false
	}

	rel, err := filepath.Rel(appDir, file)
	if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}

	parts := strings.Split(filepath.ToSlash(rel), "/")
	if scope.subsystems {
		return len(parts) == 3
	}

	return len(parts) == 2
}

func (r *Resolver) readFW1Scope(appDir string) fw1Scope {
	file := filepath.Join(appDir, "Application.cfc")

	data, err := r.fs().ReadFile(file)
	if err != nil {
		return fw1Scope{}
	}

	pr := parser.ParseWithOptions(cfpath.ToURI(file), string(data), &parser.ParseOptions{ExtractCalls: true, ScanAllScopes: true})

	base := r.ComponentPath(pr.Extends, appDir)
	if pr.Extends == "" || base == "" {
		return fw1Scope{}
	}

	data, err = r.fs().ReadFile(base)
	if err != nil {
		return fw1Scope{}
	}

	framework := parser.ParseWithOptions(cfpath.ToURI(base), string(data), &parser.ParseOptions{ExtractCalls: true, ScanAllScopes: true})

	methods := map[string]bool{}
	for i := range framework.Funcs {
		methods[strings.ToLower(framework.Funcs[i].Name)] = true
	}
	// Require the framework's real implementation, including the injection
	// method, rather than trusting an extends basename or a preset stub.
	for _, name := range []string{"setbeanfactory", "getbeanfactory", "getcontroller", "autowire"} {
		if !methods[name] {
			return fw1Scope{}
		}
	}

	controllers, services := fw1Autowires(framework)
	if !controllers {
		return fw1Scope{}
	}

	factory := false

	for i := range pr.Funcs {
		if strings.EqualFold(pr.Funcs[i].Name, "setBeanFactory") {
			return fw1Scope{}
		}
	}

	calls := pr.AllCalls()
	for i := range calls {
		call := &calls[i]
		if strings.EqualFold(call.FuncName, "setBeanFactory") && (call.Variable == "" || strings.EqualFold(call.Variable, "this")) && (call.Caller == "" || strings.EqualFold(call.Caller, "setupApplication")) {
			factory = true
		}
		// A separate subsystem factory need not expose the workspace beans.
		if strings.EqualFold(call.FuncName, "setSubsystemBeanFactory") {
			return fw1Scope{}
		}
	}

	if !factory {
		return fw1Scope{}
	}

	settings := fw1Settings(pr.Content)
	if len(settings["usingsubsystems"]) == 0 && (len(settings["defaultsubsystem"]) > 0 || len(settings["sitewidelayoutsubsystem"]) > 0) {
		return fw1Scope{}
	}

	if values := settings["usingsubsystems"]; len(values) > 0 {
		for _, value := range values {
			if value != "true" && value != "false" || value != values[0] {
				return fw1Scope{}
			}
		}
	}
	// Only the conventional base, directly at this Application, is modeled.
	// Static defaults may be repeated inside setup methods. Runtime expressions
	// do not identify another on-disk scope; conflicting literal bases do.
	staticBase := len(settings["base"]) == 0
	for _, value := range settings["base"] {
		value = r.staticPath(value)
		if value == "" {
			continue
		}

		if !r.fw1BaseAtApplication(value, appDir) {
			return fw1Scope{}
		}

		staticBase = true
	}

	if !staticBase || len(settings["unsupported"]) > 0 {
		return fw1Scope{}
	}

	for _, key := range []string{"controllersfolder", "servicesfolder", "subsystemsfolder"} {
		for _, value := range settings[key] {
			if key == "subsystemsfolder" || value != strings.TrimSuffix(key, "folder") {
				return fw1Scope{}
			}
		}
	}
	// FW/1 4 moved subsystems beneath subsystemsFolder and no longer creates
	// services. Only the older getCachedComponent layout is modeled here.
	if !services && len(settings["usingsubsystems"]) > 0 && settings["usingsubsystems"][0] == "true" {
		return fw1Scope{}
	}

	return fw1Scope{managed: true, services: services, subsystems: len(settings["usingsubsystems"]) > 0 && settings["usingsubsystems"][0] == "true"}
}

func fw1Autowires(pr *parser.ParseResult) (controllers, services bool) {
	calls := pr.AllCalls()
	for i := range calls {
		call := &calls[i]
		if strings.EqualFold(call.FuncName, "autowire") && call.Variable == "" {
			switch strings.ToLower(call.Caller) {
			case "getcachedcomponent":
				controllers, services = true, true
			case "getcachedcontroller":
				controllers = true
			}
		}
	}

	return controllers, services
}

// Extract only direct framework settings, using scanner tokens so comments and
// quoted examples cannot manufacture a managed scope. Other config forms remain
// unsupported rather than guessing a subsystem layout.
func fw1Settings(content string) map[string][]string {
	out := map[string][]string{}

	for _, region := range parser.ClassifyRegions(content) {
		if region.Kind == parser.RegionSkip {
			continue
		}

		scanner := parser.NewScanner(region.Text)
		previous := []parser.Token{}
		active := region.Kind == parser.RegionScript

		for {
			tok := scanner.NextSkipComments()
			if tok.Kind == parser.TokEOF {
				break
			}

			if region.Kind == parser.RegionTag {
				if tok.Kind == parser.TokIdent && strings.EqualFold(tok.Value, "cfset") && len(previous) > 0 && previous[len(previous)-1].Kind == parser.TokLT {
					active = true
					previous = nil

					continue
				}

				if tok.Kind == parser.TokGT {
					active = false
					previous = nil

					continue
				}
			}

			if active && tok.Kind == parser.TokEquals && len(previous) > 0 && strings.EqualFold(previous[len(previous)-1].Value, "framework") && scanner.PeekSkipComments().Kind == parser.TokLBrace {
				out["unsupported"] = append(out["unsupported"], "struct config")
			}

			if active && tok.Kind == parser.TokEquals {
				if key := fw1SettingName(previous); key != "" {
					out[key] = append(out[key], fw1SettingValue(scanner))
					previous = nil

					continue
				}
			}

			previous = append(previous, tok)
			if len(previous) > 7 {
				previous = previous[1:]
			}
		}
	}

	return out
}

func fw1SettingName(tokens []parser.Token) string {
	n := len(tokens)
	if n < 3 || tokens[n-1].Kind != parser.TokIdent || tokens[n-2].Kind != parser.TokDot || !strings.EqualFold(tokens[n-3].Value, "framework") {
		return ""
	}

	if n >= 4 && tokens[n-4].Kind == parser.TokDot {
		if n < 5 || !strings.EqualFold(tokens[n-5].Value, "variables") || n >= 6 && tokens[n-6].Kind == parser.TokDot {
			return ""
		}
	}

	key := strings.ToLower(tokens[n-1].Value)
	switch key {
	case "base", "usingsubsystems", "defaultsubsystem", "sitewidelayoutsubsystem", "controllersfolder", "servicesfolder", "subsystemsfolder":
		return key
	default:
		return ""
	}
}

func fw1SettingValue(scanner *parser.Scanner) string {
	value := scanner.NextSkipComments()

	end := scanner.PeekSkipComments().Kind
	if (value.Kind != parser.TokString && value.Kind != parser.TokIdent) || (end != parser.TokSemicolon && end != parser.TokGT && end != parser.TokRBrace && end != parser.TokEOF) {
		return "#unknown#"
	}

	if value.Kind == parser.TokString && len(value.Value) >= 2 {
		return value.Value[1 : len(value.Value)-1]
	}

	return strings.ToLower(value.Value)
}

func (r *Resolver) fw1BaseAtApplication(base, appDir string) bool {
	if !strings.HasPrefix(base, "/") {
		return cfpath.SamePath(filepath.Join(appDir, filepath.FromSlash(base)), appDir)
	}

	trimmed := strings.Trim(base, "/")

	segment, rest, _ := strings.Cut(trimmed, "/")
	for key, root := range r.EffectiveMappings(appDir) {
		if strings.EqualFold(strings.Trim(key, "/"), segment) && cfpath.SamePath(filepath.Join(root, filepath.FromSlash(rest)), appDir) {
			return true
		}
	}

	for _, root := range r.WorkspaceFolders {
		if cfpath.SamePath(filepath.Join(root, filepath.FromSlash(trimmed)), appDir) {
			return true
		}
	}

	return false
}
