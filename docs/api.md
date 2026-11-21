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
| `POST /v1/notebooks` | `{name, workType}` or `{templateId, name?}` | `201 Notebook` |
| `GET /v1/notebooks/{id}` | — | `Notebook` with context stats |
| `POST /v1/notebooks/{id}/context` | `{path}` (absolute) | `Notebook` |
| `PATCH /v1/notebooks/{id}/context/{contextId}` | `{enabled}` | `Notebook` |
| `POST /v1/notebooks/{id}/runs` | `{prompt?, promptName?, vars?, overrides?, save?, toolRules?}` | SSE stream |
| `GET /v1/outputs?notebook=&workType=` | — | `{"outputs": [Output]}` (no data) |
| `GET /v1/notebooks/{id}/outputs/{outputId}` | — | `Output` with `data` and `markdown` |
| `GET /v1/settings` | — | config (key references only) |
| `GET /v1/models?provider=` | — | `{"models": {"provider": ["model", …]}}` |
| `GET /v1/notebooks/{id}/schedules` | — | `{"schedules": [Schedule]}` |
| `POST /v1/notebooks/{id}/schedules` | `{cron, prompt?, promptName?, vars?, missed?, enabled?}` | `201 Schedule` |
| `PATCH /v1/notebooks/{id}/schedules/{scheduleId}` | `{enabled}` | `Schedule` |
| `DELETE /v1/notebooks/{id}/schedules/{scheduleId}` | — | `204` |
| `PUT /v1/notebooks/{id}/memory` | `{memory: [string]}` (≤ 50 items, ≤ 1000 chars) | `Notebook` |
| `GET /v1/templates?workType=` | — | `{"templates": [Template]}` |
| `GET /v1/templates/{templateId}` | — | `Template` |
| `POST /v1/templates` | `{fromNotebook, name, id?, description?}` or `{extend, name, id?}` | `201 Template` |
| `DELETE /v1/templates/{templateId}` | — | `204` (user templates only) |
| `GET /v1/events` | — | SSE stream of scheduler events (keepalive comments every 25 s) |
| `GET /v1/chats?q=&archived=` | — | `{"sessions": [ChatSummary]}` (pinned first, then newest) |
| `POST /v1/chats` | `{title?, model?, thinkingEffort?}` | `201 ChatSession` |
| `GET /v1/chats/{chatId}` | — | `ChatSession` with messages |
| `PATCH /v1/chats/{chatId}` | `{title?, pinned?, archived?, model?, thinkingEffort?}` | `ChatSession` |
| `DELETE /v1/chats/{chatId}` | — | `204` |
| `POST /v1/chats/{chatId}/messages` | `{text, attachments?: [{kind, ref, label?}]}` | SSE stream (see below) |
| `GET /v1/chats/{chatId}/repeatable` | — | `RepeatablePlan` |
| `POST /v1/chats/{chatId}/notebook` | `{name?, workType?, templateId?\|"none", prompt?}` | `201 Notebook` |
| `POST /v1/chats/{chatId}/messages/{messageId}/output` | `{workType}` | `201 {notebookId, outputId, runId, title, markdown}` |
| `GET /v1/chat-memory` | — | `{"memory": [string]}` |
| `PUT /v1/chat-memory` | `{memory: [string]}` (≤ 50 items, ≤ 1000 chars) | `{"memory": [string]}` |
| `GET /v1/approvals` | — | `{"approvals": [PendingApproval]}` (unanswered approvals and interrupted runs) |
| `POST /v1/runs/{runId}/approvals/{approvalId}` | `{decision, remember?}`: decision `allow` or `deny`; remember `run` or `notebook` | `202 {"status": "resuming"}` or `{"status": "answered"}` when another approval is still open |
| `POST /v1/runs/{runId}/resume` | — | `202 {"status": "resuming"}` (interrupted runs, or paused runs with every approval answered) |
| `POST /v1/runs/{runId}/cancel` | — | `204` |
| `GET /v1/runs/{runId}/events` | — | SSE of a live run, always ending with a `status` event `{runId, status}`; a run not executing gets only that event |

`Notebook`: `{id, name, workType, settings, createdAt, outputs, memory, template?, context: [{id, label, path,
inPlace, enabled, addedAt, sources, estimatedTokens}]}`.

`Template`: `{id, name, description, workType, version, parent?, prompts: [{name, text, settings}],
slots: [{name, kind, description, required}], memory, settings, schedule?: {cron, promptName, missed},
source: "builtin"|"user", parentUpdate?, changedPrompts?}`. Templates never contain context data;
anything that looks like a credential is rejected.

