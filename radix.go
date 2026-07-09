package zen

import (
	"net/url"
	"strings"
)

// ── param types ──────────────────────────────────────────────────────────────

// paramPair stores a single URL parameter key-value pair extracted from
// either a named path segment (:param) or a catch-all (*param).
type paramPair struct {
	Key   string
	Value string
}

// params is an ordered slice of paramPair entries populated during route
// matching. The slice is reused from a pool to eliminate allocations.
type params []paramPair

// get searches the parameter slice for the first entry matching the given
// name. Returns the value and a boolean indicating whether it was found.
func (ps params) get(name string) (string, bool) {
	for _, p := range ps {
		if p.Key == name {
			return p.Value, true
		}
	}
	return "", false
}

// ── method trees ─────────────────────────────────────────────────────────────

// methodTree associates an HTTP method string with the root of its radix tree.
type methodTree struct {
	method string
	root   *node
}

// methodTrees is a slice of methodTree entries, one per registered HTTP method.
// Iterating linearly over a small set (at most 9) is faster than a map lookup.
type methodTrees []methodTree

// get returns the root node for the given HTTP method, or nil if no routes
// have been registered under that method.
func (trees methodTrees) get(method string) *node {
	for _, t := range trees {
		if t.method == method {
			return t.root
		}
	}
	return nil
}

// ── radix tree node ──────────────────────────────────────────────────────────

// nodeType classifies a radix tree node into one of four categories.
type nodeType uint8

const (
	static   nodeType = iota // Regular path segment text
	root                     // Tree root node (empty path before first split)
	param                    // Named parameter segment (:param)
	catchAll                 // Catch-all segment (*param), must be terminal
)

// node represents a single vertex in the radix tree. Each node stores a path
// fragment, optional handlers, and child nodes indexed by their first byte.
// Priority ordering ensures frequently accessed children are checked first.
type node struct {
	// path is the compressed path fragment this node represents. For param
	// nodes this includes the colon prefix (e.g. ":id"). For catch-all nodes
	// it includes the asterisk prefix (e.g. "*rest").
	path string

	// indices is a compact lookup string where each byte is the first
	// character of a static child's path, aligned with the children slice
	// by position. Wildcard children are not indexed here; they are always
	// the last entry in children and tracked via wildChild.
	indices string

	// children holds the child nodes of this node. Static children are
	// ordered by priority (descending). The wildcard child, if present,
	// is always the last entry.
	children []*node

	// handlers is the chain of middleware plus final handler registered for
	// the full path ending at this node. Nil if this node is purely a
	// structural split with no route registered at this point.
	handlers []HandlerFunc

	// priority tracks how often this node is traversed during route
	// insertion, used to reorder children so hot paths are found earlier.
	priority uint32

	// nType classifies this node as static, root, param, or catchAll.
	nType nodeType

	// wildChild is true when the last entry in children is a wildcard node
	// (param or catchAll). This avoids scanning indices for wildcards
	// since they are always positioned at the end.
	wildChild bool

	// fullPath is the complete registered route path that terminates at
	// this node. Only set for nodes with handlers; empty otherwise.
	fullPath string
}

// incrementChildPrio bumps the priority of the child at position pos and
// bubble-sorts it upward until the children remain in descending priority
// order. The indices string is kept in sync with the reordered children.
// Returns the new position of the promoted child.
func (n *node) incrementChildPrio(pos int) int {
	cs := n.children
	cs[pos].priority++
	prio := cs[pos].priority

	newPos := pos
	for newPos > 0 && cs[newPos-1].priority < prio {
		cs[newPos-1], cs[newPos] = cs[newPos], cs[newPos-1]
		newPos--
	}

	if newPos != pos {
		n.indices = n.indices[:newPos] +
			n.indices[pos:pos+1] +
			n.indices[newPos:pos] +
			n.indices[pos+1:]
	}

	return newPos
}

