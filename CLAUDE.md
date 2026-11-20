# Tessivo — rules for agents and contributors

Tessivo is a native desktop + CLI app that analyzes, summarizes, documents and reports using
self-hosted OpenAI-compatible models (LiteLLM). Go core and CLI (`tess`), Flutter desktop UI,
Eino for agent mode, MCP for tools. Spec: see `docs/README.md`.

## Non-negotiable quality gates (`task check`)

- `task fmt:check`: gofmt -s, gofumpt (extra rules) and goimports must report no diff. Fix with `task fmt`.
- `task lengths`: lines ≤ 120 characters; files ≤ 500 lines (tests ≤ 800). Split files, don't wrap hacks.
- `task lint`: golangci-lint v2 with gosec, lll, funlen (80 lines), gocyclo (15), wrapcheck, errorlint.
  `//nolint` needs a specific linter and an explanation.
- `task cover`: total Go coverage ≥ 80%. Flutter: `task desktop:check`, also ≥ 80%.
- `task vuln`: govulncheck clean.
- Race detector on in tests.
- `task audit`: `go mod verify`, govulncheck, gosec. `task fuzz` for parsers; `task bench` for hot paths.
- Install hooks once: `task hooks:install` (format, lengths, lint on commit).

## Security and performance rules

- Treat every input as hostile: files, archives, model output, MCP results, HTTP responses.
  Bound sizes, counts and time; never trust a declared length.
- No secrets in argv, logs, errors or audit entries; redact before writing.
- Files Tessivo writes are 0600, directories 0700/0750; writes are atomic.
- Parsers get fuzz tests; hot paths get benchmarks; concurrency is bounded.

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
