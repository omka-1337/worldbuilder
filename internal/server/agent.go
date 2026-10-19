package server

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"worldbuilder/internal/agent"
	"worldbuilder/internal/vault"
)

// agentRulesFile is where an author adds rules of their own for the
// assistant in one world; hidden, so it is no article.
const agentRulesFile = ".worldbuilder/agent.md"

// claude finds Claude Code once and remembers it; a failed search is
// tried again next time, in case it was installed meanwhile.
type claudeBin struct {
	mu   sync.Mutex
	path string
}

func (c *claudeBin) get() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.path != "" {
		return c.path, nil
	}
	p, err := agent.Find()
	if err == nil {
		c.path = p
	}
	return p, err
}

// agentStatus tells the chat whether the assistant can run.
func (s *server) agentStatus(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	bin, err := s.claude.get()
	out := map[string]any{"available": err == nil, "rulesFile": agentRulesFile}
	if err == nil {
		out["version"] = agent.Version(r.Context(), bin)
	}
	writeJSON(w, out)
}

// agentChat sends the author's message to the assistant and streams its
// answer back as lines of JSON, one agent.Event each.
func (s *server) agentChat(w http.ResponseWriter, r *http.Request, v *vault.Vault) {
	var req struct {
		Message string `json:"message"`
		Session string `json:"session"`
		Article string `json:"article"` // the title open in the app, if any
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil || req.Message == "" {
		http.Error(w, "a message is needed", http.StatusBadRequest)
		return
	}
	bin, err := s.claude.get()
	if errors.Is(err, agent.ErrNotFound) {
		http.Error(w, "Claude Code is not installed", http.StatusServiceUnavailable)
		return
	}

	prompt := req.Message
	if req.Article != "" {
		if path, ok := v.PathOf(req.Article); ok {
			prompt += "\n\n" + agent.Describe(req.Article, path)
		}
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)
	emit := func(e agent.Event) {
		if err := enc.Encode(e); err == nil && flusher != nil {
			flusher.Flush()
		}
	}
	err = agent.Run(r.Context(), agent.Request{
		Bin:     bin,
		Dir:     v.Root(),
		Prompt:  prompt,
		Rules:   agent.Rules(s.agentWorld(v)),
		Session: req.Session,
	}, emit)
	if err != nil && r.Context().Err() == nil {
		log.Printf("agent: %v", err)
		emit(agent.Event{Type: "error", Error: err.Error()})
	}
	// The assistant may have changed articles: read them again now rather
	// than wait for the file watcher.
	if err := v.Reload(); err != nil {
		log.Printf("reload after agent: %v", err)
	}
}

// agentWorld describes the world for the assistant's rules.
func (s *server) agentWorld(v *vault.Vault) agent.World {
	world := v.World()
	out := agent.World{
		Name:     world.Name,
		MainPage: world.MainPage,
		Formats:  world.Calendar.Formats,
		Era:      world.Era,
		Dates:    v.DateSamples(4),
	}
	for _, u := range world.Calendar.Units {
		out.Units = append(out.Units, agent.Unit{Name: u.Name, Write: u.Write, Names: u.Names})
	}
	for _, t := range v.Types() {
		at := agent.Type{Key: t.Key, Label: t.Label, Event: t.Event}
		for _, f := range t.Fields {
			at.Fields = append(at.Fields, agent.Field{Key: f.Key, Label: f.Label, Kind: f.Type, End: f.End})
		}
		out.Types = append(out.Types, at)
	}
	if extra, err := os.ReadFile(filepath.Join(v.Root(), filepath.FromSlash(agentRulesFile))); err == nil {
		out.Extra, out.ExtraPath = string(extra), agentRulesFile
	}
	return out
}
