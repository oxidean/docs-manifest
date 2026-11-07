package validate_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/oxidean/docs-manifest/internal/manifest"
	"github.com/oxidean/docs-manifest/internal/validate"
)

func TestDir(t *testing.T) {
	cases := []struct {
		name    string
		dir     string
		wantErr string // substring expected in the error; "" = success
	}{
		{"valid", "../../testdata/valid", ""},
		{"missing manifest", "../../testdata/missing-manifest", "manifest_missing"},
		{"bad schema", "../../testdata/bad-schema", "schema"},
		{"missing page", "../../testdata/missing-page", "page_missing"},
		{"orphan page", "../../testdata/orphan-page", "orphan_page"},
		{"orphan file", "../../testdata/orphan-file", "orphan_file"},
		{"broken link", "../../testdata/broken-link", "broken_link"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validate.Dir(tc.dir)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestMissingManifestIsNamedError(t *testing.T) {
	_, err := manifest.Load("../../testdata/missing-manifest")
	if !errors.Is(err, manifest.ErrMissing) {
		t.Fatalf("expected ErrMissing, got %v", err)
	}
}

// A valid tree where the link checker must distinguish internal from
// external links and skip anchors.
func TestValidHasNoLinkFindings(t *testing.T) {
	_, err := validate.Dir("../../testdata/valid")
	if err != nil {
		t.Fatalf("valid fixture failed: %v", err)
	}
}
