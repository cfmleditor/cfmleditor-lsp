package resolve

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cfmleditor/clif/internal/parser"
)

// The plans as they were made before they were made on request: every
// method's at once. The plans made on request must be the same plans.
func producerMethodsEager(content string) map[string]*producerMethod {
	methods := map[string]*producerMethod{}
	tags := []producerTag{}

	for _, region := range parser.ClassifyRegions(content) {
		if region.Kind == parser.RegionScript {
			scriptProducerMethods(region.Text, methods)
			tags = append(tags, producerTag{name: "cfscript", body: region.Text})
		}

		if region.Kind == parser.RegionTag {
			tags = append(tags, producerTags(region.Text)...)
		}
	}

	tagProducerMethodsEager(tags, methods)

	return methods
}

func tagProducerMethodsEager(tags []producerTag, methods map[string]*producerMethod) {
	for i := 0; i < len(tags); i++ {
		if tags[i].name != "cffunction" {
			continue
		}

		name, ok := producerAttr(tags[i].body, "name")
		if !ok {
			continue
		}

		end := i + 1
		for end < len(tags) && tags[end].name != "/cffunction" {
			if tags[end].name == "cffunction" {
				break
			}

			end++
		}

		if end >= len(tags) || tags[end].name != "/cffunction" {
			continue
		}

		method := &producerMethod{defaults: map[string]string{}}

		for _, tag := range tags[i+1 : end] {
			if tag.name != "cfargument" {
				continue
			}

			arg, found := producerAttr(tag.body, "name")
			if !found {
				continue
			}

			arg = strings.ToLower(arg)
			method.parameters = append(method.parameters, arg)

			if def, found := producerAttr(tag.body, "default"); found {
				// A computed default is present even though its value is unknown.
				method.defaults[arg] = "'" + strings.ReplaceAll(def, "'", "''") + "'"
			}
		}

		p := producerTagParser{tags: tags[i+1 : end]}

		if end-i <= 4096 {
			method.body = p.block()
		} else {
			p.failed = true
		}

		if !p.failed && p.position == len(p.tags) {
			method.sensitive = (producerSensitive(method.body) || producerCallBinding(method.body)) && !producerReturnsThis(method.body)
			method.wanted = producerWanted(method.body)
		} else {
			var source strings.Builder
			for _, tag := range tags[i+1 : end] {
				source.WriteString(tag.body)
				source.WriteByte('\n')

				if (tag.name == "cfif" || tag.name == "cfscript") && producerSensitive([]producerNode{{kind: "if", expression: tag.body}}) {
					method.sensitive = true
				}
			}

			method.body = []producerNode{{kind: "unsafe", expression: source.String()}}
		}

		methods[strings.ToLower(name)] = method

		i = end
	}
}

// plansMustMatch fails when a file's plans made on request differ from the
// plans made at once, asked for one at a time or all together.
func plansMustMatch(t *testing.T, name, content string) {
	t.Helper()

	want := producerMethodsEager(content)
	plans := newProducerPlans(content)

	for method, plan := range want {
		if got := plans.get(method); !reflect.DeepEqual(got, plan) {
			t.Errorf("%s: plan of %s made on request differs", name, method)
		}
	}

	if got := plans.get("nosuchmethod"); got != nil {
		t.Errorf("%s: a method the file lacks has a plan", name)
	}

	if got := newProducerPlans(content).all(); !reflect.DeepEqual(got, want) {
		t.Errorf("%s: all plans differ: %d made, want %d", name, len(got), len(want))
	}
}

func TestPlansMadeOnRequestAreThePlansMadeAtOnce(t *testing.T) {
	plansMustMatch(t, "tag and script", `<cfcomponent>
<cffunction name="make"><cfargument name="kind" default="a"><cfif arguments.kind eq "a"><cfreturn new A()></cfif><cfreturn new B()></cffunction>
<cffunction name="twice"><cfreturn new A()></cffunction>
<cffunction name="twice"><cfreturn new B()></cffunction>
<cffunction name="unclosed"><cfreturn 1>
<cfscript>
function scripted(x) { return new C(); }
function make() { return new D(); }
</cfscript>
</cfcomponent>`)

	n := 0

	err := filepath.WalkDir("../../testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".cfc") && !strings.HasSuffix(path, ".cfm") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		plansMustMatch(t, path, string(data))

		n++

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if n == 0 {
		t.Fatal("found no fixtures under ../../testdata")
	}
}
