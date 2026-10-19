// Package agent runs the writing assistant: the author's own Claude Code,
// started without a window in the world's folder, as the Claude extensions
// for Obsidian and VS Code do. It uses whatever account Claude Code is
// signed in with; Worldbuilder holds no key.
//
// The assistant gets only file tools — read, search, edit, write — so it
// can work on articles but cannot run commands, browse or delete.
package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrNotFound reports that Claude Code is not installed where we look.
var ErrNotFound = errors.New("claude code not found")

// tools are all the assistant may use: enough to read and write articles.
const tools = "Read,Edit,Write,Glob,Grep"

// Find returns the Claude Code executable: on the PATH, or in the places
// its installers put it (a desktop app may start without the shell's PATH).
func Find() (string, error) {
	if p, err := exec.LookPath("claude"); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".local", "bin", "claude"),
		filepath.Join(home, ".claude", "local", "claude"),
		"/usr/local/bin/claude",
		"/opt/homebrew/bin/claude",
	}
	if runtime.GOOS == "windows" {
		candidates = []string{
			filepath.Join(home, ".local", "bin", "claude.exe"),
			filepath.Join(os.Getenv("APPDATA"), "npm", "claude.cmd"),
		}
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return "", ErrNotFound
}

// Event is one step of an answer, as the chat shows it.
type Event struct {
	Type    string `json:"type"`              // "session", "text", "tool", "done", "error"
	Text    string `json:"text,omitempty"`    // a piece of the answer, or the whole of it when done
	Tool    string `json:"tool,omitempty"`    // Read, Edit, Write, Glob, Grep
	Target  string `json:"target,omitempty"`  // the file (relative to the world) or the search
	Session string `json:"session,omitempty"` // to continue the conversation
	Error   string `json:"error,omitempty"`
}

// Request is one message to the assistant.
type Request struct {
	Bin     string // Claude Code
	Dir     string // the world's folder: where it works, and all it may edit
	Prompt  string
	Rules   string // appended to Claude Code's own system prompt
	Session string // a conversation to continue; empty starts one
}

// Run sends a message and reports the answer as it comes, until it is done
// or ctx ends (the chat closed or stopped it).
func Run(ctx context.Context, req Request, emit func(Event)) error {
	args := []string{
		"-p",
		"--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--tools", tools,
		"--permission-mode", "acceptEdits",
		"--permission-prompts", "none", // nobody is there to answer: what would ask is refused
		"--strict-mcp-config", // no MCP servers from the author's other setups
		"--append-system-prompt", req.Rules,
	}
	if req.Session != "" {
		args = append(args, "--resume", req.Session)
	}
	cmd := exec.CommandContext(ctx, req.Bin, args...)
	cmd.Dir = req.Dir
	cmd.Stdin = strings.NewReader(req.Prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	done := false
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20) // tool results can be whole articles
	for sc.Scan() {
		var e streamEvent
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		for _, ev := range e.events(req.Dir) {
			if ev.Type == "done" || ev.Type == "error" {
				done = true
			}
			emit(ev)
		}
	}
	err = cmd.Wait()
	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case err != nil && !done:
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		emit(Event{Type: "error", Error: msg})
	case !done:
		emit(Event{Type: "error", Error: "Claude Code finished without an answer"})
	}
	return nil
}

// streamEvent is the part of Claude Code's stream-json output the chat needs.
type streamEvent struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
	Event     struct {
		Type  string `json:"type"`
		Delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	} `json:"event"`
	Message struct {
		Content []struct {
			Type  string          `json:"type"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	} `json:"message"`
	Result  string   `json:"result"`
	IsError bool     `json:"is_error"`
	Errors  []string `json:"errors"`
	Limit   *struct {
		Status string `json:"status"`
	} `json:"rate_limit_info"`
}

func (e streamEvent) events(dir string) []Event {
	switch e.Type {
	case "system":
		if e.Subtype == "init" && e.SessionID != "" {
			return []Event{{Type: "session", Session: e.SessionID}}
		}
	case "stream_event":
		if e.Event.Type == "content_block_delta" && e.Event.Delta.Type == "text_delta" {
			return []Event{{Type: "text", Text: e.Event.Delta.Text}}
		}
	case "assistant":
		var out []Event
		for _, c := range e.Message.Content {
			if c.Type == "tool_use" {
				out = append(out, Event{Type: "tool", Tool: c.Name, Target: toolTarget(c.Input, dir)})
			}
		}
		return out
	case "rate_limit_event":
		if e.Limit != nil && e.Limit.Status == "rejected" {
			return []Event{{Type: "error", Error: "limit"}}
		}
	case "result":
		if e.IsError {
			msg := e.Result
			if msg == "" {
				msg = strings.Join(e.Errors, "; ")
			}
			return []Event{{Type: "error", Error: msg, Session: e.SessionID}}
		}
		return []Event{{Type: "done", Text: e.Result, Session: e.SessionID}}
	}
	return nil
}

// toolTarget is what a tool call works on, for the chat to name: a file
// relative to the world, or what is searched for.
func toolTarget(input json.RawMessage, dir string) string {
	var in struct {
		FilePath string `json:"file_path"`
		Pattern  string `json:"pattern"`
		Path     string `json:"path"`
	}
	_ = json.Unmarshal(input, &in)
	switch {
	case in.FilePath != "":
		if rel, err := filepath.Rel(dir, in.FilePath); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
		return in.FilePath
	case in.Pattern != "":
		return in.Pattern
	}
	return ""
}

// Version is the installed Claude Code's version line, for the chat to show.
func Version(ctx context.Context, bin string) string {
	out, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Mentioned is something the author pointed at with @ in a message: an
// article (with its file) or a folder of the world.
type Mentioned struct {
	Title string // an article's title; empty for a folder
	Path  string // the file, or the folder ending in "/"
}

// Mentions is the note added after a message naming what the author
// pointed at, so the assistant reads those files first.
func Mentions(items []Mentioned) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[App note, not written by the author: the @ mentions in the message point at these; read them before answering.")
	for _, m := range items {
		if m.Title != "" {
			fmt.Fprintf(&b, "\n- the article %q: file %s", m.Title, m.Path)
		} else {
			fmt.Fprintf(&b, "\n- the folder %s: the articles in it and its subfolders", m.Path)
		}
	}
	b.WriteString("]")
	return b.String()
}

// Describe is the note added after a message about the article open in
// the app, so "this article" means something.
func Describe(title, path string) string {
	return fmt.Sprintf("[App note, not written by the author: the article %q is open, file %s.]", title, path)
}