// addChild inserts a child node into the children slice. If the node already
// has a wildcard child (always stored last), the new child is inserted before
// it to preserve the invariant that wildcards remain at the end.
func (n *node) addChild(child *node) {
	if n.wildChild && len(n.children) > 0 {
		wildcardChild := n.children[len(n.children)-1]
		n.children = append(n.children[:len(n.children)-1], child, wildcardChild)
	} else {
		n.children = append(n.children, child)
	}
}

// longestCommonPrefix returns the number of bytes shared from the start of
// two strings. Used during route insertion to determine where a node split
// must occur.
func longestCommonPrefix(a, b string) int {
	i := 0
	max_ := min(len(a), len(b))
	for i < max_ && a[i] == b[i] {
		i++
	}
	return i
}

// ── addRoute ─────────────────────────────────────────────────────────────────

// addRoute inserts a path into the radix tree and associates it with the
// given handler chain. The path may include :param and *param wildcard
// segments. Duplicate registrations cause a panic.
func (n *node) addRoute(path string, handlers []HandlerFunc) {
	fullPath := path
	n.priority++

	// Fast path: empty root node — insert directly without traversal.
	if len(n.path) == 0 && len(n.children) == 0 {
		n.insertChild(path, fullPath, handlers)
		n.nType = root
		return
	}

	parentFullPathIndex := 0

walk:
	for {
		i := longestCommonPrefix(path, n.path)

		// Split the current node if the common prefix does not consume
		// all of n.path. The existing children and handlers move to a
		// new child; the current node becomes the shared prefix.
		if i < len(n.path) {
			child := node{
				path:      n.path[i:],
				wildChild: n.wildChild,
				nType:     static,
				indices:   n.indices,
				children:  n.children,
				handlers:  n.handlers,
				priority:  n.priority - 1,
				fullPath:  n.fullPath,
			}

			n.children = []*node{&child}
			n.indices = string(n.path[i])
			n.path = path[:i]
			n.handlers = nil
			n.wildChild = false
			n.fullPath = fullPath[:parentFullPathIndex+i]
		}

		// Path is fully consumed by this node — attach handlers or descend.
		if i < len(path) {
			path = path[i:]
			c := path[0]

			// Skip over param-matching child nodes when the next byte
			// is a slash. This handles the case where a param node has
			// a single static child for the trailing slash segment.
			if n.nType == param && c == '/' && len(n.children) == 1 {
				parentFullPathIndex += len(n.path)
				n = n.children[0]
				n.priority++
				continue walk
			}

			// Check for an existing static child that matches the next byte.
			for i, max_ := 0, len(n.indices); i < max_; i++ {
				if c == n.indices[i] {
					parentFullPathIndex += len(n.path)
					i = n.incrementChildPrio(i)
					n = n.children[i]
					continue walk
				}
			}

			// No matching static child found. Create a new child or check
			// for wildcard conflicts.
			if c != ':' && c != '*' && n.nType != catchAll {
				// Regular static segment — create a new child.
				n.indices += string([]byte{c})
				child := &node{fullPath: fullPath}
				n.addChild(child)
				n.incrementChildPrio(len(n.indices) - 1)
				n = child
			} else if n.wildChild {
				// A wildcard child already exists — check if the new
				// segment conflicts with the existing wildcard pattern.
				n = n.children[len(n.children)-1]
				n.priority++

				// If the existing wildcard path is a valid prefix of
				// the remaining path segment, continue walking.
				if len(path) >= len(n.path) && n.path == path[:len(n.path)] &&
					n.nType != catchAll &&
					(len(n.path) >= len(path) || path[len(n.path)] == '/') {
					continue walk
				}

				// Conflicting wildcard — panic with a descriptive error.
				pathSeg := path
				if n.nType != catchAll {
					pathSeg, _, _ = strings.Cut(pathSeg, "/")
				}
				prefix := fullPath[:strings.Index(fullPath, pathSeg)] + n.path
				panic("'" + pathSeg + "' in new path '" + fullPath +
					"' conflicts with existing wildcard '" + n.path +
					"' in existing prefix '" + prefix + "'")
			}

			// Insert the remaining path as a child subtree.
			n.insertChild(path, fullPath, handlers)
			return
		}

		// Path exactly matches this node — register handlers or panic
		// if a handler is already registered for this path.
		if n.handlers != nil {
			panic("handlers are already registered for path '" + fullPath + "'")
		}
		n.handlers = handlers
		n.fullPath = fullPath
		return
	}
}

