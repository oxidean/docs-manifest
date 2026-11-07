# docs-manifest

Versioned product-docs contract tooling for the Oxidean org site
([oxidean.dev](https://github.com/oxidean/oxidean.dev)).

Each product repo authors docs in a curated `docs/site/` subtree behind an
authored `manifest.yaml` (`manifest_version: 1`). This repo provides:

- `schema/manifest.schema.json` — the contract.
- `docs-manifest` CLI — `validate` and `pack` (deterministic
  `docs-<version>.tar.gz` + `.sha256` bundles).
- Reusable GitHub workflows — PR validation and release-asset packing.

The site discovers `docs-*.tar.gz` release assets per product, pins them in
a lock file, and renders every product version under
`/docs/<product>/<version>/`.

## The contract

`docs/site/manifest.yaml`:

```yaml
manifest_version: 1
product:
  slug: oxl                # URL path segment
  name: oxl                # display name
  repo: oxidean/oxl        # owning repo
  tag_pattern: "v*"        # release tags that carry docs bundles
version: "0.4.0"           # must equal the tag minus its non-digit prefix
pages:                     # ordered sidebar tree
  - label: oxl
    link: /                # -> index.md(x)
  - label: Getting started
    link: /getting-started/
  - label: Language
    items:
      - label: Overview
        link: /language/
assets:
  - images/**              # non-page files shipped in the bundle
```

Rules enforced by `validate`:

- Every `link` resolves to a page file: `/a/b/` -> `a/b/index.md(x)` or
  `a/b.md(x)`; `/` -> `index.md(x)`.
- Every `.md`/`.mdx` in the subtree is reachable from `pages` (no orphan
  pages).
- Internal links inside page content resolve within the subtree.
- Every non-page file matches an `assets` glob (no stray files — the
  bundle is byte-deterministic).
- `pack` refuses to build unless `--version == manifest.version`, so a
  release tag always ships docs stamped for that tag.

## CLI

```bash
docs-manifest validate docs/site
docs-manifest pack docs/site --version 0.4.0 --out docs-0.4.0.tar.gz
docs-manifest schema   # print the manifest_version 1 JSON schema
docs-manifest version
```

Failures exit non-zero with named checks: `manifest_missing`, `schema`,
`page_missing`, `orphan_page`, `orphan_file`, `broken_link`,
`version_mismatch`.

## Wiring a product repo

```yaml
# .github/workflows/docs-site.yml (caller)
name: docs site
on:
  pull_request:
    paths: ['docs/site/**']
  push:
    tags: ['v*']          # this repo's tag_pattern
  workflow_dispatch:
    inputs:
      tag: { required: true, type: string }

jobs:
  validate:
    if: github.event_name == 'pull_request'
    uses: oxidean/docs-manifest/.github/workflows/validate.yml@v0.1.0
    # with: { dir: docs/site }

  pack:
    if: github.event_name != 'pull_request'
    permissions:
      contents: write
    uses: oxidean/docs-manifest/.github/workflows/release.yml@v0.1.0
    with:
      tag: ${{ github.ref_name || inputs.tag }}
```

Bootstrap onto an already-published release:
`gh workflow run docs-site.yml -f tag=v0.4.0` — the same reusable
workflow packs and attaches the bundle to that existing release.

`manifest.version` must bump with every matching tag, even when content is
unchanged — that is the freshness gate.

## Development

```bash
go test ./...
go vet ./...
go build ./cmd/docs-manifest
```

Tagging `v*.*.*` runs the goreleaser pipeline (binaries, checksums, cosign
bundles) — same shape as Ox-Agent's release.

Commit subjects use `type(scope): subject`; `scripts/commit-check.sh`
enforces the grammar and the no-AI-attribution rule
(`git config core.hooksPath .githooks` to install the hook).
