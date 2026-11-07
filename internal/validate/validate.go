// Package validate checks a docs/site subtree against the manifest
// contract: every declared page exists, every page file is reachable,
// internal links resolve, and non-page files are declared assets.
package validate

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/oxidean/docs-manifest/internal/manifest"
)

// Finding is one named validation failure.
type Finding struct {
	Check  string // stable machine name, e.g. "page_missing"
	Detail string
}

func (f Finding) String() string { return f.Check + ": " + f.Detail }

// Error aggregates findings; Error() renders them one per line.
type Error struct{ Findings []Finding }

func (e *Error) Error() string {
	var b strings.Builder
	for _, f := range e.Findings {
		fmt.Fprintf(&b, "%s\n", f)
	}
	return strings.TrimRight(b.String(), "\n")
}

var pageExts = map[string]bool{".md": true, ".mdx": true}

// Dir validates the docs subtree rooted at dir. Returns nil on success or
// an *Error listing every finding.
func Dir(dir string) (*manifest.Manifest, error) {
	m, err := manifest.Load(dir)
	if err != nil {
		return m, err
	}

	files, err := listFiles(dir)
	if err != nil {
		return m, err
	}

	var findings []Finding
	findings = append(findings, checkPages(dir, m)...)
	findings = append(findings, checkOrphans(dir, m, files)...)
	findings = append(findings, checkLinks(dir, m)...)

	if len(findings) > 0 {
		return m, &Error{Findings: findings}
	}
	return m, nil
}

// listFiles returns every regular file under dir, relative slash paths,
// manifest.yaml excluded.
func listFiles(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		switch {
		case d.IsDir():
			if d.Name() == ".git" || strings.HasPrefix(d.Name(), ".") && rel != "." {
				return fs.SkipDir
			}
		case d.Type().IsRegular():
			if rel != manifest.FileName {
				out = append(out, rel)
			}
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// slugCandidates maps a manifest link ("/a/b/") to the page files that can
// back it: a/b/index.{md,mdx} and a/b.{md,mdx}.
func slugCandidates(link string) []string {
	p := strings.Trim(link, "/")
	var c []string
	if p == "" {
		for _, e := range []string{"md", "mdx"} {
			c = append(c, "index."+e)
		}
		return c
	}
	for _, e := range []string{"md", "mdx"} {
		c = append(c, p+"/index."+e, p+"."+e)
	}
	return c
}

// fileForSlug returns the existing page file for a link, or "".
func fileForSlug(dir, link string) string {
	for _, c := range slugCandidates(link) {
		if st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(c))); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

// checkPages verifies every declared link resolves to a page file.
func checkPages(dir string, m *manifest.Manifest) []Finding {
	var out []Finding
	for _, link := range m.Links() {
		if fileForSlug(dir, link) == "" {
			out = append(out, Finding{
				Check:  "page_missing",
				Detail: fmt.Sprintf("link %q has no page file (expected one of %s)", link, strings.Join(slugCandidates(link), ", ")),
			})
		}
	}
	return out
}

// checkOrphans fails on files that are neither reachable pages nor
// declared assets.
func checkOrphans(dir string, m *manifest.Manifest, files []string) []Finding {
	reachable := map[string]bool{}
	for _, link := range m.Links() {
		if f := fileForSlug(dir, link); f != "" {
			reachable[f] = true
		}
	}

	var out []Finding
	for _, f := range files {
		if reachable[f] {
			continue
		}
		if pageExts[path.Ext(f)] {
			out = append(out, Finding{
				Check:  "orphan_page",
				Detail: fmt.Sprintf("page file %q is not reachable from manifest pages", f),
			})
			continue
		}
		if !assetMatch(m.Assets, f) {
			out = append(out, Finding{
				Check:  "orphan_file",
				Detail: fmt.Sprintf("file %q matches no assets pattern and is not a page", f),
			})
		}
	}
	return out
}

func assetMatch(patterns []string, file string) bool {
	for _, p := range patterns {
		if ok, err := doublestar.Match(p, file); err == nil && ok {
			return true
		}
	}
	return false
}

var (
	mdLink   = regexp.MustCompile(`\]\(\s*([^)\s]+)`)
	htmlAttr = regexp.MustCompile(`(?:href|src)\s*=\s*["']([^"']+)["']`)
	scheme   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
)

// checkLinks verifies internal links inside page content resolve to files
// in the subtree. External links and anchors are skipped.
func checkLinks(dir string, m *manifest.Manifest) []Finding {
	var out []Finding
	for _, link := range m.Links() {
		f := fileForSlug(dir, link)
		if f == "" {
			continue // already reported as page_missing
		}
		out = append(out, checkPageLinks(dir, f)...)
	}
	return out
}

func checkPageLinks(dir, rel string) []Finding {
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		return []Finding{{Check: "broken_link", Detail: fmt.Sprintf("%s: unreadable: %v", rel, err)}}
	}
	var targets []string
	for _, re := range []*regexp.Regexp{mdLink, htmlAttr} {
		for _, m := range re.FindAllSubmatch(raw, -1) {
			targets = append(targets, string(m[1]))
		}
	}

	pageDir := path.Dir(rel)
	seen := map[string]bool{}
	var out []Finding
	for _, t := range targets {
		t = strings.TrimSpace(t)
		switch {
		case t == "", strings.HasPrefix(t, "#"), scheme.MatchString(t), strings.HasPrefix(t, "//"):
			continue
		}
		t = strings.SplitN(strings.SplitN(t, "#", 2)[0], "?", 2)[0]
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true

		var resolved string
		if strings.HasPrefix(t, "/") {
			resolved = resolveRootLink(dir, t)
		} else {
			resolved = resolveRelLink(dir, pageDir, t)
		}
		if resolved == "" {
			out = append(out, Finding{
				Check:  "broken_link",
				Detail: fmt.Sprintf("%s links to %q, which resolves outside or missing from the subtree", rel, t),
			})
		}
	}
	return out
}

// resolveRootLink resolves a "/..." link against the subtree root, first as
// a page slug then as a literal file path.
func resolveRootLink(dir, link string) string {
	if f := fileForSlug(dir, link); f != "" {
		return f
	}
	p := strings.TrimPrefix(link, "/")
	if p == "" || strings.Contains(p, "..") {
		return ""
	}
	if st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err == nil && !st.IsDir() {
		return p
	}
	return ""
}

// resolveRelLink resolves a relative link against the linking page's
// directory. The result must exist inside the subtree.
func resolveRelLink(dir, pageDir, link string) string {
	clean := path.Clean(path.Join(pageDir, link))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return ""
	}
	p := filepath.Join(dir, filepath.FromSlash(clean))
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return clean
	}
	// A directory link resolves if it carries an index page.
	if idx := fileForSlug(dir, "/"+clean+"/"); idx != "" {
		return idx
	}
	return ""
}
