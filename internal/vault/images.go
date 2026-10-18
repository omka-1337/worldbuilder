package vault

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Infobox keys every article has, whatever its type.
const (
	ImageKey   = "image"   // the picture at the top of the infobox: ![[file.png]]
	CaptionKey = "caption" // the line under it
)

// imageTypes are the formats accepted for upload, by sniffed content type.
// SVG is left out on purpose: it can carry scripts.
var imageTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// SaveImage stores an uploaded picture for the article titled title. It goes
// into the article's own folder, so it travels with the article when that is
// moved or renamed, and is named after the article. Returns the file name to
// embed as ![[name]] and its URL.
func (v *Vault) SaveImage(title string, data []byte) (name, url string, err error) {
	ext, ok := imageTypes[http.DetectContentType(data)]
	if !ok {
		return "", "", &InvalidError{"unsupported image format"}
	}
	base := SafeName(title)
	if base == "" {
		return "", "", &InvalidError{"an article needs a title"}
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	dir := stem(fileNameFor(title)) // a new article will be created at the top level
	if p, ok := v.lookup(title); ok {
		dir = stem(p.Path)
	}
	// Embeds are looked up by file name across the whole world, so the name
	// must be unique there, not just in this folder.
	name = base + ext
	for i := 2; ; i++ {
		_, taken := v.idx.files[strings.ToLower(name)]
		if !taken && !fileExists(filepath.Join(v.root, filepath.FromSlash(path.Join(dir, name)))) {
			break
		}
		name = fmt.Sprintf("%s-%d%s", base, i, ext)
	}

	rel := path.Join(dir, name)
	abs := filepath.Join(v.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", "", err
	}
	if err := writeAtomic(abs, data); err != nil {
		return "", "", err
	}
	v.idx.files[strings.ToLower(name)] = rel
	return name, v.fileURL(rel), nil
}

// imageURL resolves an infobox image value like ![[name.png]] to its URL,
// or "" when it names no known file. Callers hold v.mu.
func (v *Vault) imageURL(value string) string {
	name := strings.TrimSpace(value)
	name = strings.TrimPrefix(name, "!")
	name = strings.TrimSuffix(strings.TrimPrefix(name, "[["), "]]")
	if i := strings.IndexByte(name, '|'); i >= 0 {
		name = name[:i]
	}
	if rel, ok := v.attachment(strings.TrimSpace(name)); ok {
		return v.fileURL(rel)
	}
	return ""
}
