package zen

import (
	"net/url"
	"strings"
)

// ── param types ────────────────────────────────────────────────────────────

type paramPair struct {
	Key   string
	Value string
}

type params []paramPair

func (ps params) get(name string) (string, bool) {
	for _, p := range ps {
		if p.Key == name {
			return p.Value, true
		}
	}
	return "", false
}

// ── method trees ───────────────────────────────────────────────────────────

type methodTree struct {
	method string
	root   *node
}

type methodTrees []methodTree

func (trees methodTrees) get(method string) *node {
	for _, t := range trees {
		if t.method == method {
			return t.root
		}
	}
	return nil
}

// ── radix tree node ────────────────────────────────────────────────────────

type nodeType uint8

const (
	static nodeType = iota
	root
	param
	catchAll
)

type node struct {
	path      string
	indices   string
	children  []*node
	handlers  []HandlerFunc
	priority  uint32
	nType     nodeType
	wildChild bool
	fullPath  string
}

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

func (n *node) addChild(child *node) {
	if n.wildChild && len(n.children) > 0 {
		wildcardChild := n.children[len(n.children)-1]
		n.children = append(n.children[:len(n.children)-1], child, wildcardChild)
	} else {
		n.children = append(n.children, child)
	}
}

func countParams(path string) int {
	colons := strings.Count(path, ":")
	stars := strings.Count(path, "*")
	return colons + stars
}

func longestCommonPrefix(a, b string) int {
	i := 0
	max_ := min(len(a), len(b))
	for i < max_ && a[i] == b[i] {
		i++
	}
	return i
}

// ── addRoute ───────────────────────────────────────────────────────────────

func (n *node) addRoute(path string, handlers []HandlerFunc) {
	fullPath := path
	n.priority++

	if len(n.path) == 0 && len(n.children) == 0 {
		n.insertChild(path, fullPath, handlers)
		n.nType = root
		return
	}

	parentFullPathIndex := 0

walk:
	for {
		i := longestCommonPrefix(path, n.path)

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

		if i < len(path) {
			path = path[i:]
			c := path[0]

			if n.nType == param && c == '/' && len(n.children) == 1 {
				parentFullPathIndex += len(n.path)
				n = n.children[0]
				n.priority++
				continue walk
			}

			for i, max_ := 0, len(n.indices); i < max_; i++ {
				if c == n.indices[i] {
					parentFullPathIndex += len(n.path)
					i = n.incrementChildPrio(i)
					n = n.children[i]
					continue walk
				}
			}

			if c != ':' && c != '*' && n.nType != catchAll {
				n.indices += string([]byte{c})
				child := &node{fullPath: fullPath}
				n.addChild(child)
				n.incrementChildPrio(len(n.indices) - 1)
				n = child
			} else if n.wildChild {
				n = n.children[len(n.children)-1]
				n.priority++

				if len(path) >= len(n.path) && n.path == path[:len(n.path)] &&
					n.nType != catchAll &&
					(len(n.path) >= len(path) || path[len(n.path)] == '/') {
					continue walk
				}

				pathSeg := path
				if n.nType != catchAll {
					pathSeg, _, _ = strings.Cut(pathSeg, "/")
				}
				prefix := fullPath[:strings.Index(fullPath, pathSeg)] + n.path
				panic("'" + pathSeg + "' in new path '" + fullPath +
					"' conflicts with existing wildcard '" + n.path +
					"' in existing prefix '" + prefix + "'")
			}

			n.insertChild(path, fullPath, handlers)
			return
		}

		if n.handlers != nil {
			panic("handlers are already registered for path '" + fullPath + "'")
		}
		n.handlers = handlers
		n.fullPath = fullPath
		return
	}
}

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

	n.path = path
	n.handlers = handlers
	n.fullPath = fullPath
}

// ── getValue ───────────────────────────────────────────────────────────────

type nodeValue struct {
	handlers []HandlerFunc
	params   *params
	tsr      bool
	fullPath string
}

