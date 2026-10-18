package vault

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
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
	if rel, ok := v.attachment(embedName(value)); ok {
		return v.fileURL(rel)
	}
	return ""
}

// embedName is the file name in an embed such as ![[map.png|300]].
func embedName(value string) string {
	name := strings.TrimSpace(value)
	name = strings.TrimPrefix(name, "!")
	name = strings.TrimSuffix(strings.TrimPrefix(name, "[["), "]]")
	if i := strings.IndexByte(name, '|'); i >= 0 {
		name = name[:i]
	}
	return strings.TrimSpace(name)
}

// ImageInfo is a picture of the world and the articles showing it.
type ImageInfo struct {
	Name  string   `json:"name"`
	URL   string   `json:"url"`
	Pages []string `json:"pages"`
}

// Images lists the world's pictures for browsing them all: those in
// articles first, grouped by the first article (by title) that shows them
// and in its order, then the ones no article uses, by name.
func (v *Vault) Images() []ImageInfo {
	v.mu.RLock()
	defer v.mu.RUnlock()
	pages := make([]*Page, 0, len(v.idx.pages))
	for _, p := range v.idx.pages {
		pages = append(pages, p)
	}
	c := v.collator()
	sort.Slice(pages, func(i, j int) bool { return c.CompareString(pages[i].Title, pages[j].Title) < 0 })

	out := []ImageInfo{}
	at := map[string]int{} // by path
	for _, p := range pages {
		for _, name := range p.images {
			rel, ok := v.attachment(name)
			if !ok {
				continue
			}
			i, seen := at[rel]
			if !seen {
				i = len(out)
				at[rel] = i
				out = append(out, ImageInfo{Name: path.Base(rel), URL: v.fileURL(rel), Pages: []string{}})
			}
			if !slices.Contains(out[i].Pages, p.Title) {
				out[i].Pages = append(out[i].Pages, p.Title)
			}
		}
	}
	var unused []string
	for _, rel := range v.idx.files {
		if _, seen := at[rel]; !seen && isImage(rel) {
			unused = append(unused, rel)
		}
	}
	c.SortStrings(unused)
	for _, rel := range unused {
		out = append(out, ImageInfo{Name: path.Base(rel), URL: v.fileURL(rel), Pages: []string{}})
	}
	return out
}
