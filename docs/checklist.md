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
| gRPC contract | — | ☐ |
| Flutter desktop shell (theme, navigation, screen stubs) | — | ☐ |
| Extractors: DOCX, XLSX, PPTX (zip + XML, stdlib) | SRC-1 | ☐ |
| OS keychain secrets (go-keyring) | SET-1 | ☐ |
| SQLite index (FTS) alongside the folder store | NB-3 | ☐ |
| Agent executor on Eino + MCP tools | WT/E11 | ☐ |

## CLI reference (current)

`tess new`, `tess context add|list|enable|disable`, `tess run` (notebook or piped stdin, `--prompt`,
`--prompt-name`, `--var`, `--model`, `--thinking`, `--json`, `--out`, `--no-save`), `tess outputs`,
`tess models`, `tess worktypes`, `tess config init|show|path`, `tess version`.
Exit codes: 0 ok, 1 error, 2 usage, 3 output failed schema validation.
