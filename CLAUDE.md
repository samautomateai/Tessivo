# Tessivo — rules for agents and contributors

Tessivo is a native desktop + CLI app that analyzes, summarizes, documents and reports using
self-hosted OpenAI-compatible models (LiteLLM). Go core and CLI (`tess`), Flutter desktop UI,
Eino for agent mode, MCP for tools. Spec: see `docs/README.md`.

## Non-negotiable quality gates (`task check`)

- `task lengths`: lines ≤ 120 characters; files ≤ 500 lines (tests ≤ 800). Split files, don't wrap hacks.
- `task lint`: golangci-lint v2 with gosec, lll, funlen (80 lines), gocyclo (15), wrapcheck, errorlint.
  `//nolint` needs a specific linter and an explanation.
- `task cover`: total Go coverage ≥ 80%. Flutter: `task desktop:check`, also ≥ 80%.
- `task vuln`: govulncheck clean.
- Race detector on in tests.

## Engineering rules

- **Latest stable toolchains**: Go version pinned in `go.mod` (`toolchain` line); Flutter stable channel.
- **Standard library first.** Add a dependency only when building it ourselves makes no sense
  (protocols, OS integration, databases, agent framework). Record every direct dependency and its
  reason in `docs/adr/0003-dependency-policy.md` in the same change.
- **Core stays engine-agnostic**: Eino, SQLite, gRPC types never leak out of their adapter packages.
- **Secrets** never in config, notebooks, templates, logs or test fixtures.
- **Every change updates docs**: the relevant `docs/` page, `docs/checklist.md` (story status) and an
  ADR when a decision changes. Not optional.
- Commit and push small, working increments; CI must stay green.

## Layout

- `cmd/tess` — CLI entry point (thin).
- `internal/` — core packages (see `docs/architecture.md`).
- `tools/` — repo tooling (length and coverage gates).
- `api/proto` — gRPC contract between the desktop UI and the core.
- `desktop/` — Flutter app.
