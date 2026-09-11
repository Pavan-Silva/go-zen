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

// DefaultConfigPath is the JSON config file Apply loads when no source is
// configured.
const DefaultConfigPath = "configs/rbac.json"

// config is the JSON shape of an RBAC definition file.
//
//	{
//	  "roles": [
//	    {"name": "viewer", "permissions": ["posts:read"]},
//	    {"name": "editor", "permissions": ["posts:write"], "inheritsFrom": ["viewer"]}
//	  ]
//	}
type config struct {
	Roles []Role `json:"roles"`
}

// Role describes a role for Apply. A role owns permissions, and its
// effective permission set is the union of its own Permissions plus every role
// it inherits from (transitively). When using InheritsFrom, only list the
// additional permissions — inherited ones are included automatically.
//
// Permissions are exact resource:action strings ("orders:read"); wildcards
// are not supported.
type Role struct {
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

// roleDefs holds the raw role definitions so inheritance can be resolved
// across separate registration batches. It is swapped alongside rolePerms.
var roleDefs atomic.Pointer[map[string]Role]

// Option configures role registration for Apply. Use WithFile to load roles
// from a JSON config file or WithRoles to provide them inline; both may be
// combined.
type Option func(*options)

type options struct {
	file    string
	hasFile bool
	roles   []Role
}

// WithFile selects a JSON config file to load role definitions from. An empty
// path falls back to DefaultConfigPath.
func WithFile(path string) Option {
	return func(o *options) {
		if path == "" {
			path = DefaultConfigPath
		}
		o.file = path
		o.hasFile = true
	}
}

// WithRoles registers role definitions inline. When combined with WithFile,
// the file roles are registered first, then these definitions.
func WithRoles(configs ...Role) Option {
	return func(o *options) {
		o.roles = configs
	}
}

// Apply registers role definitions for permission checks. Call it once at
// startup, before serving. With no options Apply falls back to loading
// DefaultConfigPath, so Apply() is equivalent to Engine.EnableRBAC(). The
// source is configured with function options:
//
//	Apply()                                // loads configs/rbac.json
//	Apply(WithFile("rbac.json"))           // any JSON config file
//	Apply(WithRoles(Role{...}))            // programmatic definitions
//
// Apply returns an error only when a configured file cannot be read or parsed.
func Apply(opts ...Option) error {
	o := &options{}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}

	if !o.hasFile && len(o.roles) == 0 {
		o.file = DefaultConfigPath
	}

	if err := applyFile(o.file); err != nil {
		return err
	}
	if len(o.roles) > 0 {
		registerRoles(o.roles...)
	}
	return nil
}

// registerRoles merges new role definitions into the registry snapshot.
// Re-registering a role replaces its previous definition while other roles are
// preserved.
// registerRoles merges new role definitions into the registry snapshot.
// Re-registering a role replaces its previous definition while other roles are
// preserved. Inheritance resolves against every registered role, not just the
// roles in this batch, so a role may inherit from one registered earlier.
func registerRoles(configs ...Role) {
	defs := make(map[string]Role, len(configs))
	if prev := roleDefs.Load(); prev != nil {
		maps.Copy(defs, *prev)
	}
	for _, c := range configs {
		defs[c.Name] = c
	}
	roleDefs.Store(&defs)

	roles := make(map[string]map[string]struct{}, len(defs))
	for name := range defs {
		roles[name] = collectPermissions(name, defs, map[string]bool{})
	}

	rolePerms.Store(&registry{roles: roles})
}

// applyFile loads role definitions from path and registers them. An empty
// path (only possible when no file was requested) is a no-op.
func applyFile(path string) error {
	if path == "" {
		return nil
	}

	cfg, err := loadConfig(path)
	if err != nil {
		return err
	}
	registerRoles(cfg.Roles...)
	return nil
}

// loadConfig reads and parses role definitions from a JSON config file.
func loadConfig(path string) (*config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("rbac: read config %s: %w", path, err)
	}

	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("rbac: parse config %s: %w", path, err)
	}

	return &cfg, nil
}

// HasPermission reports whether an actor holding the given roles is granted
// the permission, resolving through the roles registered with Apply.
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
func collectPermissions(name string, defs map[string]Role, visiting map[string]bool) map[string]struct{} {
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
