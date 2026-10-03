package resolve

import "testing"

func TestGuardedLazyFieldsReturnTheirInitializedType(t *testing.T) {
	tests := []struct {
		name, guard, initial string
	}{
		{"presence", `!structKeyExists(variables,"service")`, ""},
		{"primitive sentinel", `isSimpleValue(variables.service)`, `variables.service="";`},
		{"null", `isNull(variables.service)`, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{
				"Service.cfc": `component { function ready(){} }`,
				"Owner.cfc":   `component {` + tc.initial + ` function getService(){if(` + tc.guard + `){variables.service=new Service();}return variables.service;} }`,
				"Page.cfc":    `component {function run(){new Owner().getService().ready();}}`,
			})

			expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Page.cfc"), map[string]string{
				"getService.ready": "",
			})
		})
	}
}

func TestGuardedLazyFieldsFailClosed(t *testing.T) {
	tests := []struct {
		name, extra, initial string
	}{
		{"conflicting component", `function replace(){variables.service=new Other();}`, ``},
		{"primitive replacement", `function replace(){variables.service="bad";}`, ``},
		{"conflicting startup component", ``, `variables.service=new Other();`},
		{"primitive startup for null guard", ``, `variables.service="bad";`},
		{"whole scope replacement", `function replace(){variables={};}`, ``},
		{"overridden guard", `function isNull(value){return false;}`, ``},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{
				"Service.cfc": `component { function ready(){} }`,
				"Other.cfc":   `component { function ready(){} }`,
				"Owner.cfc":   `component {` + tc.initial + ` function getService(){if(isNull(variables.service)){variables.service=new Service();}return variables.service;}` + tc.extra + ` }`,
				"Page.cfc":    `component {function run(){new Owner().getService().ready();}}`,
			})

			got := reasonsWith(t, &Resolver{}, dir, "Page.cfc")
			if reason, exists := got["getService.ready"]; !exists || reason == "" {
				t.Fatalf("guarded field resolved without an all-writes-agree contract: %v", got)
			}
		})
	}
}
