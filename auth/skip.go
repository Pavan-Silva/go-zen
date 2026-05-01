package auth

import (
	"net/http"
	"strings"

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
	for _, prefix := range prefixes {
		if prefix == "" {
			continue
		}
		if prefix != "/" {
			prefix = strings.TrimSuffix(prefix, "/")
			if prefix == "" {
				prefix = "/"
			}
		}
		normalized = append(normalized, prefix)
	}

	return func(r *http.Request) bool {
		path := r.URL.Path
		for _, prefix := range normalized {
			if hasPathPrefix(path, prefix) {
				return true
			}
		}
		return false
	}
}

func hasPathPrefix(path, prefix string) bool {
	if prefix == "/" {
		return true
	}
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
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
