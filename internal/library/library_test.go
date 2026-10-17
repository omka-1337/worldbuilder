package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCreateListOpen(t *testing.T) {
	lib, err := New(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	info, err := lib.Create(`  Світ: "Перший"  `)
	if err != nil {
		t.Fatal(err)
	}
	// The folder name drops characters Windows forbids; the display name keeps them.
	if info.ID != "Світ- -Перший-" {
		t.Errorf("folder name = %q", info.ID)
	}
	if info.Name != `Світ: "Перший"` || info.Pages != 1 {
		t.Errorf("info = %+v", info)
	}
	if _, err := lib.Create(`Світ: "Перший"`); !errors.Is(err, ErrWorldExist) {
		t.Errorf("duplicate create: %v", err)
	}
	if _, err := lib.Create("  "); !errors.Is(err, ErrBadName) {
		t.Errorf("blank name: %v", err)
	}

	v, err := lib.Open(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.World().Name != `Світ: "Перший"` || len(v.World().Types) == 0 {
		t.Errorf("world = %+v", v.World())
	}
	view, err := v.Get("Головна")
	if err != nil {
		t.Fatal(err)
	}
	// Links carry the world prefix so the UI can route them.
	if !strings.Contains(view.HTML, `href="/w/`) {
		t.Errorf("links lack world prefix: %s", view.HTML)
	}

	worlds, err := lib.List()
	if err != nil || len(worlds) != 1 || worlds[0].ID != info.ID {
		t.Errorf("list = %+v, %v", worlds, err)
	}
}

func TestOpenRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	lib, _ := New(context.Background(), filepath.Join(root, "lib"))
	os.Mkdir(filepath.Join(root, "outside"), 0o755)
	for _, id := range []string{"", ".", "..", "../outside", ".hidden", "a/b", `a\b`, "missing"} {
		if _, err := lib.Open(id); !errors.Is(err, ErrNoWorld) {
			t.Errorf("Open(%q) = %v, want ErrNoWorld", id, err)
		}
	}
}

func TestDocumentsDirFromXDG(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG user dirs are Linux-only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	os.MkdirAll(filepath.Join(home, ".config"), 0o755)

	write := func(s string) {
		os.WriteFile(filepath.Join(home, ".config", "user-dirs.dirs"), []byte(s), 0o644)
	}
	cases := []struct{ config, want string }{
		{`XDG_DOCUMENTS_DIR="$HOME/Документи"`, filepath.Join(home, "Документи")},
		{`XDG_DOCUMENTS_DIR="$HOME/"`, filepath.Join(home, "Documents")},
		{`XDG_MUSIC_DIR="$HOME/Music"`, filepath.Join(home, "Documents")},
	}
	for _, c := range cases {
		write("# comment\n" + c.config + "\n")
		if got, _ := documentsDir(); got != c.want {
			t.Errorf("%s: got %s, want %s", c.config, got, c.want)
		}
	}
}
