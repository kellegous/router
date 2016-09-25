package router

import (
	"net/http"
	"strings"
)

// Verb ...
type Verb int

const (
	// Delete is a constant representing the HTTP verb, DELETE
	Delete Verb = iota

	// Get is a constant representing the HTTP verb, GET
	Get

	// Head is a constant representing the HTTP verb, HEAD
	Head

	// Options is a constant representing the HTTP verb, OPTIONS
	Options

	// Patch is a constant representing the HTTP verb, PATCH
	Patch

	// Post is a constant representing the HTTP verb, POST
	Post

	// Put is a constant representing the HTTP verb, PUT
	Put
	unknownVerb
)

var verbs = map[string]Verb{
	"DELETE":  Delete,
	"GET":     Get,
	"HEAD":    Head,
	"OPTIONS": Options,
	"PATCH":   Patch,
	"POST":    Post,
	"PUT":     Put,
}

// Handler instances are just request handler functions
type Handler func(http.ResponseWriter, *http.Request, []string)

// Builder allows the creation of an immutable router so locking can be avoided
// at serving time.
type Builder interface {
	Handle(Verb, string, Handler)
	HandleAll(string, Handler)
	Build() http.Handler
}

type router struct {
	// The child nodes underneath this node.
	ch map[string]*router
	rt *route
}

type route struct {
	vb [unknownVerb]Handler
}

func (r *router) place(path string) *router {
	if path == "" {
		return r
	}

	if r.ch == nil {
		r.ch = map[string]*router{}
	}

	k := path
	ix := strings.Index(path, "/")
	if ix >= 0 {
		k = path[:ix+1]
	}

	ch := r.ch[k]
	if ch == nil {
		ch = &router{}
		r.ch[k] = ch
	}

	return ch.place(path[len(k):])
}

func (r *router) find(path string, names *[]string) *router {
	if path == "" {
		return r
	}

	// find the next component, the text up until the next /. If there isn't a /,
	// just take the whole path.
	k := path
	ix := strings.Index(path, "/")
	if ix >= 0 {
		k = path[:ix+1]
	}

	// Check for a child under that path component.
	if c := r.ch[k]; c != nil {
		// If we find a child, continue our search with the rest of the path.
		if h := c.find(path[len(k):], names); h != nil {
			if h.rt != nil {
				return h
			}
		}
	}

	// Now we check if a wildcard node is registered. There are two wildcard types
	// "*" and "*/".
	w := "*"
	if k[len(k)-1] == '/' {
		w = "*/"
	}

	if c := r.ch[w]; c != nil {
		*names = append(*names, strings.TrimRight(k, "/"))
		if h := c.find(path[len(k):], names); h != nil {
			if h.rt != nil {
				return h
			}
		}
	}

	return nil
}

func (r *router) set(verb Verb, h Handler) {
	if r.rt == nil {
		r.rt = &route{}
	}
	r.rt.vb[verb] = h
}

// ServeHTTP handles the HTTP request.
func (r *router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	v, ok := verbs[req.Method]
	if !ok {
		http.Error(w,
			http.StatusText(http.StatusMethodNotAllowed),
			http.StatusMethodNotAllowed)
		return
	}

	var names []string

	t := r.find(req.URL.Path[1:], &names)
	if t == nil || t.rt == nil {
		http.NotFound(w, req)
		return
	}

	if h := t.rt.vb[v]; h != nil {
		h(w, req, names)
		return
	}

	http.Error(w,
		http.StatusText(http.StatusMethodNotAllowed),
		http.StatusMethodNotAllowed)
}

// Handle registers a verb/route in the router.
func (r *router) Handle(verb Verb, path string, h Handler) {
	r.place(path[1:]).set(verb, h)
}

// HandleAll registers a route on all verbs in the router.
func (r *router) HandleAll(path string, h Handler) {
	n := r.place(path[1:])
	for i := 0; i < int(unknownVerb); i++ {
		n.set(Verb(i), h)
	}
}

// Build takes a snapshot of the contents in builder and converts it to a
// http.Handler for serving requests. It also clears the content in the Builder.
func (r *router) Build() http.Handler {
	n := *r
	*r = router{}
	return &n
}

// New creates a Builder.
func New() Builder {
	return &router{}
}
