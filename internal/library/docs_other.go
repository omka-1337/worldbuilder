//go:build !windows

package library

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// documentsDir follows the XDG user-dirs config on Linux, where the folder
// is often localized (e.g. ~/Документи), and uses ~/Documents elsewhere.
func documentsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	fallback := filepath.Join(home, "Documents")
	if runtime.GOOS == "darwin" {
		return fallback, nil
	}

	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	f, err := os.Open(filepath.Join(config, "user-dirs.dirs"))
	if err != nil {
		return fallback, nil
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		value, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "XDG_DOCUMENTS_DIR=")
		if !ok {
			continue
		}
		dir := strings.Trim(value, `"`)
		dir = strings.Replace(dir, "$HOME", home, 1)
		if !filepath.IsAbs(dir) || filepath.Clean(dir) == filepath.Clean(home) {
			return fallback, nil // a value of "$HOME/" means the folder is disabled
		}
		return dir, nil
	}
	return fallback, nil
}
