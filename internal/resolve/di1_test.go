package resolve

import (
	"path/filepath"
	"testing"

	"github.com/cfmleditor/clif/internal/index"
	cfpath "github.com/cfmleditor/clif/internal/path"
	"github.com/cfmleditor/clif/internal/vfs"
)

// TestDI1AutowiresByBeanName: an FW/1 application's DI/1 registers each
// component under its file name and under that name and its folder's
// singular — model/services/user.cfc is `user` and `userService` — and a
// controller's `property userService;` is autowired by that name, as
// getBean( "userService" ) returns it. Without diLocations DI/1 searches
// model and controllers.
func TestDI1AutowiresByBeanName(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Application.cfc":           `component extends="framework.one" {}`,
		"model/services/user.cfc":   `component { function list() {} }`,
		"model/beans/formatter.cfc": `component { function format( v ) {} }`,
		"controllers/main.cfc": `component {
	property userService;
	property formatterBean;
	function default( rc ) {
		userService.list();
		userService.notAMethod();
		formatterBean.format( 1 );
	}
}`,
	})

	idx := index.New()
	beanPaths := cfpath.BeanPathsFor(nil, []string{dir})
	idx.SetBeans(cfpath.BuildBeanMap(beanPaths, vfs.OS{}))

	if got := idx.LookupBean("userService"); got != filepath.Join(dir, "model", "services", "user.cfc") {
		t.Fatalf("userService bean = %q", got)
	}

	expectReasons(t, reasonsWith(t, &Resolver{Index: idx}, dir, "controllers/main.cfc"), map[string]string{
		"userService.list":       "",
		"userService.notAMethod": "method 'notAMethod' not found in user",
		"formatterBean.format":   "",
	})
}
