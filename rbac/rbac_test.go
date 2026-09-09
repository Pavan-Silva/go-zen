package rbac

import "testing"

func TestRegisterRoles_Inheritance(t *testing.T) {
	RegisterRoles(
		RoleConfig{Name: "viewer", Permissions: []string{"posts:read"}},
		RoleConfig{Name: "editor", Permissions: []string{"posts:write"}, InheritsFrom: []string{"viewer"}},
		RoleConfig{Name: "admin", Permissions: []string{"users:delete"}},
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

func TestRegisterRoles_TransitiveInheritance(t *testing.T) {
	RegisterRoles(
		RoleConfig{Name: "a", Permissions: []string{"x"}},
		RoleConfig{Name: "b", InheritsFrom: []string{"a"}},
		RoleConfig{Name: "c", InheritsFrom: []string{"b"}},
	)

	if !HasPermission([]string{"c"}, "x") {
		t.Error("grandchild role should inherit grandparent's permission")
	}
}

func TestRegisterRoles_CyclicInheritance(t *testing.T) {
	RegisterRoles(
		RoleConfig{Name: "a", InheritsFrom: []string{"b"}},
		RoleConfig{Name: "b", InheritsFrom: []string{"a"}},
	)

	if HasPermission([]string{"a", "b"}, "anything") {
		t.Error("cyclic inheritance must not grant permissions")
	}
}

func TestHasPermission_EdgeCases(t *testing.T) {
	RegisterRoles(RoleConfig{Name: "viewer", Permissions: []string{"posts:read"}})

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
	RegisterRoles(RoleConfig{Name: "viewer", Permissions: []string{"posts:read"}})
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
