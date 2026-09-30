package resolve

import (
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

// BeanResolvers augments a configured getBean ID resolver with literal startup
// registrations. It never changes component basename lookup (new Content must
// still mean Content.cfc), or overrides an earlier explicit resolver rule.
// Call before publishing the resolver or indexing application files.
func (r *Resolver) BeanResolvers(beanPaths map[string]string) []parser.Resolver {
	insertion := -1

	for i := range r.Resolvers {
		rule := &r.Resolvers[i]
		if strings.EqualFold(rule.Prefix, "getBean") && rule.Resolve == "$1" {
			insertion = i

			break
		}
	}

	if insertion < 0 || len(r.StartupFiles) == 0 {
		return r.Resolvers
	}

	appDirs := []string{}

	for _, root := range r.WorkspaceFolders {
		if dir := r.FindApplicationRoot(root); dir != "" {
			appDirs = append(appDirs, dir)
		}
	}

	beans := cfpath.BuildBeanMap(cfpath.BeanPathsFor(beanPaths, appDirs), r.fs())
	factories := map[string]map[string][]beanRegistration{}
	seen := map[string]bool{}

	queue := r.configuredStartupFiles()
	for len(queue) > 0 && len(seen) < maxStartupTemplates {
		file := queue[0]
		queue = queue[1:]

		if seen[pathKey(file)] {
			continue
		}

		seen[pathKey(file)] = true

		data, err := r.fs().ReadFile(file)
		if err != nil {
			continue
		}

		content := string(data)

		pr := parser.Parse(cfpath.ToURI(file), content, r.Resolvers)
		for _, call := range beanCalls(content) {
			comp, _ := r.receiverComponent(call.receiver, call.line, "", call.method, pr, filepath.Dir(file), nil)

			name, entry, valid := r.beanRegistration(comp, call, filepath.Dir(file))
			if !valid {
				continue
			}

			factoryKey := strings.ToLower(comp + ":" + call.receiver)

			registrations := factories[factoryKey]
			if registrations == nil {
				registrations = map[string][]beanRegistration{}
				factories[factoryKey] = registrations
			}

			key := strings.ToLower(name)
			registrations[key] = append(registrations[key], entry)
		}

		for _, raw := range parser.ExtractIncludes(content) {
			if target := r.IncludePath(raw, file); target != "" {
				queue = append(queue, target)
			}
		}
	}
	// Different factories may register the same ID. Only publish a type on
	// which every observed registration agrees; never chain across factories.
	paths := map[string]string{}

	for _, registrations := range factories {
		for key := range registrations {
			path := registeredBean(key, registrations, beans, map[string]bool{})
			if old, found := paths[key]; found && !cfpath.SamePath(old, path) {
				paths[key] = ""
			} else if !found {
				paths[key] = path
			}
		}
	}

	keys := make([]string, 0, len(paths))
	for key := range paths {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	discovered := []parser.Resolver{}

	for _, key := range keys {
		path := paths[key]
		if path == "" {
			continue
		}

		discovered = append(discovered, parser.Resolver{
			Match:   `(?i)(?:^|\.)getBean\(\s*(?:\w+\s*[=:]\s*)?["']` + regexp.QuoteMeta(key) + `["']\s*(?:,[^()]*)?\)$`,
			Resolve: path, Prefix: "getBean",
		})
	}

	// Startup parsing may have initialized a rule's sync.Once. Recreate
	// configured rules rather than copying their synchronization state.
	out := make([]parser.Resolver, 0, len(r.Resolvers)+len(discovered))
	for i := range r.Resolvers {
		if i == insertion {
			out = append(out, discovered...)
		}

		rule := &r.Resolvers[i]
		out = append(out, parser.Resolver{
			Match: rule.Match, Resolve: rule.Resolve, Prefix: rule.Prefix,
			NoFollow: rule.NoFollow, Anchored: rule.Anchored,
			DynamicIfMissing: rule.DynamicIfMissing, NameOnly: rule.NameOnly,
		})
	}

	return out
}

type beanRegistration struct {
	target    string
	component bool
}

func registeredBean(name string, registrations map[string][]beanRegistration, beans map[string]string, visiting map[string]bool) string {
	if visiting[name] || len(visiting) >= maxStartupTemplates {
		return ""
	}

	entries, found := registrations[name]
	if !found {
		return beans[name]
	}

	visiting[name] = true
	defer delete(visiting, name)

	path := ""

	for _, entry := range entries {
		next := entry.target
		if !entry.component {
			next = registeredBean(next, registrations, beans, visiting)
		}

		if next == "" || (path != "" && !cfpath.SamePath(path, next)) {
			return ""
		}

		path = next
	}

	return path
}

type beanCall struct {
	receiver, method string
	line             uint32
	args             [][]parser.Token
}

// Use tokens, rather than source regexes, to exclude comments, quoted examples,
// interpolation and computed IDs. Tag templates only contribute cfset calls.
func beanCalls(content string) []beanCall {
	calls := []beanCall{}

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

			method := strings.ToLower(tok.Value)
			if active && tok.Kind == parser.TokIdent && (method == "addalias" || method == "declarebean") && scanner.PeekSkipComments().Kind == parser.TokLParen {
				receiver := beanReceiver(previous)

				scanner.NextSkipComments()

				args := beanArguments(scanner)

				line := region.StartLine + tok.Line
				if receiver != "" && line >= 0 && line <= math.MaxUint32 {
					calls = append(calls, beanCall{receiver: receiver, method: method, line: uint32(line), args: args})
				}

				previous = nil

				continue
			}

			previous = append(previous, tok)
			// Only the immediate dotted receiver is needed; bound storage on large files.
			if len(previous) > 16 {
				previous = previous[1:]
			}
		}
	}

	return calls
}

