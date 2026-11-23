# Local API (v1)

The desktop app starts the core itself (`tess serve --exit-with-parent`, reusing a healthy running
core if there is one); you can also run it by hand. The core (`tess serve`) listens on a Unix socket (macOS/Linux) or a loopback port (Windows) and
writes its address and token to `endpoint.json` (0600) in the config dir's `run/` folder.
Every request except `GET /v1/health` needs `Authorization: Bearer <token>`. Bodies are JSON
(`Content-Type: application/json`, ≤ 1 MiB, unknown fields rejected). Errors are
`{"error": "..."}` with 400 (invalid), 401, 403 (browser request), 404, 421 (bad host),
422 (output failed validation), 429 (too many runs, or a token limit reached) or 500.

| Method and path | Body | Response |
| --- | --- | --- |
| `GET /v1/health` | — | `{"ok": true, "version": "…"}` |
| `GET /v1/worktypes` | — | `{"workTypes": [{id, name, question, starterPrompt, suggestedContext, schema}]}` |
| `GET /v1/notebooks` | — | `{"notebooks": [Notebook]}` (no context stats) |
| `POST /v1/notebooks` | `{name, workType}` or `{templateId, name?}` | `201 Notebook` |
| `GET /v1/notebooks/{id}` | — | `Notebook` with context stats |
| `PATCH /v1/notebooks/{id}` | `{name?, archived?}` | `Notebook` (archived notebooks keep their schedules but do not run them) |
| `DELETE /v1/notebooks/{id}` | — | `204`; `409` while a run is executing, paused for an approval or interrupted |
| `GET /v1/notebooks/{id}/prompts` | — | `{"prompts": [Prompt]}` sorted by name |
| `PUT /v1/notebooks/{id}/prompts/{name}` | `{text, settings?}` | `Prompt` (name: letters, digits, `.`, `_`, `-`; text 1 B–16 KiB) |
| `DELETE /v1/notebooks/{id}/prompts/{name}` | — | `204`; `409` while a schedule runs it |
| `GET /v1/usage` | — | `{day, tokens, dayLimit, runLimit}`: today's tokens across every model call, and `limits.max_tokens_per_day` / `max_tokens_per_run` (0: none) |
| `GET /v1/search?q=&limit=` | — | `{"results": [{kind, notebookId, id?, title, snippet}]}`: notebooks, outputs (newest first), prompts, research notebooks and notes holding every word (case-insensitive), at most 100 |
| `POST /v1/notebooks/{id}/context` | `{path}` (absolute), `{server, uri, label?}` for an MCP resource, `{url, label?}` for a web page fetched on every run (http/https, no credentials), `{text, label?}` for pasted text (≤ 2 MiB, saved as Markdown), or `{git: {repo, base}, label?}` for a repository's changes against `base`, read with git on every run | `Notebook` |
| `PATCH /v1/notebooks/{id}/context/{contextId}` | `{enabled}` | `Notebook` |
| `POST /v1/notebooks/{id}/runs` | `{prompt?, promptName?, vars?, overrides?, save?, toolRules?}` | SSE stream |
| `GET /v1/outputs?notebook=&workType=` | — | `{"outputs": [Output]}` (no data) |
| `GET /v1/notebooks/{id}/outputs/{outputId}` | — | `Output` with `data` and `markdown` |
| `GET /v1/notebooks/{id}/outputs/{outputId}/export?format=md|docx|pdf` | — | the file (`Content-Disposition: attachment`): Markdown, or a Word document or PDF made with the standard library ||docx` | — | the file (`Content-Disposition: attachment`): Markdown, or a Word document made with the standard library |
| `GET /v1/notebooks/{id}/outputs/{outputId}/run` | — | `{runId, prompt, promptName?, vars?, model, mode?}`: the run that made the output, to run it again (`POST …/runs` with `overrides.model` for another model) |
| `GET /v1/notebooks/{id}/outputs/{outputId}/diff?against=` | — | `{from, to, lines: [{op, text}], counts: {added, removed}}`: the output against the version it revised (or `against`); `op` is the character code of `" "`, `"+"` or `"-"`; `400` for an original without `against` |
| `GET /v1/settings` | — | config (key references only) |
| `PUT /v1/settings` | `{providers, aliases, defaults, limits}` | the config after the change; validated, saved to the config file and applied at once (MCP servers have their own routes) |
| `GET /v1/secrets/{ref}` | — | `{set: bool}`: whether an API key is available (keychain or environment); its value never leaves the core |
| `PUT /v1/secrets/{ref}` | `{value}` | `204`; stores the key in the OS keychain (one line, ≤ 8 KiB); `503` when the core has no keychain |
| `DELETE /v1/secrets/{ref}` | — | `204` |
| `GET /v1/diagnostics` | — | a zip for bug reports: `about.txt` (version, OS), `checks.txt` (setup and counts, whether each key is set), `config.json` (key references only), `core.log` (its last 2 MiB); everything redacted, no keys, notebook names or content |
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
| `PATCH /v1/chats/{chatId}` | `{title?, pinned?, archived?, model?, thinkingEffort?, mcpServers?}` | `ChatSession` (`mcpServers`: configured servers whose tools the chat may use) |
| `DELETE /v1/chats/{chatId}` | — | `204` |
| `POST /v1/chats/{chatId}/messages` | `{text, attachments?: [{kind, ref, label?}]}` | SSE stream (see below); with `mcpServers` set it also sends `tool` (`{kind: call|result|denied|blocked, tool, args?, summary?}`) and `approval` (`{id, tool, args}`) events |
| `POST /v1/chats/{chatId}/approvals/{callId}` | `{allow, remember?}` | `204`: answers a tool call the reply waits on (`approval` event); `remember` keeps it as this chat's rule; `404` when none waits. A reply waits up to 5 minutes, then the call is denied |
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
| `GET /v1/mcp/servers` | — | `{"servers": [MCPServer]}` |
| `POST /v1/mcp/servers` | `MCPServer` | `201 MCPServer` (`400` invalid or a credential written as text, `409` name taken) |
| `DELETE /v1/mcp/servers/{name}` | — | `204` (`404` unknown, `409` attached to a notebook) |
| `POST /v1/mcp/servers/{name}/test` | — | `{tools: [{name, description}], resources: [{uri, name, mimeType}], prompts: [{name, description}]}` |
| `PUT /v1/notebooks/{id}/mcp` | `{servers: [name]}` | `Notebook` (`400` for a server that is not configured; at most 20) |
| `PUT /v1/notebooks/{id}/mcp` | `{servers: [name]}` | `Notebook` (`400` for a server that is not configured; at most 20) |
| `GET /v1/notebooks/{id}/outputs/{outputId}/thread` | — | `ChatSession` of the output's thread (empty if none yet) |
| `PATCH …/outputs/{outputId}/thread` | `{model?, thinkingEffort?}` | `ChatSession` |
| `POST …/outputs/{outputId}/thread/messages` | `{text, attachments?}` | SSE stream (see Output threads) |
| `GET …/outputs/{outputId}/suggestions` | — | `{"suggestions": [Suggestion]}` |
| `POST …/outputs/{outputId}/suggestions/generate` | — | `{"suggestions": [Suggestion]}` (the new ones) |
| `POST …/outputs/{outputId}/suggestions/{suggestionId}/apply` | — | `{suggestion, promptName?}` |
| `POST …/outputs/{outputId}/suggestions/{suggestionId}/dismiss` | — | `{suggestion}` |
| `POST …/outputs/{outputId}/versions` | `{fromMessageId?}` | `201 Output` (`422` if the revision fails validation) |

`Notebook`: `{id, name, workType, settings, createdAt, outputs, memory, template?, mcpServers, context: [{id, label, path,
inPlace, enabled, addedAt, sources, estimatedTokens}]}`.

`Template`: `{id, name, description, workType, version, parent?, prompts: [{name, text, settings}],
slots: [{name, kind, description, required}], memory, settings, schedule?: {cron, promptName, missed},
source: "builtin"|"user", parentUpdate?, changedPrompts?}`. Templates never contain context data;
anything that looks like a credential is rejected.

`Output`: `{id, runId, workType, title, createdAt, notebookId, notebookName, version, parentId?, fromThread?,
data?, markdown?}`. `version` is 1 for an original output and one more for each revision; `parentId` is the
output a version revises and `fromThread` the thread reply it was made from.

`Prompt`: `{name, text, settings: {model?, temperature?, max_output_tokens?, thinking_effort?}}`; a run or schedule given `promptName` uses its text and settings (layered between the notebook and the run).

`Schedule`: `{id, cron, prompt, promptName, vars, overrides, maxTokens?, missed: "skip"|"run_once", enabled,
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

## Output threads

Each output has one thread (`…` above is `/v1/notebooks/{id}`), stored in the notebook's `threads/` folder and
not listed with chats. A reply sees the output, the prompt behind it, the notebook's pinned memory and enabled
context; global chat memory is not used. The model, thinking effort and stop button work as in chat. Messages
with role `note` (for example "Saved as v2 (…)") are for the reader and never sent to the model.

Sending validates first (`400`, `404` unknown notebook, output or run, `429`), then streams `delta`,
`suggestion` (a `Suggestion`, once per suggestion the reply makes) and `finished`
(`{"user", "assistant", "suggestions"}`); `event: error` carries `{"status", "error", "reply"}`, with status
`409` when a reply or a new version is already running in that thread.

`Suggestion`: `{id, kind: "memory"|"prompt", text, reason?, messageId?, status: "open"|"applied"|"dismissed",
createdAt, decidedAt?, target?}`. `target` (prompt suggestions) says what applying changes:
`{kind: "saved"|"schedule"|"new", promptName?, scheduleId?, schedules?: [scheduleId], current}`:

- `saved`: the saved prompt the run used (directly or through its schedule); every schedule in `schedules`
  uses it and changes with it.
- `schedule`: that schedule's own prompt.
- `new`: an ad-hoc run; applying saves a new prompt `refined-<n>` and returns its `promptName`. Later suggestions
  and versions of that output then target that prompt.

Runs recorded before prompt names and schedules were tracked are matched by exact prompt text when exactly one
saved prompt or schedule prompt has it; a saved prompt that no longer exists falls back to the schedule or `new`.

Suggestions come from the model's `suggest_memory` and `suggest_prompt` tools (at most 3 per reply) or from
`suggestions/generate` (one structured call, for models without tool support). An output keeps at most 20 open
suggestions; `suggestions/generate` answers `409` when that cap is reached. Applying a memory suggestion appends it to pinned memory (`409` when memory is full). Applying a
prompt suggestion answers `409` if the prompt changed after the suggestion was made, and `400` if it adds a
`{{variable}}` that the run, or any schedule the change reaches, does not set. Applying or dismissing a resolved suggestion answers `409`. Applies are
audited as `suggestion.apply` (IDs, kind and a digest only).

**New version.** The output's data and the thread up to `fromMessageId` (default: the latest reply) go through the
pipeline with the notebook's current context, memory, prompt and model, also for agent-mode notebooks. The result
is validated and saved as a new output with `parentId`; the parent never changes. Invalid output answers `422` and
saves no output.

## MCP servers

`MCPServer`: `{name, command?, args?, env?, url?, headers?, allow_insecure_http?}`; exactly one of `command` (a
program started over stdio, no shell) or `url` (Streamable HTTP; `https`, or `http` to loopback unless
`allow_insecure_http`). Names use `a-z`, `0-9`, `_` and `-` (at most 32); at most 50 servers. Servers are saved in
the config file under `mcp.servers`.

Credentials never go in the config: `env` and `headers` values may hold `${secret:NAME}` placeholders (arguments and URLs may not hold credentials at all), resolved
from the environment or the OS keychain only when the server starts. A value that looks like a credential, or any
text besides placeholders and a scheme word (`Bearer`, `Basic`, `Token`) under a name that looks sensitive (token,
key, secret, password, auth, cookie, credential), is refused with `400`. Values are returned as stored.

`test` starts or dials the server, completes the handshake and lists up to 500 tools, 500 resources and 200
prompts (a server without resources or prompts lists none). It takes a run slot (`429` when all are busy) and has a
60 s limit. Errors are redacted, including the values the placeholders resolved to.

**Notebook attachment.** A notebook's agent runs connect to its attached servers when they start (or resume) and
stop them when they finish or pause. Each server tool is offered as `mcp__<server>__<tool>` (lower case,
characters outside `a-z0-9_` replaced with `_`, at most 64; a clash gets `_2`, `_3`, …), with the server's input
schema when it is a JSON object of at most 16 KiB. Like every tool that is not read-only, MCP tools default to
`ask`; rules can name them exactly or with `mcp__<server>__*`. A server that cannot be reached fails the run and
names the server. Results are capped and redacted like other tool results.

**MCP resources as sources.** A context item with `kind: "mcp-resource"` names a configured server and a resource
URI (at most 2048 bytes). It is read on every run (pipeline and agent) and in threads and chats that attach the
notebook, through one connection per server, and counts toward the context budget like any other source; its
`estimatedTokens` is 0 until then. Only text is accepted (or a blob whose type is text or JSON), at most 2 MiB. A
resource that cannot be read fails the run with the server and URI in the message. Runs record it as
`mcp:<server>/<uri>`. The CLI form is `tess context add <notebook> mcp://<server>/<uri>`.

## Research notebooks

A second kind of notebook, stored under `<workspace parent>/research/<id>/`, separate from job notebooks. All routes
are under `/v1/research`.

| Route | Body | Returns |
|---|---|---|
| `GET /v1/research` | — | `{"notebooks": [Research]}` (no content checks) |
| `POST /v1/research` | `{name, description?}` | `201 Research` |
| `GET /v1/research/{id}` | — | `Research`; sources at full or summary are checked against their content, and changed or pending ones are queued for a summary |
| `PATCH /v1/research/{id}` | `{name?, description?, settings?}` | `Research` |
| `DELETE /v1/research/{id}` | — | `204` |
| `POST …/{id}/sources` | `{path}` (absolute file or folder) or `{server, uri}` (MCP resource) | `Research`; the summary runs in the background |
| `GET …/{id}/sources/{sid}` | — | `{text, summary}` (folders: files joined with their paths) |
| `PATCH …/{id}/sources/{sid}` | `{level: "full"\|"summary"\|"off"}` | `Research` (`400` for summary without a ready summary) |
| `DELETE …/{id}/sources/{sid}` | — | `Research` |
| `POST …/{id}/sources/{sid}/summary` | — | `202` (summarized again only if the content changed) |
| `GET …/{id}/context` | — | `{sources, summaries, notes, tokens, cut: [label]}`: what a question would send now, its size before fitting, and the sources that would be shortened |
| `GET\|POST …/{id}/chats` | `{title?}` | `{"sessions": [ChatSummary]}` / `201 ChatSession` |
| `GET …/{id}/chats/{cid}` | — | `{session: ChatSession, citations: {messageId: [Citation]}}` |
| `PATCH\|DELETE …/{id}/chats/{cid}` | `{title?, model?, thinkingEffort?}` | `ChatSession` / `204` |
| `POST …/{id}/chats/{cid}/messages` | `{text}` | SSE: `delta {text}`, then `finished` (`{user, assistant, citations, skipped?}`) or `error` |
| `POST …/{id}/chats/{cid}/messages/{mid}/note` | — | `201 Note` |
| `GET\|POST …/{id}/notes` · `PATCH\|DELETE …/{id}/notes/{nid}` | `{title?, text?, include?}` | `{"notes": [Note]}` / `Note` / `204` |

`Research`: `{id, name, description, createdAt, settings, notes, sources: [Source]}`. `Source`:
`{id, label, kind: "file"|"folder"|"mcp-resource", path?, server?, uri?, level, levelSet?, tokens?, summary:
"ready"|"failed"|"pending"|"updating"|"unreadable", error?}` (`unreadable`: the source cannot be read now; it is skipped when sending). `Note`: `{id, title, text, origin: "human"|"ai", include}`.
`Citation`: `{n, kind: "source"|"note", id, label, level?, path?}`.

**Summaries.** One per source, made in the background (at most 2 at a time) with the notebook's model, and remade
only when the source's content changes (a sha256 fingerprint of what extraction reads). A new source starts at
`full` and switches to `summary` when its first summary is ready, unless the user set a level. The events stream
sends `research.summary.started|ready|failed` with `notebookId` and `sourceId`.

**Questions.** Sources at `full` send their text (a folder as one block of files), sources at `summary` their
summary, then included notes; sources and notes may use 3/5 of the budget, largest cut first. The model cites with
`[n]` or `[n:path]`; numbers it was not given are ignored, and a path outside the folder is dropped. Earlier answers
are sent with their numbers replaced by labels. A source that cannot be read is skipped, and the stored answer
starts with a line naming it.
