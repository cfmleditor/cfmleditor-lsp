package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
	"go.lsp.dev/uri"
)

func TestPropertyDefinition_BeanLookupViaInject(t *testing.T) {
	dir := beansTestdataDir()
	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.BeanPaths = map[string]string{
		"dao":      filepath.Join(dir, "dao"),
		"services": filepath.Join(dir, "services"),
	}

	// Build bean map
	beans := buildBeanMap(srv.BeanPaths, vfs.OS{})
	srv.index.SetBeans(beans)

	// Parse PropertyTest.cfc which has inject="UserDAO@dao"
	abs := filepath.Join(dir, "PropertyTest.cfc")
	docURI := uri.URI("file://" + abs)
	content := readTestFile(t, abs)
	srv.setDocument(docURI, content)

	pr := parser.ParseWithOptions(docURI, content, parser.ParseOptions{
		BeanLookup: srv.index.LookupBean,
	})
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)

	// Index the target CFC
	daoAbs := filepath.Join(dir, "dao", "UserDAO.cfc")
	daoURI := uri.URI("file://" + daoAbs)
	daoContent := readTestFile(t, daoAbs)
	srv.setDocument(daoURI, daoContent)
	srv.index.IndexFile(daoURI, daoContent)

	// userDAO should have a component ref (absolute path from bean map)
	ref := srv.index.LookupComponentRefInFile("userDAO", docURI, 100)
	if ref == nil {
		t.Fatal("expected component ref for userDAO")
	}

	if !strings.HasSuffix(ref.Component, "dao/UserDAO.cfc") {
		t.Errorf("expected path ending in dao/UserDAO.cfc, got %q", ref.Component)
	}
}

func TestPropertyDefinition_TypeBasedRef(t *testing.T) {
	dir := beansTestdataDir()
	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	abs := filepath.Join(dir, "PropertyTest.cfc")
	docURI := uri.URI("file://" + abs)
	content := readTestFile(t, abs)
	srv.setDocument(docURI, content)

	pr := parser.Parse(docURI, content)
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)

	// logger has type="services.BeanUserService" which is a CFC path
	ref := srv.index.LookupComponentRefInFile("logger", docURI, 100)
	if ref == nil {
		t.Fatal("expected component ref for logger")
	}

	if ref.Component != "services.BeanUserService" {
		t.Errorf("expected services.BeanUserService, got %q", ref.Component)
	}
}

func TestPropertyDefinition_PrimitiveTypeNoRef(t *testing.T) {
	dir := beansTestdataDir()
	srv := newTestServer()

	abs := filepath.Join(dir, "PropertyTest.cfc")
	docURI := uri.URI("file://" + abs)
	content := readTestFile(t, abs)
	srv.setDocument(docURI, content)

	pr := parser.Parse(docURI, content)
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)

	// config has type="string" — should NOT create a component ref
	ref := srv.index.LookupComponentRefInFile("config", docURI, 100)
	if ref != nil {
		t.Errorf("string-typed property should not have component ref, got %v", ref)
	}
}

func TestPropertyDefinition_TagCFC(t *testing.T) {
	dir := beansTestdataDir()
	srv := newTestServer()
	srv.BeanPaths = map[string]string{
		"dao": filepath.Join(dir, "dao"),
	}
	beans := buildBeanMap(srv.BeanPaths, vfs.OS{})
	srv.index.SetBeans(beans)

	abs := filepath.Join(dir, "PropertyTestTag.cfc")
	docURI := uri.URI("file://" + abs)
	content := readTestFile(t, abs)
	srv.setDocument(docURI, content)

	pr := parser.ParseWithOptions(docURI, content, parser.ParseOptions{
		BeanLookup: srv.index.LookupBean,
	})
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)

	// userDAO has inject="UserDAO@dao"
	ref := srv.index.LookupComponentRefInFile("userDAO", docURI, 100)
	if ref == nil {
		t.Fatal("expected component ref for userDAO in tag CFC")
	}

	if !strings.HasSuffix(ref.Component, "dao/UserDAO.cfc") {
		t.Errorf("expected path ending in dao/UserDAO.cfc, got %q", ref.Component)
	}

	// helper has type="services.BeanUserService"
	ref = srv.index.LookupComponentRefInFile("helper", docURI, 100)
	if ref == nil {
		t.Fatal("expected component ref for helper")
	}

	if ref.Component != "services.BeanUserService" {
		t.Errorf("expected services.BeanUserService, got %q", ref.Component)
	}
}

func TestPropertyDefinition_AccessorsGenerated(t *testing.T) {
	dir := beansTestdataDir()
	srv := newTestServer()

	abs := filepath.Join(dir, "PropertyTest.cfc")
	docURI := uri.URI("file://" + abs)
	content := readTestFile(t, abs)
	srv.setDocument(docURI, content)

	pr := parser.Parse(docURI, content)
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)

	// Check that accessor functions are indexed
	expected := []string{"getUserDAO", "setUserDAO", "getOrderDAO", "setOrderDAO",
		"getLogger", "setLogger", "getConfig", "setConfig",
		"getBeanUserService", "setBeanUserService"}
	for _, name := range expected {
		defs := srv.index.Lookup(name)
		found := false

		for _, d := range defs {
			if d.URI == docURI {
				found = true

				break
			}
		}

		if !found {
			t.Errorf("expected indexed function %s for PropertyTest.cfc", name)
		}
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}

	return string(data)
}
