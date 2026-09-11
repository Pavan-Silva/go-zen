package rbac

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustApplyRoles(t *testing.T, configs ...Role) {
	t.Helper()
	if err := Apply(WithRoles(configs...)); err != nil {
		t.Fatal(err)
	}
}

func TestWithRoles_Inheritance(t *testing.T) {
	mustApplyRoles(t,
		Role{Name: "viewer", Permissions: []string{"posts:read"}},
		Role{Name: "editor", Permissions: []string{"posts:write"}, InheritsFrom: []string{"viewer"}},
		Role{Name: "admin", Permissions: []string{"users:delete"}},
	)

	editor := []string{"editor"}
	if !HasPermission(editor, "posts:write") {
		t.Error("editor should have its own permission")
	}
	if !HasPermission(editor, "posts:read") {
		t.Error("editor should inherit viewer's permission")
	}
	if HasPermission(editor, "users:delete") {
		t.Error("editor should not have admin's permission")
	}
}

func TestWithRoles_TransitiveInheritance(t *testing.T) {
	mustApplyRoles(t,
		Role{Name: "a", Permissions: []string{"x"}},
		Role{Name: "b", InheritsFrom: []string{"a"}},
		Role{Name: "c", InheritsFrom: []string{"b"}},
	)

	if !HasPermission([]string{"c"}, "x") {
		t.Error("grandchild role should inherit grandparent's permission")
	}
}

func TestWithRoles_CyclicInheritance(t *testing.T) {
	mustApplyRoles(t,
		Role{Name: "a", InheritsFrom: []string{"b"}},
		Role{Name: "b", InheritsFrom: []string{"a"}},
	)

	if HasPermission([]string{"a", "b"}, "anything") {
		t.Error("cyclic inheritance must not grant permissions")
	}
}

func TestHasPermission_EdgeCases(t *testing.T) {
	mustApplyRoles(t, Role{Name: "viewer", Permissions: []string{"posts:read"}})

	if HasPermission(nil, "posts:read") {
		t.Error("nil roles must not grant")
	}
	if HasPermission([]string{"viewer"}, "") {
		t.Error("empty permission must not match")
	}
	if HasPermission([]string{"ghost"}, "posts:read") {
		t.Error("unregistered role must grant nothing")
	}
	if HasPermission([]string{"viewer"}, "posts:read") == false {
		t.Error("registered permission should match exactly")
	}
	if HasPermission([]string{"viewer"}, "posts:READ") {
		t.Error("permissions must be case-sensitive")
	}
}

func TestHasRole_Helpers(t *testing.T) {
	roles := []string{"admin", "editor"}

	if !HasRole(roles, "admin") || !HasRole(roles, "editor") {
		t.Error("HasRole should match held roles exactly")
	}
	if HasRole(roles, "ADMIN") {
		t.Error("HasRole must be case-sensitive")
	}
	if HasRole(roles, "") {
		t.Error("empty role must not match")
	}
	if HasRole(nil, "admin") {
		t.Error("nil roles must not match")
	}

	if !HasAnyRole(roles, "guest", "admin") {
		t.Error("HasAnyRole should match one of the candidates")
	}
	if HasAnyRole(roles, "guest", "writer") {
		t.Error("HasAnyRole should not match when none are held")
	}
	if HasAnyRole(roles) {
		t.Error("HasAnyRole with no candidates must be false")
	}

	if !HasAllRoles(roles, "admin", "editor") {
		t.Error("HasAllRoles should match when every role is held")
	}
	if HasAllRoles(roles, "admin", "owner") {
		t.Error("HasAllRoles should fail when a role is missing")
	}
	if HasAllRoles(roles) || HasAllRoles(nil) {
		t.Error("HasAllRoles with an empty required list must not match")
	}
}

func TestHasAnyAllPermissions(t *testing.T) {
	mustApplyRoles(t, Role{Name: "viewer", Permissions: []string{"posts:read"}})
	roles := []string{"viewer"}

	if !HasAnyPermission(roles, "posts:read", "posts:write") {
		t.Error("HasAnyPermission should match a granted permission")
	}
	if HasAnyPermission(roles, "posts:write", "users:delete") {
		t.Error("HasAnyPermission should fail with no granted permissions")
	}
	if HasAnyPermission(roles) {
		t.Error("HasAnyPermission with no permissions must be false")
	}

	if !HasAllPermissions(roles, "posts:read") {
		t.Error("HasAllPermissions should match a granted permission")
	}
	if HasAllPermissions(roles, "posts:read", "posts:write") {
		t.Error("HasAllPermissions should fail when any permission is missing")
	}
	if HasAllPermissions(roles) || HasAllPermissions(nil) {
		t.Error("HasAllPermissions with no permissions must not match")
	}
	if HasPermission(nil, "posts:read") {
		t.Error("nil roles must not grant")
	}
}

func TestApply_CrossBatchInheritance(t *testing.T) {
	mustApplyRoles(t, Role{Name: "viewer", Permissions: []string{"posts:read"}})
	mustApplyRoles(t, Role{Name: "editor", Permissions: []string{"posts:write"}, InheritsFrom: []string{"viewer"}})

	if !HasPermission([]string{"editor"}, "posts:write") {
		t.Error("editor should own its permission")
	}
	if !HasPermission([]string{"editor"}, "posts:read") {
		t.Error("editor should inherit viewer from an earlier batch")
	}
}

func TestApply_WithFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rbac.json")
	if err := os.WriteFile(path, []byte(`{
		"roles": [
			{"name": "viewer", "permissions": ["posts:read"]},
			{"name": "editor", "permissions": ["posts:write"], "inheritsFrom": ["viewer"]}
		]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Apply(WithFile(path)); err != nil {
		t.Fatal(err)
	}

	if !HasPermission([]string{"editor"}, "posts:write") {
		t.Error("file role should own its permission")
	}
	if !HasPermission([]string{"editor"}, "posts:read") {
		t.Error("file role should inherit through the config")
	}
}

func TestApply_MissingFileReturnsError(t *testing.T) {
	err := Apply(WithFile(filepath.Join(t.TempDir(), "does-not-exist.json")))
	if err == nil {
		t.Fatal("expected an error for a missing config file")
	}
	if !strings.Contains(err.Error(), "does-not-exist.json") {
		t.Fatalf("error should name the missing file, got: %v", err)
	}
}

func TestApply_ZeroOptionsUsesDefaultPath(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "configs", "rbac.json"), []byte(`{
		"roles": [{"name": "viewer", "permissions": ["posts:read"]}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	if err := Apply(); err != nil {
		t.Fatal(err)
	}

	if !HasPermission([]string{"viewer"}, "posts:read") {
		t.Fatal("Apply() should register roles from the default config file")
	}
}
