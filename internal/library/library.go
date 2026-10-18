// Package library manages a folder of worlds: each subdirectory is one
// world, named after it.
package library

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"worldbuilder/internal/history"
	"worldbuilder/internal/vault"
)

var (
	ErrNoWorld    = errors.New("world not found")
	ErrWorldExist = errors.New("a world with this name already exists")
	ErrBadName    = errors.New("invalid world name")
)

//go:embed default-world.yaml
var defaultWorld string

//go:embed default-main.md
var defaultMain string

// DefaultRoot is <Documents>/Worldbuilder.
func DefaultRoot() (string, error) {
	docs, err := documentsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(docs, "Worldbuilder"), nil
}

// Library opens worlds on demand and keeps them open, each with a file
// watcher bound to the library's context.
type Library struct {
	root string
	ctx  context.Context

	mu      sync.Mutex
	open    map[string]*vault.Vault
	history map[string]*history.Repo
}

// New creates root if needed. Watchers of opened worlds stop with ctx.
func New(ctx context.Context, root string) (*Library, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Library{root: root, ctx: ctx, open: map[string]*vault.Vault{}, history: map[string]*history.Repo{}}, nil
}

func (l *Library) Root() string { return l.root }

// WorldInfo is a world as shown on the home page. ID is the folder name.
type WorldInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Pages       int       `json:"pages"`
	Modified    time.Time `json:"modified"`
}

// List returns the worlds in the library, most recently changed first.
func (l *Library) List() ([]WorldInfo, error) {
	entries, err := os.ReadDir(l.root)
	if err != nil {
		return nil, err
	}
	out := []WorldInfo{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, err := l.info(e.Name())
		if err != nil {
			continue
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
	return out, nil
}

// info reads a world's config and counts its articles without parsing them.
func (l *Library) info(id string) (WorldInfo, error) {
	dir := filepath.Join(l.root, id)
	w, err := vault.LoadWorld(dir)
	if err != nil {
		return WorldInfo{}, err
	}
	info := WorldInfo{ID: id, Name: w.Name, Description: w.Description}
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != dir && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if fi, err := d.Info(); err == nil && fi.ModTime().After(info.Modified) {
			info.Modified = fi.ModTime()
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".md") {
			info.Pages++
		}
		return nil
	})
	return info, err
}

// Create makes a new world folder with a default world.yaml and main page.
func (l *Library) Create(name string) (WorldInfo, error) {
	name = strings.Join(strings.Fields(name), " ")
	id := vault.SafeName(name)
	if name == "" || id == "" {
		return WorldInfo{}, ErrBadName
	}
	dir := filepath.Join(l.root, id)
	if err := os.Mkdir(dir, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return WorldInfo{}, ErrWorldExist
		}
		return WorldInfo{}, err
	}

	quoted, _ := json.Marshal(name) // JSON strings are valid YAML scalars
	files := map[string]string{
		"world.yaml": fmt.Sprintf(defaultWorld, quoted),
		"Головна.md": fmt.Sprintf(defaultMain, name),
	}
	for file, content := range files {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
			return WorldInfo{}, err
		}
	}
	// A world has a history from its first moment.
	if repo, err := history.Open(dir, "Створено світ"); err != nil {
		fmt.Fprintf(os.Stderr, "history %s: %v\n", id, err)
	} else {
		l.mu.Lock()
		l.history[id] = repo
		l.mu.Unlock()
	}
	return l.info(id)
}

// History returns the versions of world id, starting them on first use for
// a world that has none yet (one made before, or copied in).
func (l *Library) History(id string) (*history.Repo, error) {
	if _, err := l.Open(id); err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if repo, ok := l.history[id]; ok {
		return repo, nil
	}
	repo, err := history.Open(filepath.Join(l.root, id), "Перша версія")
	if err != nil {
		return nil, err
	}
	l.history[id] = repo
	return repo, nil
}

// Open returns the world in folder id, loading it on first use.
func (l *Library) Open(id string) (*vault.Vault, error) {
	if id == "" || id != filepath.Base(id) || strings.HasPrefix(id, ".") || strings.ContainsAny(id, `/\`) {
		return nil, ErrNoWorld
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if v, ok := l.open[id]; ok {
		return v, nil
	}

	dir := filepath.Join(l.root, id)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil, ErrNoWorld
	}
	v, err := vault.Open(dir, "/w/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	go func() {
		if err := v.Watch(l.ctx); err != nil {
			fmt.Fprintf(os.Stderr, "watch %s: %v\n", id, err)
		}
	}()
	l.open[id] = v
	return v, nil
}
