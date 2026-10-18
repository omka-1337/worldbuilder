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

	"worldbuilder/internal/history"
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
	mux.HandleFunc("POST "+w+"/rename", s.inWorld(s.rename))
	mux.HandleFunc("DELETE "+w+"/pages/{title...}", s.inWorld(s.deletePage))
	mux.HandleFunc("POST "+w+"/folders", s.inWorld(s.createFolder))
	mux.HandleFunc("POST "+w+"/folders/rename", s.inWorld(s.renameFolder))
	mux.HandleFunc("POST "+w+"/folders/delete", s.inWorld(s.deleteFolder))
	mux.HandleFunc("PUT "+w+"/calendar", s.inWorld(s.saveCalendar))
	mux.HandleFunc("POST "+w+"/calendar/preview", s.inWorld(s.previewCalendar))
	mux.HandleFunc("GET "+w+"/types", s.inWorld(s.types))
	mux.HandleFunc("PUT "+w+"/types", s.inWorld(s.saveTypes))
	mux.HandleFunc("GET "+w+"/changes", s.inWorld(s.changes))
	mux.HandleFunc("GET "+w+"/versions", s.inWorld(s.versions))
	mux.HandleFunc("POST "+w+"/versions", s.inWorld(s.saveVersion))
	mux.HandleFunc("GET /w/{world}/files/{path...}", s.inWorld(s.file))
	mux.HandleFunc("POST "+w+"/images/{title...}", s.inWorld(s.uploadImage))

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

// treeRequest is the body of the tree's actions; each uses some fields.
type treeRequest struct {
	Title    string `json:"title"`
	NewTitle string `json:"newTitle"`
	Folder   string `json:"folder"` // a folder path; empty: the top level
	Name     string `json:"name"`
}

func decodeTree(w http.ResponseWriter, r *http.Request) (treeRequest, bool) {
	var req treeRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return req, false
	}
	return req, true
}

func (s *server) move(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	if req, ok := decodeTree(w, r); ok {
		s.treeResult(w, v, v.Move(req.Title, req.Folder))
	}
}

func (s *server) rename(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	req, ok := decodeTree(w, r)
	if !ok {
		return
	}
	title, err := v.Rename(req.Title, req.NewTitle)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]string{"title": title})
}

func (s *server) deletePage(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	s.treeResult(w, v, v.Delete(titleParam(r)))
}

func (s *server) createFolder(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	if req, ok := decodeTree(w, r); ok {
		_, err := v.CreateFolder(req.Folder)
		s.treeResult(w, v, err)
	}
}

func (s *server) renameFolder(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	if req, ok := decodeTree(w, r); ok {
		_, err := v.RenameFolder(req.Folder, req.Name)
		s.treeResult(w, v, err)
	}
}

func (s *server) deleteFolder(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	if req, ok := decodeTree(w, r); ok {
		s.treeResult(w, v, v.DeleteFolder(req.Folder))
	}
}

// treeResult answers a tree action with the new tree, or its error.
func (s *server) treeResult(w http.ResponseWriter, v *vault.Vault, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, v.Tree())
}

func (s *server) saveCalendar(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	var cal vault.Calendar
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&cal); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	world, err := v.SaveCalendar(cal)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, world)
}

func (s *server) previewCalendar(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	var req struct {
		Calendar vault.Calendar `json:"calendar"`
		Sample   string         `json:"sample"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, v.PreviewCalendar(req.Calendar, req.Sample))
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
	// A file opened on its own (an SVG, an HTML page someone put in the
	// world) must not run scripts with access to this app.
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFileFS(w, r, root, name)
}

// maxImage bounds uploads; infobox pictures are not meant to be originals.
const maxImage = 20 << 20

func (s *server) uploadImage(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	data, err := io.ReadAll(io.LimitReader(r.Body, maxImage+1))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(data) > maxImage {
		http.Error(w, "image too large", http.StatusRequestEntityTooLarge)
		return
	}
	name, url, err := v.SaveImage(titleParam(r), data)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]string{"name": name, "url": url})
}

// changes lists what differs from the world's last saved version, naming
// articles by title.
func (s *server) changes(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	repo, err := s.lib.History(r.PathValue("world"))
	if err != nil {
		writeError(w, err)
		return
	}
	changes, err := repo.Changes()
	if err != nil {
		writeError(w, err)
		return
	}
	type change struct {
		history.Change
		Title string `json:"title,omitempty"`
	}
	out := make([]change, len(changes))
	for i, c := range changes {
		out[i].Change = c
		out[i].Title, _ = v.TitleAt(c.Path)
	}
	writeJSON(w, out)
}

func (s *server) versions(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	repo, err := s.lib.History(r.PathValue("world"))
	if err != nil {
		writeError(w, err)
		return
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		limit = 50
	}
	log, err := repo.Log(limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, log)
}

func (s *server) saveVersion(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	repo, err := s.lib.History(r.PathValue("world"))
	if err != nil {
		writeError(w, err)
		return
	}
	version, err := repo.Commit(req.Message)
	if errors.Is(err, history.ErrNoChanges) {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if errors.Is(err, history.ErrNoMessage) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, version)
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