`Output`: `{id, runId, workType, title, createdAt, notebookId, notebookName, data?, markdown?}`.

`Schedule`: `{id, cron, prompt, promptName, vars, overrides, missed: "skip"|"run_once", enabled,
createdAt, notebookId, nextRun, history: [{scheduledFor, startedAt, durationMs, status, runId,
outputId, error}]}` (status `ok`, `failed`, `skipped` or `waiting_approval`). Cron: five fields, names, ranges, steps, lists, `@daily`-style macros and an
optional `CRON_TZ=Area/City` prefix; otherwise the core's local time zone.

## Events stream

Each event is `event: event` with `data: {kind, notebookId, notebookName, scheduleId,
scheduledFor, runId?, outputId?, title?, error?, time}` where `kind` is `schedule.started`,
`schedule.finished`, `schedule.failed`, `schedule.skipped` or `schedule.paused` (an agent run waits for approval;
later occurrences are skipped with `error: "previous run awaiting approval"` until it is answered). Slow subscribers drop events;
at most 16 streams at once.

Agent runs add events with `{kind, runId, notebookId, notebookName, scheduleId?, approvalId?, tool?, error?,
time}` where `kind` is `run.approval_needed`, `run.resumed`, `run.finished`, `run.failed` or `run.cancelled`.

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
connection cancels a pipeline run.

Agent runs (`overrides.mode: "agent"`) also send `tool_call` `{text, id, tool, args}`, `tool_result`
`{text, id, tool, result}` (≤ 2 KiB) and `approval_needed` `{text, runId, approvalId, tool, args}`. A run that
stops to wait for approval ends with `event: paused` and `{runId, approvals: [PendingApproval]}`. Closing the
connection does not stop an agent run: cancel it with `POST /v1/runs/{runId}/cancel` and re-attach with
`GET /v1/runs/{runId}/events`. Errors: `404` unknown run or approval; `409` already answered, expired, still
waiting for an approval, not paused, or in use by another process; `429` too many runs.

`PendingApproval`: `{runId, notebookId, notebookName, scheduleId?, status, approvalId?, tool?, args?, requestedAt,
expiresAt?}`. `status` is `waiting_approval`, or `interrupted` for a run that stopped when the core shut down or
crashed (resume or cancel it; it blocks its schedule until then). Approvals expire after
`agent.approval_expiry_days` (default 7), checked whenever pending runs are listed; the run then fails.

## Chat

`ChatSession`: `{id, title, pinned?, archived?, model?, thinkingEffort?, createdAt, updatedAt,
messages: [ChatMessage]}`; `model` and `thinkingEffort` left empty use the app defaults.
`ChatSummary`: `{id, title, pinned?, archived?, model?, updatedAt, messageCount, preview}`.
`ChatMessage`: `{id, role: "user"|"assistant", content, createdAt, model?, stopped?, error?,
tokens?, attachments?: [{kind, ref, label, tokens}]}`.

Attachment kinds: `path` (absolute file or folder), `notebook` (its enabled context) and
`output` (`ref` is `notebookID/outputID`). Context attached to earlier messages stays available
to later turns. Search (`q`, ≤ 200 chars) matches titles and message text, case-insensitively.

Sending validates first (`400` bad message, `404` unknown chat, `409` a reply is already
streaming in that chat, `429` too many runs and replies), then streams:

```
event: delta
data: {"text":"Rotate "}

event: finished
data: {"user": ChatMessage, "assistant": ChatMessage}
```

**Make this repeatable.** `RepeatablePlan` is `{name, workType, templateId?, templateName?, prompt,
context: [absolute path], skipped: ["label: why"]}`: the work type is suggested from the chat's
wording, a template of that type is suggested when it shares at least two words with the chat,
the prompt is the last user message, and path and notebook attachments become context (saved
outputs are skipped). Creating the notebook applies the plan with any overrides; the prompt is
saved as `from-chat`.

**Save a reply.** The model restructures the reply (with the question it answered) into the
work type's output; it is validated against the schema and saved in the `chat-<workType>`
notebook, created on first use. Invalid output returns `422` and nothing is saved.

A model failure ends with `event: error` and `{"status", "error", "reply"}`; the failed reply is
stored with `stopped: true` and a short `error`. Closing the connection is the stop button: the
partial reply is stored with `stopped: true`.
