# Architecture

```
 Flutter desktop ──gRPC (local socket)──┐
                                        ▼
 tess CLI ─────────────────────────► Go core ──► LiteLLM / OpenAI-compatible providers
                                        │
                     notebooks on disk ◄┴► MCP servers (agent mode)
```

## Go packages (`internal/`)

| Package | Responsibility |
| --- | --- |
| `config` | Layered settings: app → notebook → work type → prompt → run. Most specific wins; records the source of each value. |
| `secrets` | Secret lookup: environment variables now, OS keychain later. Never written to disk by Tessivo. |
| `llm` | OpenAI-compatible client: models list, chat completions, SSE streaming, JSON-schema `response_format`, reasoning effort. |
| `llm/fallback` logic (in `llm`) | Alias → ordered model chain; retry with backoff; falls back on timeout, rate limit, 5xx, context overflow; reports which model answered. |
| `worktype` | Built-in work types (Analyze, Summarize, Document, Report): starter prompt, suggested context, output schema. |
| `schema` | Validates model output against a JSON-schema subset. |
| `extract` | Context extractors by type (text, Markdown, CSV, JSON, code base; DOCX/XLSX/PPTX next). |
| `notebook` | Notebook folder layout, manifest, context items, prompts, run records, outputs. |
| `executor` | `Executor` interface; `pipeline` mode now, `agent` mode (Eino) next. |
| `cli` | `tess` commands; `cmd/tess` only calls it. |

## Notebook folder

```
<notebook>/
  notebook.json      manifest: id, name, work type, settings overrides, context items
  prompts/           one JSON file per prompt
  context/           copied files (code bases and folders are referenced in place)
  runs/<run-id>.json run record: inputs, effective settings, model used, tokens, timing, result
  outputs/<id>.json  validated structured output (+ rendered .md)
```

SQLite (FTS, retrieval) arrives with the desktop MVP; the folder stays the source of truth so
notebooks remain portable and diff-able.