func beanReceiver(tokens []parser.Token) string {
	end := len(tokens)
	if end < 2 || tokens[end-1].Kind != parser.TokDot || tokens[end-2].Kind != parser.TokIdent {
		return ""
	}

	start := end - 2
	for start >= 2 && tokens[start-1].Kind == parser.TokDot && tokens[start-2].Kind == parser.TokIdent {
		start -= 2
	}
	// Do not mistake a computed or chained receiver's last member for a variable.
	if start > 0 && (tokens[start-1].Kind == parser.TokDot || tokens[start-1].Kind == parser.TokRParen || tokens[start-1].Kind == parser.TokRBracket) {
		return ""
	}

	var receiver strings.Builder
	for _, tok := range tokens[start : end-1] {
		receiver.WriteString(tok.Value)
	}

	return receiver.String()
}

func beanArguments(scanner *parser.Scanner) [][]parser.Token {
	args := [][]parser.Token{{}}
	depth := 0

	for {
		tok := scanner.NextSkipComments()
		if tok.Kind == parser.TokEOF {
			return nil
		}

		switch tok.Kind {
		case parser.TokRParen:
			if depth == 0 {
				return args
			}

			depth--
		case parser.TokLParen, parser.TokLBrace, parser.TokLBracket:
			depth++
		case parser.TokRBrace, parser.TokRBracket:
			depth--
		case parser.TokComma:
			if depth == 0 {
				args = append(args, nil)

				continue
			}
		default:
			// Other tokens are retained so computed arguments fail the literal check.
		}

		args[len(args)-1] = append(args[len(args)-1], tok)
	}
}

func beanArgument(args [][]parser.Token, name string, position int) string {
	for i, arg := range args {
		if len(arg) >= 2 && arg[0].Kind == parser.TokIdent && (arg[1].Kind == parser.TokEquals || arg[1].Kind == parser.TokColon) {
			if !strings.EqualFold(arg[0].Value, name) {
				continue
			}

			arg = arg[2:]
		} else if i != position {
			continue
		}

		if len(arg) != 1 || arg[0].Kind != parser.TokString {
			return ""
		}

		value := arg[0].Value
		if len(value) < 2 || value[0] != value[len(value)-1] {
			return ""
		}

		value = value[1 : len(value)-1]
		if strings.ContainsAny(value, "#\"'") {
			return ""
		}

		return value
	}

	return ""
}

func (r *Resolver) beanRegistration(comp string, call beanCall, dir string) (string, beanRegistration, bool) {
	first, second := "aliasName", "beanName"
	if call.method == "declarebean" {
		first, second = "beanName", "dottedPath"
	}

	def := r.ResolveFunc(comp, call.method, dir)
	// A similarly named method on another API is not a registration.
	if def == nil || len(def.Arguments) < 2 || !strings.EqualFold(def.Arguments[0].Name, first) || !strings.EqualFold(def.Arguments[1].Name, second) || r.ResolveFunc(comp, "getBean", dir) == nil {
		return "", beanRegistration{}, false
	}

	name, target := beanArgument(call.args, first, 0), beanArgument(call.args, second, 1)
	if name == "" {
		return "", beanRegistration{}, false
	}

	entry := beanRegistration{target: strings.ToLower(target)}
	if call.method == "declarebean" {
		entry.component = true
		if target != "" {
			entry.target = r.ComponentPath(target, dir)
		}
	}

	return name, entry, true
}
