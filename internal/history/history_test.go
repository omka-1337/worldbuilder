package history

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no global Git identity from the machine
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	write(t, root, "world.yaml", "name: Test\n")
	write(t, root, "Головна.md", "Привіт.\n")
	write(t, root, "Персонажі/Аедрін.md", "Маг.\n")
	write(t, root, ".trash/Старе.md", "Видалене.\n")

	// The first open creates the repository with everything in it, except
	// the trash.
	r, err := Open(root, "Перша версія")
	if err != nil {
		t.Fatal(err)
	}
	log, err := r.Log(10)
	if err != nil || len(log) != 1 || log[0].Message != "Перша версія" || log[0].Author == "" {
		t.Fatalf("log after init = %+v, %v", log, err)
	}
	if ch, _ := r.Changes(); len(ch) != 0 {
		t.Errorf("changes right after init = %+v", ch)
	}
	if _, err := r.Commit("Нічого"); err != ErrNoChanges {
		t.Errorf("empty commit: %v", err)
	}

	// Edits, new files and deletions all show up and are saved together.
	write(t, root, "Головна.md", "Привіт, світе.\n")
	write(t, root, "Вальмор.md", "Місто.\n")
	if err := os.Remove(filepath.Join(root, "Персонажі", "Аедрін.md")); err != nil {
		t.Fatal(err)
	}
	write(t, root, ".trash/Аедрін.md", "Маг.\n")
	ch, err := r.Changes()
	want := []Change{{"Вальмор.md", "added"}, {"Головна.md", "modified"}, {"Персонажі/Аедрін.md", "deleted"}}
	if err != nil || len(ch) != len(want) {
		t.Fatalf("changes = %+v, %v", ch, err)
	}
	for i := range want {
		if ch[i] != want[i] {
			t.Errorf("change %d = %+v, want %+v", i, ch[i], want[i])
		}
	}
	if got, ok, err := r.Committed("Головна.md"); err != nil || !ok || string(got) != "Привіт.\n" {
		t.Errorf("committed = %q, %v, %v", got, ok, err)
	}
	if _, ok, err := r.Committed("Вальмор.md"); err != nil || ok {
		t.Errorf("an uncommitted file: %v, %v", ok, err)
	}
	v, err := r.Commit("  Правки  ")
	if err != nil || v.Message != "Правки" {
		t.Fatalf("commit = %+v, %v", v, err)
	}
	if ch, _ := r.Changes(); len(ch) != 0 {
		t.Errorf("changes after commit = %+v", ch)
	}
	if log, _ := r.Log(10); len(log) != 2 || log[0].Hash != v.Hash {
		t.Errorf("log = %+v", log)
	}

	// Reopening finds the same repository instead of starting over.
	r2, err := Open(root, "Знову")
	if err != nil {
		t.Fatal(err)
	}
	if log, _ := r2.Log(10); len(log) != 2 {
		t.Errorf("log after reopen = %+v", log)
	}
}
