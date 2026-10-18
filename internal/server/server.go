// Package server exposes a library of worlds over a JSON API and serves the
// web UI.
package server

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"worldbuilder/internal/library"
	"worldbuilder/internal/vault"
)

// New returns the handler for the API, world attachments and the UI.
func New(lib *library.Library, ui fs.FS) http.Handler {
	s := &server{lib: lib}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/worlds", s.worlds)
	mux.HandleFunc("POST /api/worlds", s.createWorld)

	const w = "/api/worlds/{world}"
	mux.HandleFunc("GET "+w, s.inWorld(s.world))
	mux.HandleFunc("GET "+w+"/pages", s.inWorld(s.list))
	mux.HandleFunc("GET "+w+"/pages/{title...}", s.inWorld(s.get))
	mux.HandleFunc("GET "+w+"/source/{title...}", s.inWorld(s.source))
	mux.HandleFunc("PUT "+w+"/source/{title...}", s.inWorld(s.saveSource))
	mux.HandleFunc("GET "+w+"/search", s.inWorld(s.search))
	mux.HandleFunc("GET "+w+"/categories", s.inWorld(s.categories))
	mux.HandleFunc("GET "+w+"/categories/{name...}", s.inWorld(s.category))
	mux.HandleFunc("GET "+w+"/wanted", s.inWorld(s.wanted))
	mux.HandleFunc("GET "+w+"/timeline", s.inWorld(s.timeline))
	mux.HandleFunc("GET "+w+"/tree", s.inWorld(s.tree))
	mux.HandleFunc("POST "+w+"/move", s.inWorld(s.move))
	mux.HandleFunc("GET "+w+"/types", s.inWorld(s.types))
	mux.HandleFunc("PUT "+w+"/types", s.inWorld(s.saveTypes))
	mux.HandleFunc("GET /w/{world}/files/{path...}", s.inWorld(s.file))

	mux.Handle("GET /", spa(ui))
	return localOnly(mux)
}

type server struct{ lib *library.Library }

type worldHandler func(http.ResponseWriter, *http.Request, *vault.Vault)

// inWorld resolves the {world} path segment to an open vault.
func (s *server) inWorld(h worldHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := s.lib.Open(r.PathValue("world"))
		if err != nil {
			writeError(w, err)
			return
		}
		h(w, r, v)
	}
}

func (s *server) worlds(w http.ResponseWriter, r *http.Request) {
	worlds, err := s.lib.List()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]any{"root": s.lib.Root(), "worlds": worlds})
}

func (s *server) createWorld(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	info, err := s.lib.Create(req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, info)
}

func (s *server) world(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	writeJSON(w, v.World())
}

func (s *server) list(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	writeJSON(w, v.List())
}

func (s *server) get(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	view, err := v.Get(titleParam(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, view)
}

func (s *server) source(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	writeJSON(w, v.Source(titleParam(r)))
}

func (s *server) saveSource(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	var src vault.Source
	if err := json.NewDecoder(io.LimitReader(r.Body, 10<<20)).Decode(&src); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	title, err := v.SaveSource(titleParam(r), src)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]string{"title": title})
}

func (s *server) search(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		limit = 50
	}
	writeJSON(w, v.Search(r.URL.Query().Get("q"), limit))
}

func (s *server) categories(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	writeJSON(w, v.Categories())
}

func (s *server) category(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	writeJSON(w, v.CategoryMembers(strings.ReplaceAll(r.PathValue("name"), "_", " ")))
}

func (s *server) wanted(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	writeJSON(w, v.Wanted())
}

func (s *server) timeline(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	writeJSON(w, v.Timeline())
}

func (s *server) tree(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	writeJSON(w, v.Tree())
}

func (s *server) move(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	var req struct {
		Title  string `json:"title"`
		Parent string `json:"parent"` // empty: the top level
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := v.Move(req.Title, req.Parent); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, v.Tree())
}

func (s *server) types(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	writeJSON(w, v.Types())
}

func (s *server) saveTypes(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	var in []vault.TypeInfo
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	out, err := v.SaveTypes(in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, out)
}

// file serves an attachment such as an image. os.DirFS rejects ".." paths;
// dot-files (.git and the like) and directories are refused here.
func (s *server) file(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	name := r.PathValue("path")
	for _, seg := range strings.Split(name, "/") {
		if strings.HasPrefix(seg, ".") {
			http.NotFound(w, r)
			return
		}
	}
	root := os.DirFS(v.Root())
	if info, err := fs.Stat(root, name); err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeFileFS(w, r, root, name)
}

// titleParam reads a title from the URL, where "_" stands for a space.
func titleParam(r *http.Request) string {
	return strings.ReplaceAll(r.PathValue("title"), "_", " ")
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	var parseErr *vault.ParseError
	var invalid *vault.InvalidError
	switch {
	case errors.Is(err, vault.ErrNotFound), errors.Is(err, library.ErrNoWorld):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, vault.ErrExists), errors.Is(err, library.ErrWorldExist):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, library.ErrBadName):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.As(err, &parseErr), errors.As(err, &invalid):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		log.Print(err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// spa serves the built UI, falling back to index.html so that client-side
// routes like /wiki/Title work on reload.
func spa(ui fs.FS) http.Handler {
	files := http.FileServerFS(ui)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" {
			if info, err := fs.Stat(ui, name); err == nil && !info.IsDir() {
				files.ServeHTTP(w, r)
				return
			}
		}
		if _, err := fs.Stat(ui, "index.html"); err != nil {
			http.Error(w, "UI is not built: run `make` first", http.StatusNotFound)
			return
		}
		http.ServeFileFS(w, r, ui, "index.html")
	})
}

// localOnly guards against other websites talking to the local server: the
// Host header must be a loopback name (defeats DNS rebinding), and requests
// carrying an Origin must come from this same host (defeats cross-site
// writes).
func localOnly(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			if u, err := url.Parse(origin); err != nil || u.Host != r.Host {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}