// findWildcard scans a path for the first colon or asterisk wildcard marker.
// Returns the wildcard segment (including the marker), its starting index,
// and whether it is well-formed. A wildcard is invalid if a second marker
// appears within the same segment (e.g. ":a:b").
func findWildcard(path string) (wildcard string, i int, valid bool) {
	for start, c := range []byte(path) {
		if c != ':' && c != '*' {
			continue
		}

		valid = true
		for end, c := range []byte(path[start+1:]) {
			switch c {
			case '/':
				return path[start : start+1+end], start, valid
			case ':', '*':
				valid = false
			}
		}
		return path[start:], start, valid
	}
	return "", -1, false
}

// insertChild creates the subtree necessary to register a path containing
// wildcard segments. It parses :param and *param markers, creates the
// corresponding node structure, and attaches the final handler chain at
// the leaf. Panics on invalid wildcard patterns or conflicts.
func (n *node) insertChild(path string, fullPath string, handlers []HandlerFunc) {
	for {
		wildcard, i, valid := findWildcard(path)
		if i < 0 {
			break
		}

		if !valid {
			panic("only one wildcard per path segment is allowed, has: '" +
				wildcard + "' in path '" + fullPath + "'")
		}

		if len(wildcard) < 2 {
			panic("wildcards must be named with a non-empty name in path '" + fullPath + "'")
		}

		if wildcard[0] == ':' {
			// Named parameter — split at the colon and create a param child.
			if i > 0 {
				n.path = path[:i]
				path = path[i:]
			}

			child := &node{
				nType:    param,
				path:     wildcard,
				fullPath: fullPath,
			}
			n.addChild(child)
			n.wildChild = true
			n = child
			n.priority++

			// If the parameter does not consume the entire remaining path,
			// create a static child for the suffix and continue.
			if len(wildcard) < len(path) {
				path = path[len(wildcard):]
				child := &node{
					priority: 1,
					fullPath: fullPath,
				}
				n.addChild(child)
				n = child
				continue
			}

			n.handlers = handlers
			return
		}

		// Catch-all parameter — must be at the end of the path.
		if i+len(wildcard) != len(path) {
			panic("catch-all routes are only allowed at the end of the path in path '" + fullPath + "'")
		}

		if len(n.path) > 0 && n.path[len(n.path)-1] == '/' {
			pathSeg := ""
			if len(n.children) != 0 {
				pathSeg, _, _ = strings.Cut(n.children[0].path, "/")
			}
			panic("catch-all wildcard '" + path +
				"' in new path '" + fullPath +
				"' conflicts with existing path segment '" + pathSeg +
				"' in existing prefix '" + n.path + pathSeg + "'")
		}

		i--
		if i < 0 || path[i] != '/' {
			panic("no / before catch-all in path '" + fullPath + "'")
		}

		// Split at the slash before the catch-all and insert a two-level
		// catch-all node structure: an intermediate parent with indices="/"
		// and a leaf holding the catch-all path and handlers.
		n.path = path[:i]

		child := &node{
			wildChild: true,
			nType:     catchAll,
			fullPath:  fullPath,
		}
		n.addChild(child)
		n.indices = "/"
		n = child
		n.priority++

		child = &node{
			path:     path[i:],
			nType:    catchAll,
			handlers: handlers,
			priority: 1,
			fullPath: fullPath,
		}
		n.children = []*node{child}
		return
	}

	// No wildcards remaining — attach directly to the current node.
	n.path = path
	n.handlers = handlers
	n.fullPath = fullPath
}

// ── getValue ─────────────────────────────────────────────────────────────────