type skippedNode struct {
	path        string
	node        *node
	paramsCount int16
}

func (n *node) getValue(path string, ps *params, skippedNodes *[]skippedNode, unescape bool) (value nodeValue) {
	var globalParamsCount int16

walk:
	for {
		prefix := n.path
		if len(path) > len(prefix) {
			if path[:len(prefix)] == prefix {
				path = path[len(prefix):]

				idxc := path[0]
				for i, c := range []byte(n.indices) {
					if c == idxc {
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

					value.tsr = path == "/" && n.handlers != nil
					return value
				}

				n = n.children[len(n.children)-1]
				globalParamsCount++

				switch n.nType {
				case param:
					end := 0
					for end < len(path) && path[end] != '/' {
						end++
					}

					if ps != nil {
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
						if unescape {
							if v, err := url.QueryUnescape(val); err == nil {
								val = v
							}
						}
						(*value.params)[i] = paramPair{
							Key:   n.path[1:],
							Value: val,
						}
					}

					if end < len(path) {
						if len(n.children) > 0 {
							path = path[end:]
							n = n.children[0]
							continue walk
						}

						value.tsr = len(path) == end+1
						return value
					}

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
					if ps != nil {
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
						if unescape {
							if v, err := url.QueryUnescape(val); err == nil {
								val = v
							}
						}
						(*value.params)[i] = paramPair{
							Key:   n.path[2:],
							Value: val,
						}
					}

					value.handlers = n.handlers
					value.fullPath = n.fullPath
					return value

				default:
					panic("invalid node type")
				}
			}
		}

		if path == prefix {
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

			if value.handlers = n.handlers; value.handlers != nil {
				value.fullPath = n.fullPath
				return value
			}

			if path == "/" && n.wildChild && n.nType != root {
				value.tsr = true
				return value
			}

			if path == "/" && n.nType == static {
				value.tsr = true
				return value
			}

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

		value.tsr = path == "/" ||
			(len(prefix) == len(path)+1 && prefix[len(path)] == '/' &&
				path == prefix[:len(prefix)-1] && n.handlers != nil)

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

// ── Router ─────────────────────────────────────────────────────────────────

type radixRouter struct {
	trees     methodTrees
	maxParams int
}

func newRadixRouter() *radixRouter {
	return &radixRouter{
		trees: make(methodTrees, 0, 9),
	}
}

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

	if pc := countParams(path); pc > r.maxParams {
		r.maxParams = pc
	}
}

type routeResult struct {
	handlers      []HandlerFunc
	ps            params
	tsr           bool
	allowedMethod string
}

func (r *radixRouter) find(method, path string, ps *params, skipped *[]skippedNode) routeResult {
	// HEAD → fall back to GET
	if method == "HEAD" {
		if root := r.trees.get("HEAD"); root != nil {
			value := root.getValue(path, ps, skipped, true)
			if value.handlers != nil || value.tsr {
				return routeResult{handlers: value.handlers, ps: *ps, tsr: value.tsr}
			}
		}
		method = "GET"
	}

	root := r.trees.get(method)
	if root == nil {
		return routeResult{allowedMethod: r.allowed(path, method)}
	}

	value := root.getValue(path, ps, skipped, true)
	if value.handlers != nil || value.tsr {
		return routeResult{handlers: value.handlers, ps: *ps, tsr: value.tsr}
	}

	return routeResult{allowedMethod: r.allowed(path, method)}
}

func (r *radixRouter) allowed(path, currentMethod string) string {
	var methods []string
	for _, t := range r.trees {
		if t.method == currentMethod {
			continue
		}
		ps := make(params, 0, 4)
		var skipped []skippedNode
		v := t.root.getValue(path, &ps, &skipped, true)
		if v.handlers != nil {
			methods = append(methods, t.method)
		}
	}
	if len(methods) > 0 {
		return strings.Join(methods, ", ")
	}
	return ""
}

// convertServeMuxPattern converts "GET /path" or "/path" patterns
// and translates {param} → :param and {path...} → *path
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
