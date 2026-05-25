package server

import (
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
)

func TestDefinitionTestdata_TagResolverGetUser(t *testing.T) {
	srv := newTestdataServer()
	srv.WorkspaceFolders = append(srv.WorkspaceFolders, filepath.Join(testdataDir(), "beans"))
	openTestdataFile(t, srv, "services/UserService.cfc")
	openTestdataFile(t, srv, "beans/services/BeanUserService.cfc")
	docURI := openTestdataFile(t, srv, "DefinitionTestTag.cfc")

	// Line 24: '<cfset var user = variables.userService.getUser(1)>' — cursor on "getUser"
	// Should resolve to services/UserService.cfc (via resolver), not return multiple matches
	result := definitionAt(t, srv, docURI, 24, 48)
	if result == nil {
		t.Fatal("expected definition result, got nil")
	}

	if _, ok := result.([]protocol.Location); ok {
		t.Error("expected single Location from resolver, got multiple results")
	}

	assertLocationFile(t, result, "services/UserService.cfc")
}