// nodeValue is the result of a route lookup. It carries the matched handler
// chain, any extracted parameters, a trailing-slash-redirect (TSR) hint, and
// the full registered path for the matched route.
type nodeValue struct {
	handlers []HandlerFunc
	params   *params
	tsr      bool
	fullPath string
}

// skippedNode records a point in the tree where a wildcard branch was
// available but a static child was chosen instead. If the static path fails
// to match, the router backtracks to these saved positions and retries via
// the wildcard path. paramsCount captures the parameter count at that point
// so backtracking can correctly truncate collected parameters.
type skippedNode struct {
	path        string
	node        *node
	paramsCount int16
}

// getValue traverses the radix tree to match a request path against
// registered routes. Returns matched handlers (or nil), extracted URL
// parameters appended to the provided ps slice, and a TSR flag indicating
// whether a trailing-slash redirect would resolve to a valid route.
func (n *node) getValue(path string, ps *params, skippedNodes *[]skippedNode) (value nodeValue) {
	var globalParamsCount int16

walk:
	for {
		prefix := n.path
		if len(path) > len(prefix) {
			// Path is longer than this node's prefix — check for a match.
			if path[:len(prefix)] == prefix {
				path = path[len(prefix):]

				idxc := path[0]
				for i, c := range []byte(n.indices) {
					if c == idxc {
						// Save the wildcard alternative as a backtrack point
						// so we can retry via the wildcard if the static
						// path ultimately fails.
						if n.wildChild {
							index := len(*skippedNodes)
							*skippedNodes = (*skippedNodes)[:index+1]
							(*skippedNodes)[index] = skippedNode{
								path: prefix + path,
								node: &node{
									path:      n.path,
									wildChild: n.wildChild,
									nType:     n.nType,
									priority:  n.priority,
									children:  n.children,
									handlers:  n.handlers,
									fullPath:  n.fullPath,
								},
								paramsCount: globalParamsCount,
							}
						}

						n = n.children[i]
						continue walk
					}
				}

				// No static child matches. If there is no wildcard child
				// either, try backtracking via skipped nodes.
				if !n.wildChild {
					if path != "/" {
						for length := len(*skippedNodes); length > 0; length-- {
							skippedNode := (*skippedNodes)[length-1]
							*skippedNodes = (*skippedNodes)[:length-1]
							if strings.HasSuffix(skippedNode.path, path) {
								path = skippedNode.path
								n = skippedNode.node
								if value.params != nil {
									*value.params = (*value.params)[:skippedNode.paramsCount]
								}
								globalParamsCount = skippedNode.paramsCount
								continue walk
							}
						}
					}

					// TSR check: path has a trailing slash and this node has handlers.
					value.tsr = path == "/" && n.handlers != nil
					return value
				}

				// Descend into the wildcard child and extract the parameter value.
				n = n.children[len(n.children)-1]
				globalParamsCount++

				switch n.nType {
				case param:
					// Scan until the next slash boundary.
					end := 0
					for end < len(path) && path[end] != '/' {
						end++
					}

					// Grow the params slice if capacity is exhausted.
					if cap(*ps) < int(globalParamsCount) {
						newParams := make(params, len(*ps), globalParamsCount)
						copy(newParams, *ps)
						*ps = newParams
					}

					if value.params == nil {
						value.params = ps
					}
					i := len(*value.params)
					*value.params = (*value.params)[:i+1]
					val := path[:end]
					if v, err := url.QueryUnescape(val); err == nil {
						val = v
					}
					(*value.params)[i] = paramPair{
						Key:   n.path[1:],
						Value: val,
					}

					if end < len(path) {
						// More path remains — descend into the static child
						// that follows the parameter segment.
						if len(n.children) > 0 {
							path = path[end:]
							n = n.children[0]
							continue walk
						}

						// No child but path remains — TSR if only a trailing
						// slash was left unmatched.
						value.tsr = len(path) == end+1
						return value
					}

					// Path fully consumed. Return handlers if registered,
					// otherwise check for TSR via the trailing-slash child.
					if value.handlers = n.handlers; value.handlers != nil {
						value.fullPath = n.fullPath
						return value
					}
					if len(n.children) == 1 {
						n = n.children[0]
						value.tsr = (n.path == "/" && n.handlers != nil) || (n.path == "" && n.indices == "/")
					}
					return value

				case catchAll:
					// Capture the entire remaining path (minus leading slash).
					if cap(*ps) < int(globalParamsCount) {
						newParams := make(params, len(*ps), globalParamsCount)
						copy(newParams, *ps)
						*ps = newParams
					}

					if value.params == nil {
						value.params = ps
					}
					i := len(*value.params)
					*value.params = (*value.params)[:i+1]
					val := path
					if len(val) > 0 && val[0] == '/' {
						val = val[1:]
					}
					if v, err := url.QueryUnescape(val); err == nil {
						val = v
					}
					(*value.params)[i] = paramPair{
						Key:   n.path[2:],
						Value: val,
					}

					value.handlers = n.handlers
					value.fullPath = n.fullPath
					return value

				default:
					panic("invalid node type")
				}
			}
		}

		// Path matches the node prefix exactly or is shorter.
		if path == prefix {
			// If this node has no handlers and is not a root-level
			// path, try backtracking through skipped nodes.
			if n.handlers == nil && path != "/" {
				for length := len(*skippedNodes); length > 0; length-- {
					skippedNode := (*skippedNodes)[length-1]
					*skippedNodes = (*skippedNodes)[:length-1]
					if strings.HasSuffix(skippedNode.path, path) {
						path = skippedNode.path
						n = skippedNode.node
						if value.params != nil {
							*value.params = (*value.params)[:skippedNode.paramsCount]
						}
						globalParamsCount = skippedNode.paramsCount
						continue walk
					}
				}
			}

			// Return handlers if this node is a terminal route.
			if value.handlers = n.handlers; value.handlers != nil {
				value.fullPath = n.fullPath
				return value
			}

			// TSR: the request path is "/" and a wildcard child exists
			// (but the route was registered without the trailing slash).
			if path == "/" && n.wildChild && n.nType != root {
				value.tsr = true
				return value
			}

			// TSR: static node at "/" with no handlers — suggest redirect.
			if path == "/" && n.nType == static {
				value.tsr = true
				return value
			}

			// Check for a child that handles the "/" segment (TSR).
			for i, c := range []byte(n.indices) {
				if c == '/' {
					n = n.children[i]
					value.tsr = (len(n.path) == 1 && n.handlers != nil) ||
						(n.nType == catchAll && n.children[0].handlers != nil)
					return value
				}
			}

			return value
		}

		// Prefix mismatch — check TSR (e.g. path "/user" vs prefix "/users/").
		value.tsr = path == "/" ||
			(len(prefix) == len(path)+1 && prefix[len(path)] == '/' &&
				path == prefix[:len(prefix)-1] && n.handlers != nil)

		// If no TSR candidate and the path is not "/", try backtracking.
		if !value.tsr && path != "/" {
			for length := len(*skippedNodes); length > 0; length-- {
				skippedNode := (*skippedNodes)[length-1]
				*skippedNodes = (*skippedNodes)[:length-1]
				if strings.HasSuffix(skippedNode.path, path) {
					path = skippedNode.path
					n = skippedNode.node
					if value.params != nil {
						*value.params = (*value.params)[:skippedNode.paramsCount]
					}
					globalParamsCount = skippedNode.paramsCount
					continue walk
				}
			}
		}

		return value
	}
}

