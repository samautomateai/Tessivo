# Delivery checklist

Status: ☐ not started · ◐ in progress · ☑ done. Story IDs refer to the product spec.

## M0/M1 foundations

| Item | Stories | Status |
| --- | --- | --- |
| Repo, Taskfile, quality gates (lint, gosec, lengths, coverage ≥ 80%, govulncheck, CodeQL, Dependabot) | NFR Quality | ☑ |
| Layered settings with source tracking | SET-5 | ☐☑ |
| Secrets from environment (keychain next) | SET-1 | ☐☑ |
| OpenAI-compatible client: models, chat, streaming, structured output, reasoning effort | SET-2, SET-3 | ☐☑ |
| Aliases and fallback chains with retry/backoff | SET-4 | ☐☑ |
| Built-in work types with output schemas + validation and retry | WT-1, WT-2, WT-3 | ☐☑ |
| Extractors: text, Markdown, CSV, JSON, code base | SRC-1, SRC-3 | ☐☑ |
| Notebook folder store, run records, outputs | NB-1, RUN-2, REP-1 | ☐☑ |
| Pipeline executor | RUN-1 | ☐☑ |
| `tess` CLI: init, notebook, context, run, models | CLI-1, CLI-2, CLI-3 | ☐☑ |
| Tamper-evident audit log of runs, model attempts and outputs; `tess audit verify/show` | NFR Security | ☑ |
| Formatting gate (gofmt -s, gofumpt, goimports), `task audit`, fuzz and bench tasks | NFR Quality | ☑ |
| Hardened HTTP: TLS 1.2+, no redirects, bounded timeouts, idle-stream timeout; plaintext to remote hosts refused | NFR Security | ☑ |
| Parallel code-base extraction (bounded pool); benchmarks; fuzz targets for schema, CSV/JSON, Office, audit | NFR Performance | ☑ |
| Desktop ↔ core contract: local JSON API + SSE (ADR 0004, docs/api.md) | — | ☑ |
| Flutter desktop shell: theme, sidebar, workspace, new notebook, library, settings on a demo core | — | ☑ |
| Shared service layer; `tess serve` local API (token, origin/host checks, limits, SSE runs) | — | ☑ |
| Flutter client for the local API (Unix socket/loopback, token, SSE, endpoint discovery, demo fallback) | — | ☑ |
| Desktop reuses a healthy core or starts `tess serve --exit-with-parent` (bundled binary or PATH), waits for health, stops it on exit; core exits if the app dies | — | ☑ |
| Extractors: DOCX (paragraphs, tables), XLSX (all sheets, computed stats), PPTX (slides + notes); zip-bomb, entry, size and XML-token limits; fuzzed | SRC-1 | ☑ |
| OS keychain secrets, built in-house: `tess secret set/delete/check`; env then keychain lookup | SET-1 | ☑ |
| SQLite index (FTS) alongside the folder store | NB-3 | ☐ |
| Agent executor on Eino + MCP tools | WT/E11 | ☐ |

## CLI reference (current)

`tess new`, `tess context add|list|enable|disable`, `tess run` (notebook or piped stdin, `--prompt`,
`--prompt-name`, `--var`, `--model`, `--thinking`, `--json`, `--out`, `--no-save`), `tess outputs`,
`tess models`, `tess worktypes`, `tess config init|show|path`, `tess version`.
Exit codes: 0 ok, 1 error, 2 usage, 3 output failed schema validation.
