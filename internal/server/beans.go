package server

import (
	cfpath "github.com/cfmleditor/clif/internal/path"
	"github.com/cfmleditor/clif/internal/vfs"
)

// buildBeanMap is cfpath.BuildBeanMap, which the unresolved report and the
// code map build their bean maps with too.
func buildBeanMap(beanPaths map[string]string, fsys vfs.FS) map[string]string {
	return cfpath.BuildBeanMap(beanPaths, fsys)
}