// ── Router ───────────────────────────────────────────────────────────────────

// radixRouter wraps a set of per-method radix trees and provides high-level
// route registration and lookup. Each HTTP method gets its own tree so that
// method-based routing does not require linear scanning.
type radixRouter struct {
	trees methodTrees
}

// newRadixRouter creates a radixRouter pre-allocated for up to 9 HTTP
// methods (the number of standard methods supported by the Go HTTP server).
func newRadixRouter() *radixRouter {
	return &radixRouter{
		trees: make(methodTrees, 0, 9),
	}
}

// add registers a handler chain under the given HTTP method and path. The
// path is normalized (empty becomes "/", missing leading slash is added)
// before being inserted into the appropriate method tree.
func (r *radixRouter) add(method, path string, handlers []HandlerFunc) {
	if path == "" {
		path = "/"
	}
	if path[0] != '/' {
		path = "/" + path
	}

	root := r.trees.get(method)
	if root == nil {
		root = new(node)
		root.fullPath = "/"
		r.trees = append(r.trees, methodTree{method: method, root: root})
	}
	root.addRoute(path, handlers)
}

// routeResult aggregates the outcome of a route lookup: matched handlers,
// a TSR flag, and an allowed-methods string for 405 responses.
type routeResult struct {
	handlers      []HandlerFunc
	tsr           bool
	allowedMethod string
}

