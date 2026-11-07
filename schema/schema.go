// Package schema embeds the published JSON schema for manifest_version 1
// so the CLI validates against the same artifact consumers fetch.
package schema

import _ "embed"

// ManifestV1ID is the $id of the manifest_version 1 schema, used as the
// resource key when compiling for validation.
const ManifestV1ID = "https://raw.githubusercontent.com/oxidean/docs-manifest/main/schema/manifest.schema.json"

// ManifestV1 is the JSON schema for manifest_version 1 documents.
//
//go:embed manifest.schema.json
var ManifestV1 []byte
