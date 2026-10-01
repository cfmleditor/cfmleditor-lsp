package resolve

import (
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

func wheelsCallName(expression string) string {
	scanner := parser.NewScanner(expression)
	name := ""

	for {
		tok := scanner.NextSkipComments()
		if tok.Kind == parser.TokEOF || tok.Kind == parser.TokLParen {
			return name
		}

		if tok.Kind == parser.TokIdent {
			name = tok.Value
		}
	}
}

// Factory argument evidence is read only after a real method is found. No API
// is inferred from the names model/controller on an unrelated application.
func (r *Resolver) wheelsFactoryReturn(def *parser.FunctionDef, expression, baseDir string) string {
	if def == nil || !def.URI.IsFile() {
		return ""
	}

	switch strings.ToLower(def.ReturnType) {
	case "", "any", "struct", "component", "object":
	default:
		return ""
	}

	scanner := parser.NewScanner(expression)
	for scanner.PeekSkipComments().Kind != parser.TokLParen {
		if scanner.NextSkipComments().Kind == parser.TokEOF {
			return ""
		}
	}

	scanner.NextSkipComments()

	args := beanArguments(scanner)
	if args == nil || scanner.NextSkipComments().Kind != parser.TokEOF {
		return ""
	}

	global := r.ComponentPath("wheels.Global", baseDir)
	if global == "" {
		return ""
	}

	source := r.wheelsSource(def.URI.Path()).methods[strings.ToLower(def.Name)]

	original := r.wheelsPlanFunc(global, def.Name)
	if original != nil && original.URI == def.URI {
		kind := strings.ToLower(def.Name)
		if kind == "$createobjectfromroot" {
			if source.body != wheelsTokens(wheelsRootFactory) {
				return ""
			}

			return r.wheelsConstructedFactory(args, baseDir)
		}
	}
	// The mapper-spec wrapper merges literal config with call arguments. Its
	// argument-sensitive result is not stored as an unconditional method return.
	if strings.ReplaceAll(source.body, ";\t", "") == wheelsTokens(`local.args=Duplicate(config) StructAppend(local.args,arguments,true) return application.wo.$createObjectFromRoot(argumentCollection=local.args)`) && r.wheelsTestType(def.URI.Path()) {
		factory := r.wheelsPlanFunc(global, "$createObjectFromRoot")
		if factory == nil {
			return ""
		}

		config := r.wheelsFactoryConfig(def.URI.Path())
		if config == nil {
			return ""
		}

		for _, arg := range args {
			if len(arg) == 0 {
				continue
			}

			if len(arg) != 3 || arg[0].Kind != parser.TokIdent || arg[1].Kind != parser.TokEquals || arg[2].Kind != parser.TokString {
				return ""
			}

			value := beanArgument([][]parser.Token{arg}, arg[0].Value, 0)
			if value == "" {
				return ""
			}

			config[strings.ToLower(arg[0].Value)] = value
		}

		var merged [][]parser.Token
		for key, value := range config {
			merged = append(merged, []parser.Token{{Kind: parser.TokIdent, Value: key}, {Kind: parser.TokEquals, Value: "="}, {Kind: parser.TokString, Value: `"` + value + `"`}})
		}

		return r.wheelsFactoryReturn(factory, "$createObjectFromRoot("+wheelsNamedArgs(merged)+")", baseDir)
	}

	return ""
}

func (r *Resolver) wheelsConstructedFactory(args [][]parser.Token, baseDir string) string {
	path := beanArgument(args, "path", 0)
	name := beanArgument(args, "fileName", 1)
	method := beanArgument(args, "method", 2)

	if path == "" || name == "" || strings.ContainsAny(path+name, "#\\") {
		return ""
	}

	component := strings.ReplaceAll(strings.Trim(path, "/"), "/", ".") + "." + strings.ReplaceAll(name, "/", ".")

	file := r.ComponentPath(component, baseDir)
	if file == "" {
		return ""
	}

	if method == "" {
		return ""
	}

	def := r.ResolveFunc(component, method, baseDir)
	if def == nil {
		return ""
	}

	methods := r.wheelsSource(def.URI.Path()).methods
	if wheelsReturnsMapper(strings.ToLower(def.Name), methods) {
		return file
	}

	return ""
}

func wheelsNamedArgs(args [][]parser.Token) string {
	parts := make([]string, 0, len(args))

	for _, arg := range args {
		var s strings.Builder
		for _, token := range arg {
			s.WriteString(token.Value)
		}

		parts = append(parts, s.String())
	}

	return strings.Join(parts, ",")
}

// Only the observed two-use config contract qualifies: one literal assignment
// and Duplicate(config) in the wrapper. Mutation, aliases, extra users and
// local declarations withhold the result rather than guessing a lifecycle.
func (r *Resolver) wheelsFactoryConfig(file string) map[string]string {
	data, err := r.fs().ReadFile(file)
	if err != nil {
		return nil
	}

	scanner := parser.NewScanner(string(data))
	uses := 0

	var config map[string]string

	previous := ""

	for {
		tok := scanner.NextSkipComments()
		if tok.Kind == parser.TokEOF {
			if uses != 2 {
				return nil
			}

			return config
		}

		if tok.Kind != parser.TokIdent || !strings.EqualFold(tok.Value, "config") {
			previous = tok.Value

			continue
		}

		uses++

		if scanner.PeekSkipComments().Kind == parser.TokEquals {
			if config != nil || strings.EqualFold(previous, "var") || previous == "." {
				return nil
			}

			scanner.NextSkipComments()

			config = wheelsLiteralConfig(scanner)
			if config == nil {
				return nil
			}
		}

		previous = tok.Value
	}
}

func (r *Resolver) wheelsFixedMapperReturn(def *parser.FunctionDef) string {
	if def == nil || !strings.EqualFold(def.Name, "mapper") || !def.URI.IsFile() {
		return ""
	}

	global := r.ComponentPath("wheels.Global", filepath.Dir(def.URI.Path()))
	if global == "" {
		return ""
	}

	switch strings.ToLower(def.ReturnType) {
	case "", "any", "struct", "component", "object":
	default:
		return ""
	}

	original := r.wheelsPlanFunc(global, "mapper")
	if original == nil || original.URI != def.URI {
		return ""
	}

	body := r.wheelsSource(def.URI.Path()).methods["mapper"].body
	if body != wheelsTokens(`return application[$appKey()].mapper.$draw(argumentCollection=arguments);`) {
		return ""
	}

	path := r.ComponentPath("wheels.Mapper", filepath.Dir(global))
	startup := filepath.Join(filepath.Dir(global), "events", "onapplicationstart.cfc")

	data, err := r.fs().ReadFile(startup)
	if err != nil || !strings.Contains(wheelsTokens(string(data)), wheelsTokens(`application.$wheels.mapper=application.wo.$createObjectFromRoot(path="wheels",fileName="Mapper",method="$init");`)) {
		return ""
	}

	if fd, ok := r.wheelsMapperFunc(path, "$draw"); !ok || fd == nil || !samePath(fd.ReturnComponent, path) {
		return ""
	}

	return path
}

// A direct factory chain keeps its ordinary call identity. Recover argument
// evidence only if exactly one matching root call occurs on its recorded line;
// two different factories on one line must not borrow each other's arguments.
func factoryCallExpression(content, name string, line int) string {
	sc := parser.NewScanner(content)
	expression := ""

	for {
		tok := sc.NextSkipComments()
		if tok.Kind == parser.TokEOF {
			return expression
		}

		if tok.Line != line || tok.Kind != parser.TokIdent || !strings.EqualFold(tok.Value, name) || sc.PeekSkipComments().Kind != parser.TokLParen {
			continue
		}

		if expression != "" {
			return ""
		}

		start := tok.Offset

		sc.NextSkipComments()

		depth := 1
		for depth > 0 {
			end := sc.NextSkipComments()
			switch end.Kind {
			case parser.TokEOF:
				return ""
			case parser.TokLParen:
				depth++
			case parser.TokRParen:
				depth--
			default:
			}

			if depth == 0 {
				expression = content[start : end.Offset+1]
			}
		}
	}
}

func wheelsLiteralConfig(scanner *parser.Scanner) map[string]string {
	if scanner.NextSkipComments().Kind != parser.TokLBrace {
		return nil
	}

	config := make(map[string]string)

	for {
		key := scanner.NextSkipComments()
		if key.Kind != parser.TokIdent || scanner.NextSkipComments().Kind != parser.TokEquals {
			return nil
		}

		val := scanner.NextSkipComments()
		if val.Kind != parser.TokString || strings.ContainsAny(val.Value, "#\\") {
			return nil
		}

		config[strings.ToLower(key.Value)] = strings.Trim(val.Value, `"'`)

		end := scanner.NextSkipComments()
		if end.Kind == parser.TokRBrace {
			return config
		}

		if end.Kind != parser.TokComma {
			return nil
		}
	}
}

// Exact observed root factory contract; a matching component expression alone
// does not establish what a modified factory returns.
const wheelsRootFactory = `
		local.method = arguments.method;
		local.component = ListChangeDelims(arguments.path, ".", "/") & "." & ListChangeDelims(arguments.fileName, ".", "/");
		local.argumentCollection = arguments;
		if (local.method EQ 'init') {
			local.rv = application.wheelsdi.getInstance(name = "#local.component#", initArguments = local.argumentCollection);
		} else if (local.method EQ '$initModelObject' && !application.wheelsdi.hasExplicitMapping("#local.component#")) {
			local.instance = CreateObject("component", local.component);
			local.rv = local.instance.$initModelObject(
				name = local.argumentCollection.name,
				properties = local.argumentCollection.properties,
				persisted = local.argumentCollection.persisted,
				row = local.argumentCollection.row ?: 1,
				base = local.argumentCollection.base ?: true,
				useFilterLists = local.argumentCollection.useFilterLists ?: true
			);
			if (StructKeyExists(local.instance, "onDIcomplete")) {
				local.instance.onDIcomplete();
			}
		} else {
			local.instance = application.wheelsdi.getInstance(name = "#local.component#");
			local.rv = Invoke(local.instance, local.method, local.argumentCollection);
		}
		return local.rv;`
