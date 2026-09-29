package resolve

import (
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/frameworkapi"
)

// TestAWireBoxIDIsWhatItsModuleRegisters: ColdBox registers a module's models
// under its modelNamespace — the module's name unless ModuleConfig.cfc says
// otherwise — so `UserService@users` is the one in the users module, not the
// nearest file of that name. A binder's `map( id ).to( path )` is exact,
// `#moduleMapping#` being the module. A module's cfmapping is a mapping to it.
// And an id of a module the workspace does not have is the module not being
// installed, which is not a finding: cbstorages, cbmailservices and the rest
// were reported against the corpus.
func TestAWireBoxIDIsWhatItsModuleRegisters(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"modules/users/ModuleConfig.cfc": `component {
	this.cfmapping = "usersModule";
	function onLoad() {
		binder.map( "Mailer@users" )
			.to( "#moduleMapping#.models.mail.SmtpMailer" );
	}
}`,
		"modules/users/models/UserService.cfc":     `component { function list() {} }`,
		"modules/users/models/mail/SmtpMailer.cfc": `component { function send() {} }`,
		"modules/users/lib/Util.cfc":               `component { function help() {} }`,
		"modules/admin/ModuleConfig.cfc":           `component { this.modelNamespace = "cbadmin"; }`,
		"modules/admin/models/UserService.cfc":     `component { function adminOnly() {} }`,
		"config/WireBox.cfc":                       `component { function configure() { map( "Clock" ).to( "lib.SystemClock" ); } }`,
		"lib/SystemClock.cfc":                      `component { function now() {} }`,
		"handlers/Main.cfc": `component {
	property name="users" inject="UserService@users";
	property name="admin" inject="id:UserService@cbadmin";
	property name="mailer" inject="Mailer@users";
	property name="storage" inject="RequestStorage@cbstorages";
	property name="clock" inject="Clock";
	function index() {
		users.list();
		users.adminOnly();
		admin.adminOnly();
		mailer.send();
		storage.getSessionVar( "a" );
		clock.now();
		var u = new usersModule.lib.Util();
		u.help();
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "handlers/Main.cfc"), map[string]string{
		"users.list":            "",
		"users.adminOnly":       "method 'adminOnly' not found in UserService@users",
		"admin.adminOnly":       "",
		"mailer.send":           "",
		"storage.getSessionVar": "",
		"clock.now":             "",
		"u.help":                "",
	})
}

// TestACommandBoxIDIsOneOfItsServicesOrUtils: CommandBox's WireBox maps its
// services and util packages by file name, so a CommandBox module's
// `inject="FileSystem"` is commandbox.system.util.FileSystem, as
// cfwheels' CLI injects it, while a project's own file of the name wins.
func TestACommandBoxIDIsOneOfItsServicesOrUtils(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"commands/Build.cfc": `component {
	property name="fileSystemUtil" inject="FileSystem";
	property name="serverService" inject="ServerService";
	property name="mine" inject="MyOwn";
	function run() {
		fileSystemUtil.resolvePath( "x" );
		fileSystemUtil.notAMethod();
		serverService.resolveServerDetails( {} );
		mine.go();
	}
}`,
		"models/MyOwn.cfc": `component { function go() {} }`,
	})

	r := &Resolver{Stubs: frameworkapi.For([]string{"commandbox"})}
	expectReasons(t, reasonsWith(t, r, dir, "commands/Build.cfc"), map[string]string{
		"fileSystemUtil.resolvePath":         "",
		"fileSystemUtil.notAMethod":          "method 'notAMethod' not found in FileSystem",
		"serverService.resolveServerDetails": "",
		"mine.go":                            "",
	})
}
