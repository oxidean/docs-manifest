package pack_test

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oxidean/docs-manifest/internal/pack"
)

func TestPackProducesDeterministicBundle(t *testing.T) {
	tmp := t.TempDir()
	out1 := filepath.Join(tmp, "docs-1.2.3.tar.gz")
	out2 := filepath.Join(tmp, "second.tar.gz")

	if _, err := pack.Pack(pack.Options{Dir: "../../testdata/valid", Version: "1.2.3", Out: out1}); err != nil {
		t.Fatalf("pack failed: %v", err)
	}
	if _, err := pack.Pack(pack.Options{Dir: "../../testdata/valid", Version: "1.2.3", Out: out2}); err != nil {
		t.Fatalf("pack failed: %v", err)
	}

	b1, err := os.ReadFile(out1)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := os.ReadFile(out2)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(sha256sum(b1)) != hex.EncodeToString(sha256sum(b2)) {
		t.Fatal("same input produced different bundles — not deterministic")
	}

	// sidecar format: "<hex>  <basename>"
	side, err := os.ReadFile(out1 + ".sha256")
	if err != nil {
		t.Fatal(err)
	}
	want := hex.EncodeToString(sha256sum(b1)) + "  docs-1.2.3.tar.gz\n"
	if string(side) != want {
		t.Fatalf("sidecar = %q, want %q", side, want)
	}

	// bundle root holds manifest.yaml + the authored tree, sorted
	names := tarNames(t, b1)
	for _, must := range []string{"manifest.yaml", "index.mdx", "getting-started/index.mdx", "language/index.mdx", "images/mark.svg"} {
		if !contains(names, must) {
			t.Fatalf("bundle missing %q (has %v)", must, names)
		}
	}
}

func TestPackVersionGate(t *testing.T) {
	out := filepath.Join(t.TempDir(), "x.tar.gz")
	_, err := pack.Pack(pack.Options{Dir: "../../testdata/version-mismatch", Version: "9.9.9", Out: out})
	if err == nil {
		t.Fatal("expected version_mismatch, got nil")
	}
	var vm *pack.ErrVersionMismatch
	if !errors.As(err, &vm) {
		t.Fatalf("expected ErrVersionMismatch, got %v", err)
	}
	if !strings.Contains(err.Error(), "version_mismatch") {
		t.Fatalf("error lacks named check: %v", err)
	}
}

func TestPackFailsOnInvalidTree(t *testing.T) {
	out := filepath.Join(t.TempDir(), "x.tar.gz")
	if _, err := pack.Pack(pack.Options{Dir: "../../testdata/orphan-page", Version: "1.0.0", Out: out}); err == nil {
		t.Fatal("expected validation failure, got nil")
	}
}

func sha256sum(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

func tarNames(t *testing.T, b []byte) []string {
	t.Helper()
	gz, err := gzip.NewReader(strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
	}
	return names
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
