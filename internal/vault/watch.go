package vault

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watch reloads the vault when files change on disk, so edits made in
// another editor show up. It blocks until ctx is cancelled.
func (v *Vault) Watch(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	addTree := func(dir string) {
		_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			if p != v.root && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			if err := w.Add(p); err != nil {
				log.Printf("watch %s: %v", p, err)
			}
			return nil
		})
	}
	addTree(v.root)

	// Editors write files in bursts (temp file, rename, chmod), so wait for
	// things to settle before rescanning.
	const settle = 250 * time.Millisecond
	timer := time.NewTimer(settle)
	timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-w.Events:
			if strings.HasPrefix(filepath.Base(ev.Name), ".") {
				continue
			}
			if ev.Has(fsnotify.Create) {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					addTree(ev.Name)
				}
			}
			timer.Reset(settle)
		case err := <-w.Errors:
			log.Printf("watch: %v", err)
		case <-timer.C:
			if err := v.Reload(); err != nil {
				log.Printf("reload: %v", err)
			}
		}
	}
}