// find resolves a request method and path against the registered routes.
// Returns the matched handler chain, a TSR indicator, or a comma-separated
// list of allowed methods if no handler matches but other methods do (405).
// HEAD requests implicitly fall back to GET when no explicit HEAD handler
// is registered.
func (r *radixRouter) find(method, path string, ps *params, skipped *[]skippedNode) routeResult {
	// HEAD → fall back to GET
	if method == "HEAD" {
		if root := r.trees.get("HEAD"); root != nil {
			value := root.getValue(path, ps, skipped)
			if value.handlers != nil || value.tsr {
				return routeResult{handlers: value.handlers, tsr: value.tsr}
			}
		}
		method = "GET"
	}

	root := r.trees.get(method)
	if root == nil {
		return routeResult{allowedMethod: r.allowed(path, method)}
	}

	value := root.getValue(path, ps, skipped)
	if value.handlers != nil || value.tsr {
		return routeResult{handlers: value.handlers, tsr: value.tsr}
	}

	return routeResult{allowedMethod: r.allowed(path, method)}
}

// allowed returns a comma-separated string of HTTP methods that have a
// handler registered for the given path but are not the current method.
// Used to populate the Allow header in 405 responses.
func (r *radixRouter) allowed(path, currentMethod string) string {
	var methods []string
	for _, t := range r.trees {
		if t.method == currentMethod {
			continue
		}
		ps := make(params, 0, 4)
		var skipped []skippedNode
		v := t.root.getValue(path, &ps, &skipped)
		if v.handlers != nil {
			methods = append(methods, t.method)
		}
	}
	if len(methods) > 0 {
		return strings.Join(methods, ", ")
	}
	return ""
}

// convertServeMuxPattern splits a Go 1.22+ ServeMux-style pattern into
// method and path components, and translates {param} → :param and
// {path...} → *path for compatibility with the radix router syntax.
func convertServeMuxPattern(pattern string) (method, path string) {
	if strings.Contains(pattern, " ") {
		parts := strings.SplitN(pattern, " ", 2)
		method = parts[0]
		path = parts[1]
	} else {
		method = ""
		path = pattern
	}

	path = convertPath(path)
	return method, path
}

// convertPath translates Go 1.22+ ServeMux path patterns to the router's
// native syntax: {name} becomes :name and {name...} becomes *name.
// Paths without curly braces pass through unchanged.
func convertPath(p string) string {
	var b strings.Builder
	b.Grow(len(p))
	for i := 0; i < len(p); i++ {
		if p[i] == '{' {
			end := strings.IndexByte(p[i:], '}')
			if end == -1 {
				b.WriteByte(p[i])
				continue
			}
			name := p[i+1 : i+end]
			if strings.HasSuffix(name, "...") {
				b.WriteByte('*')
				b.WriteString(name[:len(name)-3])
			} else {
				b.WriteByte(':')
				b.WriteString(name)
			}
			i += end
		} else {
			b.WriteByte(p[i])
		}
	}
	return b.String()
}
