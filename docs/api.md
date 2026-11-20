# Local API (v1)

The desktop app starts the core itself (`tess serve --exit-with-parent`, reusing a healthy running
core if there is one); you can also run it by hand. The core (`tess serve`) listens on a Unix socket (macOS/Linux) or a loopback port (Windows) and
writes its address and token to `endpoint.json` (0600) in the config dir's `run/` folder.
Every request except `GET /v1/health` needs `Authorization: Bearer <token>`. Bodies are JSON
(`Content-Type: application/json`, ≤ 1 MiB, unknown fields rejected). Errors are
`{"error": "..."}` with 400 (invalid), 401, 403 (browser request), 404, 421 (bad host),
422 (output failed validation), 429 (too many runs) or 500.

| Method and path | Body | Response |
| --- | --- | --- |
| `GET /v1/health` | — | `{"ok": true, "version": "…"}` |
| `GET /v1/worktypes` | — | `{"workTypes": [{id, name, question, starterPrompt, suggestedContext, schema}]}` |
| `GET /v1/notebooks` | — | `{"notebooks": [Notebook]}` (no context stats) |
| `POST /v1/notebooks` | `{name, workType}` | `201 Notebook` |
| `GET /v1/notebooks/{id}` | — | `Notebook` with context stats |
| `POST /v1/notebooks/{id}/context` | `{path}` (absolute) | `Notebook` |
| `PATCH /v1/notebooks/{id}/context/{contextId}` | `{enabled}` | `Notebook` |
| `POST /v1/notebooks/{id}/runs` | `{prompt?, promptName?, vars?, overrides?, save?}` | SSE stream |
| `GET /v1/outputs?notebook=&workType=` | — | `{"outputs": [Output]}` (no data) |
| `GET /v1/notebooks/{id}/outputs/{outputId}` | — | `Output` with `data` and `markdown` |
| `GET /v1/settings` | — | config (key references only) |
| `GET /v1/models?provider=` | — | `{"models": {"provider": ["model", …]}}` |

`Notebook`: `{id, name, workType, settings, createdAt, outputs, context: [{id, label, path,
inPlace, enabled, addedAt, sources, estimatedTokens}]}`.

`Output`: `{id, runId, workType, title, createdAt, notebookId, notebookName, data?, markdown?}`.

## Run stream

```
event: status
data: {"text":"generating Summarize"}

event: delta
data: {"text":"{\"title\":..."}

event: finished
data: {"runId","outputId","valid","modelUsed","strategy","totalTokens","attempts":[{target,trigger,error}],"data","markdown"}
```

A failed run ends with `event: error` and `{"status": 422, "error": "…"}`. Closing the
connection cancels the run.
