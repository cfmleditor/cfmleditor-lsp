package parser

import "testing"

func TestTagFactoryChainReceiver(t *testing.T) {
	options := &ParseOptions{
		Resolvers:    []Resolver{{Match: `(?i)(?:^|\.)getBean\(\s*["']([A-Za-z_][\w.]*)["']\s*\)$`, Resolve: "models.$1", Prefix: "getBean"}},
		FuncLookup:   returnedFactoryLookup,
		ExtractCalls: true,
	}

	for _, tc := range []struct{ expression, want string }{
		{`application.factory.getBean('Builder').init()`, "models.Builder"},
		{`application.factory.getBean('Builder').configure().build()`, "models.Product"},
		{`application.factory.getBean('Builder').missing()`, ""},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			pr := ParseWithOptions(testURI, `<cfset $=`+tc.expression+`><cfset $.work()>`, options)
			if got := refsByVar(pr)["$"]; got != tc.want {
				t.Fatalf("receiver=%q want %q", got, tc.want)
			}
		})
	}
}

func TestRecordReceiverMethodFlow(t *testing.T) {
	lookup := func(component, method string) string {
		if component == "models.Content" && method == "save" {
			return component
		}

		if component == "models.Manager" && method == "getContent" {
			return "models.Content"
		}

		return ""
	}

	for _, tc := range []struct{ name, writes, want string }{
		{"self update", `arguments.rc.contentBean=new models.Content(); rc.contentBean=arguments.rc.contentBean.save();`, "models.Content"},
		{"record producer", `rc.manager=new models.Manager(); arguments.rc.contentBean=rc.manager.getContent();`, "models.Content"},
		{"unknown method", `rc.contentBean=new models.Content(); rc.contentBean=rc.contentBean.missing();`, ""},
		{"unknown seed", `rc.contentBean=arguments.value; rc.contentBean=rc.contentBean.save();`, ""},
		{"unseeded cycle", `rc.contentBean=rc.contentBean.save();`, ""},
		{"replacement", `rc.manager=new models.Manager();rc.contentBean=rc.manager.getContent();rc.manager=arguments.value;`, ""},
		{"conflicting branches", `if(value){rc.contentBean=new models.Content();}else{rc.contentBean=new models.Manager();}rc.contentBean=rc.contentBean.save();`, ""},
		{"unknown factory seed", `rc.contentBean=new models.Manager().missing();rc.contentBean=rc.manager.getContent();rc.manager=new models.Manager();`, ""},
		{"different result", `rc.contentBean=new models.Manager();rc.contentBean=rc.contentBean.getContent();`, ""},
		{"container replacement", `rc.manager=new models.Manager();rc.contentBean=rc.manager.getContent();rc=arguments.value;`, ""},
		{"wrong record", `var manager=new models.Manager();rc.contentBean=rc.manager.getContent();`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pr := ParseWithOptions(testURI, "component {\nfunction run(rc,value) {\n"+tc.writes+"\nrc.contentBean.work();\n}\n}", &ParseOptions{ExtractCalls: true, FuncLookup: lookup})
			s := pr.Scopes[0]
			got := ""

			for _, ref := range pr.FuncComponentRefsAt(s.Start, s.End, uint32(s.End)) {
				if ref.Variable == "rc.contentBean" {
					got = ref.Component
				}
			}

			if got != tc.want {
				t.Fatalf("receiver=%q want %q", got, tc.want)
			}
		})
	}
}
