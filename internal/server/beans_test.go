package server

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

func beansTestdataDir() string {
	_, file, _, _ := runtime.Caller(0)

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "beans")
}

func TestBuildBeanMap_Namespaces(t *testing.T) {
	dir := beansTestdataDir()
	beanPaths := map[string]string{
		"dao":      filepath.Join(dir, "dao"),
		"services": filepath.Join(dir, "services"),
	}
	beans := buildBeanMap(beanPaths, vfs.OS{})

	// Namespace-qualified entries should be absolute paths
	tests := []struct {
		key          string
		expectedFile string
	}{
		{"userdao@dao", "dao/UserDAO.cfc"},
		{"orderdao@dao", "dao/OrderDAO.cfc"},
		{"beanuserservice@services", "services/BeanUserService.cfc"},
	}
	for _, tt := range tests {
		got := beans[tt.key]
		if !strings.HasSuffix(got, tt.expectedFile) {
			t.Errorf("beans[%q] = %q, want suffix %q", tt.key, got, tt.expectedFile)
		}
	}

	// Bare names should exist since they're unique across namespaces
	if !strings.HasSuffix(beans["userdao"], "dao/UserDAO.cfc") {
		t.Errorf("beans[userdao] = %q, want suffix dao/UserDAO.cfc", beans["userdao"])
	}

	if !strings.HasSuffix(beans["orderdao"], "dao/OrderDAO.cfc") {
		t.Errorf("beans[orderdao] = %q, want suffix dao/OrderDAO.cfc", beans["orderdao"])
	}
}

func TestBuildBeanMap_DuplicateBareNames(t *testing.T) {
	dir := beansTestdataDir()
	// Both root and "dao" namespace — root walks recursively finding dao/UserDAO.cfc too
	beanPaths := map[string]string{
		"":    dir,
		"dao": filepath.Join(dir, "dao"),
	}
	beans := buildBeanMap(beanPaths, vfs.OS{})

	// Namespace-qualified should always work
	if !strings.HasSuffix(beans["userdao@dao"], "dao/UserDAO.cfc") {
		t.Errorf("beans[userdao@dao] = %q, want suffix dao/UserDAO.cfc", beans["userdao@dao"])
	}
}

func TestBuildBeanMap_EmptyPaths(t *testing.T) {
	beans := buildBeanMap(nil, vfs.OS{})
	if len(beans) != 0 {
		t.Errorf("expected empty map, got %d entries", len(beans))
	}

	beans = buildBeanMap(map[string]string{}, vfs.OS{})
	if len(beans) != 0 {
		t.Errorf("expected empty map, got %d entries", len(beans))
	}
}

func TestBuildBeanMap_SingleNamespace(t *testing.T) {
	dir := beansTestdataDir()
	beanPaths := map[string]string{
		"": filepath.Join(dir, "dao"),
	}
	beans := buildBeanMap(beanPaths, vfs.OS{})

	// With empty namespace, no @-qualified entries
	if _, ok := beans["userdao@"]; ok {
		t.Error("empty namespace should not produce @-qualified entries")
	}
	// Bare names should be absolute paths
	if !strings.HasSuffix(beans["userdao"], "dao/UserDAO.cfc") {
		t.Errorf("beans[userdao] = %q, want suffix dao/UserDAO.cfc", beans["userdao"])
	}
}
