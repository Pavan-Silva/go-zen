package auth

import (
	"net/http"

	"github.com/Pavan-Silva/go-zen"
)

// SkipPaths returns a SkipFunc that bypasses authentication for exact path matches.
func SkipPaths(paths ...string) zen.SkipFunc {
	allowed := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		allowed[p] = struct{}{}
	}
	return func(r *http.Request) bool {
		_, ok := allowed[r.URL.Path]
		return ok
	}
}

// SkipPrefixes returns a SkipFunc that bypasses authentication for matching path prefixes.
func SkipPrefixes(prefixes ...string) zen.SkipFunc {
	normalized := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		// Clean trailing slash up-front once during setup
		for len(p) > 1 && p[len(p)-1] == '/' {
			p = p[:len(p)-1]
		}
		if p != "" {
			normalized = append(normalized, p)
		}
	}

	return func(r *http.Request) bool {
		path := r.URL.Path
		pathLen := len(path)

		for _, prefix := range normalized {
			if prefix == "/" {
				return true
			}

			prefixLen := len(prefix)
			if pathLen < prefixLen {
				continue
			}

			// Slice boundary check: verifies match if exact match OR matched as a distinct route directory
			if path[:prefixLen] == prefix {
				if pathLen == prefixLen || path[prefixLen] == '/' {
					return true
				}
			}
		}
		return false
	}
}

// SkipMethodsAndPaths returns a SkipFunc that bypasses authentication for specific method/path pairs.
func SkipMethodsAndPaths(method string, paths ...string) zen.SkipFunc {
	allowed := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		allowed[method+" "+p] = struct{}{}
	}
	return func(r *http.Request) bool {
		_, ok := allowed[r.Method+" "+r.URL.Path]
		return ok
	}
}
