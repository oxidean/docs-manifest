// Package manifest loads and schema-validates docs/site/manifest.yaml
// documents (manifest_version 1).
package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/oxidean/docs-manifest/schema"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// FileName is the required manifest file name inside a docs subtree.
const FileName = "manifest.yaml"

// Manifest is a manifest_version 1 document.
type Manifest struct {
	ManifestVersion int      `yaml:"manifest_version" json:"manifest_version"`
	Product         Product  `yaml:"product" json:"product"`
	Version         string   `yaml:"version" json:"version"`
	Pages           []Page   `yaml:"pages" json:"pages"`
	Assets          []string `yaml:"assets" json:"assets"`
}

// Product identifies the owning product and release line.
type Product struct {
	Slug       string `yaml:"slug" json:"slug"`
	Name       string `yaml:"name" json:"name"`
	Repo       string `yaml:"repo" json:"repo"`
	TagPattern string `yaml:"tag_pattern" json:"tag_pattern"`
}

// Page is one sidebar item: a label plus exactly one of Link or Items.
type Page struct {
	Label string `yaml:"label" json:"label"`
	Link  string `yaml:"link,omitempty" json:"link,omitempty"`
	Items []Page `yaml:"items,omitempty" json:"items,omitempty"`
}

// Links flattens every link in the page tree, in declaration order.
func (m *Manifest) Links() []string {
	var out []string
	var walk func(pages []Page)
	walk = func(pages []Page) {
		for _, p := range pages {
			if p.Link != "" {
				out = append(out, p.Link)
			}
			walk(p.Items)
		}
	}
	walk(m.Pages)
	return out
}

// ErrMissing is returned when the subtree has no manifest.yaml.
var ErrMissing = errors.New("manifest_missing")

// Load reads <dir>/manifest.yaml and validates it against the embedded
// manifest_version 1 JSON schema. Returns ErrMissing when absent.
func Load(dir string) (*Manifest, error) {
	path := filepath.Join(dir, FileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: no %s in %s", ErrMissing, FileName, dir)
		}
		return nil, err
	}

	var doc any
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(false)
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("schema: %s is not valid YAML: %w", FileName, err)
	}
	// A second decode must hit EOF — reject multi-document streams.
	if err := dec.Decode(&doc); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("schema: %s must contain exactly one YAML document", FileName)
	}

	doc = normalize(doc)
	if err := schemaValidate(doc); err != nil {
		return nil, err
	}

	var m Manifest
	enc, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(enc, &m); err != nil {
		return nil, fmt.Errorf("schema: cannot decode %s: %w", FileName, err)
	}
	return &m, nil
}

// normalize converts yaml.v3's map[any]any into JSON-compatible
// map[string]any recursively.
func normalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			t[k] = normalize(val)
		}
		return t
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[fmt.Sprint(k)] = normalize(val)
		}
		return m
	case []any:
		for i, val := range t {
			t[i] = normalize(val)
		}
		return t
	default:
		return v
	}
}

func schemaValidate(doc any) error {
	c := jsonschema.NewCompiler()
	sch, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema.ManifestV1))
	if err != nil {
		return err
	}
	if err := c.AddResource(schema.ManifestV1ID, sch); err != nil {
		return err
	}
	compiled, err := c.Compile(schema.ManifestV1ID)
	if err != nil {
		return err
	}
	if err := compiled.Validate(doc); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	return nil
}
