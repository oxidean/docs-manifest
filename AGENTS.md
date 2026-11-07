<!-- ox-agent:begin -->
## Ox-Agent toolkit

This repository is managed by `ox-agent` — a repository-local AI-SDLC
runtime. Project state, skills, and contracts live under `.ox-agent/`.

- `ox-agent doctor` validates the setup; `ox-agent start <title>` opens work items.
- Work items carry state in `index.json`; drive them with
  `ox-agent transition|archive|unblock|status`.
- Human input is durable, never synchronous. When you need an answer or a
  gated approval, file `ox-agent input request` and END THE TURN —
  chat prompts expire before humans see them. Poll with
  `ox-agent input pending` / `input wait --timeout <budget>`;
  answers arrive via `input respond <id> --by <name> --answer "..."`
  (approvals: `--approve|--reject`). Escalated runs and gate hits file
  these requests automatically; a skill asks a
  question with `next_action.type: await_input` — the run stays
  dispatchable and the answer lands in the next packet under `context.inputs`.
- Skills are contracts, not free-form prompts: `.ox-agent/skills/<name>/SKILL.md`
  plus input/output `schemas/`.
- Compile the bounded packet (`ox-agent context compile <wi> --skill <s>`)
  or dispatch via `ox-agent run <wi>`; results land in `runtime/runs/`
  and are schema-validated on collection.
- Materialized skills also resolve under `.agents/skills/` for harnesses
  that scan that path.

Do not edit `.ox-agent/skills`, `schemas`, `workflows`, `pipelines`,
`lifecycle`, or `integrations` in place — they are materialized assets
tracked by `assets.lock`. Add extension skills as new directories instead.
<!-- ox-agent:end -->

## docs-manifest — org docs contract tooling

Validator + packer for the `docs/site/manifest.yaml` contract
(`manifest_version: 1`) consumed by oxidean.dev. Org-internal tooling —
deliberately **not** part of the ox-agent binary or its skill set.

### Layout

| Path | Purpose |
| ---- | ------- |
| `schema/manifest.schema.json` | published JSON schema (embedded into the binary via `schema/schema.go`) |
| `cmd/docs-manifest/` | `validate` / `pack` / `schema` / `version` CLI (stdlib `flag`, no Cobra) |
| `internal/{manifest,validate,pack}/` | loading, semantic checks, deterministic tarball |
| `testdata/` | `valid` + one fixture per named failure check |
| `.github/workflows/validate.yml` | reusable `workflow_call` — PR validation for product repos |
| `.github/workflows/release.yml` | reusable `workflow_call` — pack + attach `docs-<ver>.tar.gz` to a release, then fail-soft `workflow_dispatch` to the site repo |
| `.github/workflows/tag-release.yml` | this repo's own goreleaser pipeline (`v*.*.*` tags) |

### Gates

`go test ./...`, `go vet ./...`, `go build ./cmd/docs-manifest`; commit
grammar + no-AI-attribution via `scripts/commit-check.sh`
(`git config core.hooksPath .githooks`) and the `commit-lint` CI job.

### Contract notes for editors

- `manifest_version: 1` is forever: additive-only changes; anything
  breaking ships as `manifest_version: 2`.
- `pack` is deterministic (sorted entries, epoch mtimes, uid/gid 0,
  nameless gzip header) — keep it that way; tests assert byte equality.
- Reusable workflows install the pinned CLI from **this** repo's releases —
  bump `cli-version` defaults only after a real tag exists.
