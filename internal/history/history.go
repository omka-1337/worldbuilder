// Package history keeps a world's versions in a Git repository inside its
// folder. Git runs in-process (go-git), so authors need nothing installed;
// the repository is an ordinary one that any Git tool can open.
package history

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var (
	// ErrNoChanges reports a commit with nothing to save.
	ErrNoChanges = errors.New("no changes to save")
	// ErrNoMessage reports a commit without a description.
	ErrNoMessage = errors.New("a version needs a description")
)

// gitignore keeps deleted articles out of the history: the trash is a
// place to recover them from, not part of the world.
const gitignore = ".trash/\n"

// Repo is one world's history. Its methods are safe for concurrent use.
type Repo struct {
	root string
	mu   sync.Mutex
	repo *git.Repository
}

// Open opens the world's repository, creating it on first use with a first
// version of everything already in the folder, saved under message.
func Open(root, message string) (*Repo, error) {
	repo, err := git.PlainOpen(root)
	if err == nil {
		return &Repo{root: root, repo: repo}, nil
	}
	if !errors.Is(err, git.ErrRepositoryNotExists) {
		return nil, err
	}

	repo, err = git.PlainInitWithOptions(root, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		return nil, err
	}
	ignore := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(ignore); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(ignore, []byte(gitignore), 0o644); err != nil {
			return nil, err
		}
	}
	r := &Repo{root: root, repo: repo}
	if _, err := r.Commit(message); err != nil && !errors.Is(err, ErrNoChanges) {
		return nil, err
	}
	return r, nil
}

// Change is a file that differs from the last saved version.
type Change struct {
	Path  string `json:"path"` // slash-separated, relative to the world
	State string `json:"state"` // "added", "modified" or "deleted"
}

// Changes lists what is not saved yet, by path.
func (r *Repo) Changes() ([]Change, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	wt, err := r.repo.Worktree()
	if err != nil {
		return nil, err
	}
	status, err := wt.Status()
	if err != nil {
		return nil, err
	}
	out := []Change{}
	for path, s := range status {
		// The working tree against the last version: what is staged does
		// not matter, the app always saves everything at once.
		code := s.Worktree
		if code == git.Unmodified {
			code = s.Staging
		}
		state := ""
		switch code {
		case git.Untracked, git.Added, git.Copied:
			state = "added"
		case git.Deleted:
			state = "deleted"
		case git.Modified, git.Renamed, git.UpdatedButUnmerged:
			state = "modified"
		default:
			continue
		}
		if s.Staging == git.Added && s.Worktree == git.Deleted {
			continue // added and then removed again: nothing to save
		}
		out = append(out, Change{Path: path, State: state})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Version is one saved state of the world.
type Version struct {
	Hash    string    `json:"hash"`
	Message string    `json:"message"`
	Author  string    `json:"author"`
	When    time.Time `json:"when"`
}

func versionOf(c *object.Commit) Version {
	return Version{
		Hash:    c.Hash.String(),
		Message: strings.TrimSpace(c.Message),
		Author:  c.Author.Name,
		When:    c.Author.When,
	}
}

// Commit saves every change in the world as a new version.
func (r *Repo) Commit(message string) (Version, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return Version{}, ErrNoMessage
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	wt, err := r.repo.Worktree()
	if err != nil {
		return Version{}, err
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return Version{}, err
	}
	status, err := wt.Status()
	if err != nil {
		return Version{}, err
	}
	staged := false
	for _, s := range status {
		if s.Staging != git.Unmodified && s.Staging != git.Untracked {
			staged = true
			break
		}
	}
	if !staged {
		return Version{}, ErrNoChanges
	}
	hash, err := wt.Commit(message, &git.CommitOptions{Author: author()})
	if err != nil {
		return Version{}, err
	}
	c, err := r.repo.CommitObject(hash)
	if err != nil {
		return Version{}, err
	}
	return versionOf(c), nil
}

// Log returns up to n versions, newest first.
func (r *Repo) Log(n int) ([]Version, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []Version{}
	head, err := r.repo.Head()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return out, nil // nothing saved yet
	}
	if err != nil {
		return nil, err
	}
	iter, err := r.repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	for len(out) < n {
		c, err := iter.Next()
		if err != nil {
			break // io.EOF at the first version
		}
		out = append(out, versionOf(c))
	}
	return out, nil
}

// author signs versions with the user's Git identity when they have one,
// and with their account name otherwise.
func author() *object.Signature {
	sig := &object.Signature{Name: "Worldbuilder", Email: "worldbuilder@localhost", When: time.Now()}
	if cfg, err := config.LoadConfig(config.GlobalScope); err == nil && cfg.User.Name != "" {
		sig.Name = cfg.User.Name
		if cfg.User.Email != "" {
			sig.Email = cfg.User.Email
		}
		return sig
	}
	if u, err := user.Current(); err == nil {
		name, _, _ := strings.Cut(u.Name, ",") // GECOS: "Full Name,room,phone"
		if name == "" {
			name = u.Username
		}
		if name != "" {
			sig.Name = name
			sig.Email = fmt.Sprintf("%s@localhost", u.Username)
		}
	}
	return sig
}
