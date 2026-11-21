# Tessivo — rules for agents and contributors

Tessivo is a native desktop + CLI app that analyzes, summarizes, documents and reports using
self-hosted OpenAI-compatible models (LiteLLM). Go core and CLI (`tess`), Flutter desktop UI,
Eino for agent mode, MCP for tools. Spec: see `docs/README.md`.

## Non-negotiable quality gates (`task check` and `task ci` include the desktop gates when Flutter is installed)

- `task fmt:check`: gofmt -s, gofumpt (extra: group-params, clothe-returns) and goimports must report no diff.
  Fix with `task fmt`.
- `task lengths`: lines ≤ 120 characters; files ≤ 500 lines (tests ≤ 800). Split files, don't wrap hacks.
- `task text`: text files are UTF-8 without BOM, LF line endings (CRLF only for `.bat`/`.cmd`), end with a newline,
  no trailing whitespace. `.gitattributes` makes git check out LF on every OS; fix with `task text:fix`.
- `task lint`: golangci-lint v2 with gosec, lll, funlen (80 lines), gocyclo (15), wrapcheck, errorlint.
  `//nolint` needs a specific linter and an explanation.
- `task desktop:check` (Flutter): `dart format` (120 columns), `flutter analyze --fatal-infos` with flutter_lints,
  strict casts/inference/raw types and extra rules (`desktop/analysis_options.yaml`; leaked streams, sinks and futures
  are errors), tests with coverage ≥ 80% (`covercheck -format lcov`), `pub get --enforce-lockfile`. CI runs it on
  Windows, macOS and Linux. Dart has no free complexity linter, so function size is a code-review matter there.
- `task cover`: total Go coverage ≥ 80%.
- `task vuln`: govulncheck (Go) and osv-scanner (`desktop/pubspec.lock`) clean.
- Race detector on in tests (needs cgo + a C compiler; without one `task test` warns, CI still enforces it).
- `task audit`: `go mod verify`, govulncheck, gosec. `task fuzz` (tools/fuzzall) for parsers; `task bench` for hot paths.
- Set up once: `task install` (or `go run ./tools/devtools install` before Task exists) installs Task,
  golangci-lint v2, govulncheck, gosec, osv-scanner and the git hooks (format, lengths, text, lint on commit; Dart
  format and analyze when Dart files are staged).
  `task doctor` diagnoses and repairs; `task update` moves go.mod to the latest Go toolchain and upgrades every tool.

## Security and performance rules

- Treat every input as hostile: files, archives, model output, MCP results, HTTP responses.
  Bound sizes, counts and time; never trust a declared length.
- No secrets in argv, logs, errors or audit entries; redact before writing.
- Files Tessivo writes are 0600, directories 0700/0750; writes are atomic.
- Parsers get fuzz tests; hot paths get benchmarks; concurrency is bounded.

## Engineering rules

- **OS independent by default**: everything we build — product code, tests, tooling, Taskfile, hooks, CI — must
  work the same on Windows, macOS and Linux. Use Go and the standard library (`filepath`, `os.UserHomeDir`,
  `os.TempDir`, `os.CreateTemp`), not shell tools (`sh`, `grep`, `sed`, `awk`, `true`) or hard-coded paths
  (`/proc`, `/tmp`, `/home`, `/`-joined paths). Repo tooling is a Go program in `tools/`, not a shell script.
  Tests use `internal/testutil`: `AssertPerm` (no Unix permission bits on Windows), `Unwritable` (instead of
  `/proc` or chmod), `HelperCommand` (the test binary instead of POSIX tools).
- **Anything OS-specific needs explicit approval first**: a `runtime.GOOS` branch, build tags, an OS tool or API
  (keychain, notifications, sockets, file permissions), or anything that runs on only some OSes. Stop, explain
  why it is needed and the portable alternatives, and wait for the user's approval before writing it. When
  approved: keep it behind an interface in its own file (`*_windows.go`, `*_unix.go`), give every OS a working
  path or a clear "unsupported" error, test each branch, and note the decision in the relevant doc or ADR.
- **Latest stable toolchains**: Go version pinned in `go.mod` (`toolchain` line); Flutter stable channel.
- **Standard library first.** Add a dependency only when building it ourselves makes no sense
  (protocols, OS integration, databases, agent framework). Record every direct dependency and its
  reason in `docs/adr/0003-dependency-policy.md` in the same change.
- **Core stays engine-agnostic**: Eino, SQLite and transport types never leak out of their adapter packages;
  business logic lives in `internal/service`.
- **Secrets** never in config, notebooks, templates, logs or test fixtures.
- **Every change updates docs**: the relevant `docs/` page, `docs/checklist.md` (story status) and an
  ADR when a decision changes. Not optional.
- Commit and push small, working increments; CI must stay green.

## Layout

- `cmd/tess` — CLI entry point (thin).
- `internal/` — core packages (see `docs/architecture.md`).
- `tools/` — repo tooling in Go (dev setup, length, text, coverage gates, fuzz runner).
- `internal/testutil` — OS-independent test helpers (imported only from tests).
- `docs/api.md` — local JSON API contract between the desktop UI and the core (ADR 0004).
- `desktop/` — Flutter app.
