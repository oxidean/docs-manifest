// Package pack re-validates a docs subtree and emits the deterministic
// docs-<version>.tar.gz release bundle plus its sha256 sidecar.
package pack

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oxidean/docs-manifest/internal/validate"
)

// ErrVersionMismatch is returned when --version != manifest.version.
type ErrVersionMismatch struct{ Manifest, Flag string }

func (e *ErrVersionMismatch) Error() string {
	return fmt.Sprintf("version_mismatch: manifest declares %q but --version is %q (bump manifest.version with the tag)", e.Manifest, e.Flag)
}

// Options controls Pack.
type Options struct {
	Dir     string // docs subtree root (contains manifest.yaml)
	Version string // expected manifest.version (tag minus prefix)
	Out     string // output tarball path; empty -> docs-<version>.tar.gz in cwd
}

// Pack validates the subtree, asserts the version gate, and writes the
// tarball + .sha256 sidecar. Returns the tarball path.
func Pack(o Options) (string, error) {
	m, err := validate.Dir(o.Dir)
	if err != nil {
		return "", err
	}
	if m.Version != o.Version {
		return "", &ErrVersionMismatch{Manifest: m.Version, Flag: o.Version}
	}

	out := o.Out
	if out == "" {
		out = fmt.Sprintf("docs-%s.tar.gz", o.Version)
	}

	files, err := collect(o.Dir)
	if err != nil {
		return "", err
	}
	if err := writeTarGz(o.Dir, files, out); err != nil {
		return "", err
	}
	if err := writeSHA256(out); err != nil {
		return "", err
	}
	return out, nil
}

// collect lists every file under dir including manifest.yaml, slash-sorted.
func collect(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if rel != "." && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// writeTarGz writes the deterministic bundle: sorted entries, zeroed
// mtimes, uid/gid 0, normalized modes, nameless gzip header.
func writeTarGz(dir string, files []string, out string) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	gz, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		f.Close()
		return err
	}
	gz.Name = ""
	gz.ModTime = time.Time{}
	tw := tar.NewWriter(gz)

	zero := time.Time{}
	for _, rel := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		b, err := os.ReadFile(full)
		if err != nil {
			tw.Close()
			gz.Close()
			f.Close()
			return err
		}
		hdr := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     rel,
			Mode:     0o644,
			Size:     int64(len(b)),
			ModTime:  zero,
			Uid:      0,
			Gid:      0,
			Uname:    "",
			Gname:    "",
			Format:   tar.FormatGNU,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			tw.Close()
			gz.Close()
			f.Close()
			return err
		}
		if _, err := tw.Write(b); err != nil {
			tw.Close()
			gz.Close()
			f.Close()
			return err
		}
	}
	if err := tw.Close(); err != nil {
		gz.Close()
		f.Close()
		return err
	}
	if err := gz.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// writeSHA256 writes <path>.sha256 in `sha256sum` format.
func writeSHA256(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	line := fmt.Sprintf("%s  %s\n", hex.EncodeToString(h.Sum(nil)), filepath.Base(path))
	return os.WriteFile(path+".sha256", []byte(line), 0o644)
}
