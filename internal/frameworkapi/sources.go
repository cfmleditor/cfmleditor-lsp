package frameworkapi

// Source is where a preset's stubs were generated from: the repository and
// commit, the dot-path prefix its components are named by, and the directory
// in the repository that prefix stands for. `make framework-stubs` clones each
// at its commit and runs cmd/cfstubgen over it; the generated stubs are
// committed, so nothing needs the network to build.
type Source struct {
	Framework string // the preset the stubs serve
	Repo      string
	Commit    string
	Prefix    string // first segment of the dot-paths: coldbox, testbox, qb, ...
	Dir       string // the repository directory Prefix stands for, "" for its root

	// Extra are components a project extends by name, beyond the ones the
	// preset types: a spec's BaseSpec, a ColdBox test's BaseTestCase, a
	// Wheels model's Model.
	Extra []string
}

// idPackages are the packages a framework's own WireBox maps by file name, so
// a bare id a module injects — `inject="FileSystem"` in a CommandBox module —
// is the class of that name in one of them.
var idPackages = map[string][]string{
	"commandbox": {"commandbox.system.services", "commandbox.system.util"},
}

// Sources lists every framework the stubs cover. Moving a commit means
// running `make framework-stubs` and reviewing the diff under stubs/.
var Sources = []Source{
	{"coldbox", "https://github.com/ColdBox/coldbox-platform", "c318d8d9a48a3ab48241d031d6d8b643e6e88393", "coldbox", "", []string{
		"coldbox.system.Bootstrap",
		"coldbox.system.RestHandler",
		"coldbox.system.web.context.RequestContextDecorator",
		"coldbox.system.logging.AbstractAppender",
		"coldbox.system.testing.VirtualApp",
		"coldbox.system.testing.BaseTestCase",
		"coldbox.system.testing.BaseModelTest",
		"coldbox.system.testing.BaseInterceptorTest",
		// What `inject="wirebox:populator"`, `inject="cachebox:template"` and
		// `inject="XMLConverter@coldbox"` hand a property
		// (parser.InjectedFrameworkComponents).
		"coldbox.system.core.dynamic.ObjectPopulator",
		"coldbox.system.cache.providers.CacheBoxColdBoxProvider",
		"coldbox.system.core.conversion.XMLConverter",
	}},
	{"testbox", "https://github.com/Ortus-Solutions/TestBox", "af36cddb7b6882deab7bebb5af7f4b4d89ef66a4", "testbox", "", []string{
		"testbox.system.BaseSpec",
		"testbox.system.compat.framework.TestCase",
	}},
	{"commandbox", "https://github.com/Ortus-Solutions/commandbox", "28a136f9b477abfa50259bb0cab2af0097cc68e8", "commandbox", "src/cfml", []string{
		// Every class CommandBox's WireBox maps by file name
		// (system/config/WireBox.cfc: mapDirectory of each), so a module's
		// `inject="FileSystem"` or `inject="ServerService"` is one of them.
		"commandbox.system.services.*",
		"commandbox.system.util.*",
	}},
	{"cfmigrations", "https://github.com/coldbox-modules/qb", "8eaab747f0402e0a182c1d9775e82cc3615c21e1", "qb", "", nil},
	{"contentbox", "https://github.com/Ortus-Solutions/ContentBox", "312f1823ad4450cbe573f47f106a9b99f1934f2e", "contentbox", "modules/contentbox", nil},
	{"wheels", "https://github.com/cfwheels/cfwheels", "ef0436596635d3a717ff81bddb7af048af442f7b", "wheels", "vendor/wheels", []string{
		"wheels.Model",
		"wheels.Injector",
		"wheels.Test",
		"wheels.WheelsTest",
		"wheels.migrator.Migration",
	}},
	// cborm has no preset: a service written `extends="cborm.models.
	// VirtualEntityService"` names it, and Namespaced answers that path.
	{"cborm", "https://github.com/coldbox-modules/cborm", "3cd94ac1b256bbd02838fd67b453734c91013822", "cborm", "", []string{
		"cborm.models.BaseORMService",
		"cborm.models.VirtualEntityService",
		"cborm.models.ActiveEntity",
		"cborm.models.criterion.CriteriaBuilder",
		"cborm.models.criterion.DetachedCriteriaBuilder",
	}},
	{"fw1", "https://github.com/framework-one/fw1", "d7fb9add9b82be4d7c884ebb2d0f88ffb59ed8c9", "framework", "framework", nil},
}
