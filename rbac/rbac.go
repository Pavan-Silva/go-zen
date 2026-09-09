// Package rbac implements the role-based access control registry behind zen's
// authorization middleware. It maps roles — with transitive inheritance — to
// exact resource:action permissions, loads definitions from JSON, and answers
// role/permission checks for an actor's role list. It is a standalone package
// with no dependency on the zen core.
package rbac

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"sync/atomic"
)

// DefaultConfigPath is the JSON config file Engine.EnableRBAC loads when no
// custom path is provided.
const DefaultConfigPath = "configs/rbac.json"

// Config is the JSON shape of an RBAC definition file.
//
//	{
//	  "roles": [
//	    {"name": "viewer", "permissions": ["posts:read"]},
//	    {"name": "editor", "permissions": ["posts:write"], "inheritsFrom": ["viewer"]}
//	  ]
//	}
type Config struct {
	Roles []RoleConfig `json:"roles"`
}

// RoleConfig describes a role for RegisterRoles. A role owns permissions, and
// its effective permission set is the union of its own Permissions plus every
// role it inherits from (transitively). When using InheritsFrom, only list the
// additional permissions — inherited ones are included automatically.
//
// Permissions are exact resource:action strings ("orders:read"); wildcards
// are not supported.
type RoleConfig struct {
	Name         string   `json:"name"`
	Permissions  []string `json:"permissions"`
	InheritsFrom []string `json:"inheritsFrom"`
}

// registry is an immutable snapshot of the role→permission map. It is swapped
// wholesale on registration, so permission checks read it without a lock.
type registry struct {
	roles map[string]map[string]struct{}
}

var rolePerms atomic.Pointer[registry]

// RegisterRoles records role definitions. Re-registering a role replaces its
// previous definition while other roles are preserved. Register roles once at
// startup, before serving.
func RegisterRoles(configs ...RoleConfig) {
	defs := make(map[string]RoleConfig, len(configs))
	for _, c := range configs {
		defs[c.Name] = c
	}

	prev := rolePerms.Load()
	roles := make(map[string]map[string]struct{}, len(configs))
	if prev != nil {
		maps.Copy(roles, prev.roles)
	}

	for _, c := range configs {
		roles[c.Name] = collectPermissions(c.Name, defs, map[string]bool{})
	}

	rolePerms.Store(&registry{roles: roles})
}

// LoadConfig reads and parses role definitions from a JSON config file.
// Engine.EnableRBAC uses the DefaultConfigPath when no custom path is given.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("rbac: read config %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("rbac: parse config %s: %w", path, err)
	}

	return &cfg, nil
}

// HasPermission reports whether an actor holding the given roles is granted
// the permission, resolving through the roles registered with RegisterRoles.
// Permissions are exact, case-sensitive matches — wildcards are not supported.
// Roles that are not registered grant nothing.
func HasPermission(roles []string, permission string) bool {
	if len(roles) == 0 || permission == "" {
		return false
	}

	reg := rolePerms.Load()
	if reg == nil {
		return false
	}

	for _, role := range roles {
		if perms, ok := reg.roles[role]; ok {
			if _, ok := perms[permission]; ok {
				return true
			}
		}
	}
	return false
}

// HasRole reports whether the roles slice holds exactly the named role.
// Role names match exactly (case-sensitive); no prefix normalization is
// applied.
func HasRole(roles []string, role string) bool {
	return role != "" && slices.Contains(roles, role)
}

// HasAnyRole reports whether the roles slice holds at least one of the
// supplied candidates.
func HasAnyRole(roles []string, candidates ...string) bool {
	return slices.ContainsFunc(candidates, func(c string) bool { return HasRole(roles, c) })
}

// HasAllRoles reports whether the roles slice holds every supplied role.
// An empty required list cannot be satisfied.
func HasAllRoles(roles []string, required ...string) bool {
	if len(required) == 0 || len(roles) == 0 {
		return false
	}

	for _, role := range required {
		if !HasRole(roles, role) {
			return false
		}
	}

	return true
}

// HasAnyPermission reports whether an actor holding the given roles is granted
// at least one of the supplied permissions.
func HasAnyPermission(roles []string, permissions ...string) bool {
	return slices.ContainsFunc(permissions, func(p string) bool { return HasPermission(roles, p) })
}

// HasAllPermissions reports whether an actor holding the given roles is
// granted every supplied permission. An empty list cannot be satisfied.
func HasAllPermissions(roles []string, permissions ...string) bool {
	if len(permissions) == 0 || len(roles) == 0 {
		return false
	}

	for _, permission := range permissions {
		if !HasPermission(roles, permission) {
			return false
		}
	}

	return true
}

// collectPermissions returns the permission set reachable from a role name,
// guarding against inheritance cycles.
func collectPermissions(name string, defs map[string]RoleConfig, visiting map[string]bool) map[string]struct{} {
	perms := make(map[string]struct{})
	if name == "" || visiting[name] {
		return perms
	}

	role, ok := defs[name]
	if !ok {
		return perms
	}

	visiting[name] = true
	for _, p := range role.Permissions {
		perms[p] = struct{}{}
	}

	for _, parent := range role.InheritsFrom {
		for p := range collectPermissions(parent, defs, visiting) {
			perms[p] = struct{}{}
		}
	}

	visiting[name] = false
	return perms
}
