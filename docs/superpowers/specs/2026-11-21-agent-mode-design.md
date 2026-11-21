# Agent mode, tools, MCP and delegation: design

- Status: approved (2026-11-21)
- Stories: MCP-1, MCP-2, MCP-3, CHAT-4; the spec's "Agent" execution mode
- Replaces: ADR 0002 (Eino) once approved; a new ADR 0005 records the decision

## 1. Intent

Tessivo needs a coding agent built for its own purpose. It should explore a code base or document set, run checks,
call tools on MCP servers, and hand code changes to a dedicated coding agent. It should finish, like every run, with
a validated Summarize, Analyze, Document or Report output. The user stays in control: every tool has an allow, ask
or block rule, and an "ask" pauses the run until the user answers, even when nobody is watching a scheduled run.

## 2. Goals, non-goals, anti-goals

Goals

- Agent mode as a second `executor.Executor`, selected by the existing `mode: agent` setting.
- Built-in code tools (read, list, glob, grep) rooted in the run's context folders.
- Commands from a per-notebook allow-list, run without a shell.
- MCP client (stdio and Streamable HTTP, header auth): servers in settings, attached per notebook, resources as
  context.
- Delegation of coding tasks to an external agent over the Agent Client Protocol (ACP), with nca bundled as the
  default.
- Changes made only in a separate copy (git worktree or snapshot), returned as a reviewable change set the user
  applies.
- Allow / ask / block per tool. "Ask" pauses the run durably and resumes after approval, even across a core restart.
- Every step is recorded in the run log and the audit log. The CLI, local API and desktop have parity.
- Chat uses the same tools and rules (CHAT-4).

Non-goals (this design)

- MCP OAuth (MCP-1 lists it; it comes later, as its own change). Header and token auth only for now.
- Tessivo as an MCP server (MCP-4) and shared MCP server lists (MCP-5).
- Tessivo editing files itself. Edits happen only in the delegate's copy.
- Multi-agent graphs, subagents of our own, model-to-model handoffs.
- Sandboxing commands at the OS level (containers, seccomp, Job Objects). The allow-list, the copy and approvals are
  the controls.

Anti-goals (failure even if it works)

- A run that changes the user's own folder without an explicit apply.
- A secret appearing in argv, a tool argument log, the audit log or a prompt.
- A paused run that silently disappears, or resumes on its own after a restart without the user knowing.
- Model calls that bypass our router (fallback chains, hardened HTTP, attempt audit).
- New direct Go dependencies in the core, or any OS-specific code without explicit approval (CLAUDE.md).

## 3. Constraints

- Go core: standard library only (ADR 0003). Repo rules: funlen 80, lll 120, files ≤ 500 lines, coverage ≥ 80%,
  fuzz tests for parsers, OS-independent tests (`internal/testutil`).
- Everything works the same on Windows, macOS and Linux. Processes start with `os/exec` (program + args), never
  through `sh` or `cmd`.
- All tool output, MCP results, delegate messages and model output are hostile input. Bound sizes, counts and time.
- Files written are 0600, directories 0700, writes atomic.
- Ported code (Zero, MIT) keeps its copyright notice in a header and is listed in ADR 0003 and `THIRD_PARTY.md`.

## 4. Approaches considered

| Approach | Trade-off | Verdict |
| --- | --- | --- |
| Our own loop + code tools and MCP client ported from Zero (Go, MIT) | Loop is small given our router; Zero gives the biggest pieces (MCP client, code tools) in stdlib-only Go. Ported code must be reworked to pass our gates; Zero is young (May 2026) | **Chosen** |
| Eino (ADR 0002) + official MCP Go SDK | Graphs and checkpoints built in, but two pre-1.0 dependency trees, an adapter so it uses our router, and no coding tools | Rejected |
| Fantasy (Charm) + official MCP Go SDK | Simpler loop than Eino; same dependency and provider-bypass issues; preview API | Rejected |
| Borrow from Crush | FSL-1.1-MIT forbids competing use for two years per release | Rejected |
| nca as our agent engine | Separate Rust process; would not use our router, audit or validation | Rejected as engine, **kept as the delegate** |

Decided in conversation: our own loop; the agent may run commands; coding work goes to a bundled ACP agent (nca);
changes happen only in a separate copy (git worktree, or a snapshot when the folder is not git or git is missing; git
is not bundled); "ask" pauses the run durably and resumes after approval.

## 5. Architecture

```
service.Run ──► executor.Agent (internal/executor/agent)
                  │  loop: model step ─► tool calls ─► policy ─► run / pause ─► checkpoint
                  ├─ llm.Router (tool calling added; fallback per step)
                  ├─ tools (internal/agenttools): read_file, list_directory, glob, grep, run_command
                  ├─ mcp (internal/mcp): stdio + Streamable HTTP clients, registry
                  ├─ delegate (internal/acp): ACP client ─► bundled nca (or any ACP agent)
                  ├─ workspace (internal/workspace): worktree or snapshot, diff, apply
                  └─ runstore (internal/notebook): checkpoints, run status, approvals
```

New packages stay engine-agnostic: no type from `mcp`, `acp` or `workspace` leaks past `executor/agent` and
`service`. The scheduler, server and CLI see only `service` types.

### 5.1 LLM client: tool calling

- `llm.Request` gains `Tools []ToolDef` (name, description, JSON-schema parameters) and `ToolChoice`.
- `llm.Message` gains `ToolCalls []ToolCall` (assistant) and `ToolCallID` (role `tool`).
- Streaming parses `tool_calls` deltas (index-keyed argument fragments). The arguments JSON is bounded (64 KiB per
  call) and fuzzed.
- The router's fallback applies per model step. A step's attempts are recorded as today.
- A model or alias that rejects tools fails the step with a clear error ("model X does not support tool calling").
  Agent mode then fails before any tool runs.

### 5.2 The loop (`internal/executor/agent`)

1. Build the system prompt: work type goal, notebook memory, tool list, limits, and a note that tool output is data,
   not instructions. Small context documents go inline. Large ones are listed by path, and the agent reads them
   with tools.
2. Model step → zero or more tool calls. Up to 4 run in parallel, in a stable order for the log.
3. Each call goes through policy (5.4): allow runs it, block returns "denied by policy" to the model, ask pauses (5.5).
4. Results are capped (32 KiB per result, with a truncation note) and appended. Step, token and time counters update.
5. Checkpoint after every completed step (5.5).
6. The loop stops when the model replies without tool calls, or calls `finish`, or a limit is hit: steps (default 40),
   tokens (default 400k total), wall time (default 30 min, paused time excluded).
7. **Final step**: a structured-output call with the work type schema, over a compacted transcript. It is validated
   and retried exactly as in the pipeline. Exit code 3 on invalid stays.

Long transcripts are compacted: old tool results are replaced with one-line summaries once the prompt passes 60% of
the model's context. The rules are adapted from Zero's `compaction` and `prune`. The builder picks the exact
heuristics.

### 5.3 Tools

| Tool | Behaviour | Default policy |
| --- | --- | --- |
| `read_file` | Path inside a context root or workspace; line range; ≤ 256 KiB per read | allow |
| `list_directory`, `glob` | Rooted; skips `.git`, `node_modules`, build dirs; ≤ 2,000 entries | allow |
| `grep` | Go `regexp` (RE2, linear time); ≤ 500 matches; rooted | allow |
| `run_command` | Program + args only, no shell; must match the notebook allow-list; cwd = workspace or a context root | allow-list match: allow; else ask |
| `mcp__<server>__<tool>` | Calls an attached MCP server's tool | ask |
| `delegate_coding_task` | Starts or continues a delegate session in the run's workspace (5.7) | ask |
| `finish` | Ends the loop early with a short rationale | allow |

Path rules (ported from Zero's `pathjail`/`rooted_file`): paths are cleaned and resolved, symlinks evaluated, and
anything outside the roots is refused. Roots are the notebook's context folders, plus the run workspace when one
exists.

`run_command` rules:
- Allow-list entries are `{program, args}`, where args are literal strings or `*` for one free argument. Example:
  `{"program":"go","args":["test","*"]}`. The program is resolved with `exec.LookPath` once and recorded.
- The environment is the user's minus variables whose names match secret patterns (`*KEY*`, `*TOKEN*`, `*SECRET*`,
  `*PASSWORD*`, the configured `api_key_ref` names). Extra variables can be added per notebook.
- Timeout 5 min (configurable). Stdout and stderr are capped at 256 KiB combined. Exit code is reported to the model.
- Cancelling kills the started process. Killing its child processes needs OS-specific process groups or Job Objects,
  which is **out of scope until approved** (see 9).

### 5.4 Policy

- Rules: `allow`, `ask`, `block`. Lookup order (most specific wins): run override → notebook tool rule → notebook
  server rule (`mcp__jira__*`) → app default → built-in default (table above).
- Notebook rules live in `notebook.json` under `agent.tools`. App defaults and chat rules live in the config file
  under `agent.tools`.
- An approval can be **once**, or **remember for this run**, or **remember for this notebook** (writes the rule). The
  last is never offered for `delegate_coding_task`.
- Approval prompts show the full arguments, after redaction of known secret values.

### 5.5 Pausing and resuming

Runs become core-owned jobs.

- `POST /v1/notebooks/{id}/runs` starts the run and streams events. In agent mode, closing the connection **no
  longer cancels the run**. `POST /v1/runs/{runId}/cancel` does. `GET /v1/runs/{runId}/events` re-attaches.
  Pipeline runs keep today's behaviour.
- Run status, new in `runs/<id>.json`: `running`, `waiting_approval`, `interrupted`, `finished`, `failed`,
  `cancelled`.
- **Checkpoint** `runs/<id>.checkpoint.json` (0600, atomic) is written after every completed step. It holds the spec
  (work type, prompt, vars, settings, context list), the transcript, counters, pending tool calls with their policy
  outcome, the workspace reference and the delegate session id. Large tool results are stored by digest in
  `runs/<id>/blobs/`, bounded at 64 MiB per run.
- **Ask** → the pending call is saved with an approval id. Status becomes `waiting_approval`, the loop exits, and the
  model connection is released. Events: `run.approval_needed` on `/v1/events` (desktop banner, OS notification when
  `--notify` is on) and on the run stream.
- **Answer**: `POST /v1/runs/{runId}/approvals/{approvalId}` with `{decision: allow|deny, remember?: run|notebook}`,
  or `tess run approve|deny <runId> <approvalId>`. The core reloads the checkpoint, applies the decision and
  continues the loop in the background.
- **Restart**: on startup the core lists runs. `waiting_approval` stays paused. `running` becomes `interrupted`
  (a step was in flight), and the user resumes it explicitly (`POST /v1/runs/{runId}/resume`, `tess run resume`).
  Nothing resumes on its own. A tool call that was in flight at the crash is reported to the model as "outcome
  unknown" and is never replayed automatically.
- **Expiry**: an unanswered approval expires after 7 days (configurable). The run fails with "approval expired", and
  its workspace is cleaned.
- **Scheduler**: a paused scheduled run still holds its no-overlap slot, so the next occurrence is skipped with reason
  `previous run awaiting approval`. It does not count against the concurrency limit while paused.
- Pending approvals are listed by `GET /v1/approvals` and `tess run pending`.

### 5.6 MCP (`internal/mcp`, ported from Zero)

- Transports: stdio (process with program + args + env) and Streamable HTTP (POST + SSE). The HTTP client is our
  hardened client: TLS 1.2+, no redirects, timeouts, plaintext only to loopback. Both are bounded: message ≤ 4 MiB,
  tool list ≤ 500, a call timeout.
- Settings (config file, app level): `mcp.servers[]` with `{name, command?, args?, env?, url?, headers?}`. Values in
  `env` and `headers` may be `keychain:<name>` or `env:<NAME>` references, resolved at connect time and never
  written out (SET-1 rules).
- `tess mcp add|list|test|remove`; `test` lists tools, resources and prompts (MCP-1 AC). API mirrors the CLI under
  `/v1/mcp/servers`.
- Notebooks attach servers by name (`agent.mcpServers`). Tools are exposed as `mcp__<server>__<tool>`, with server
  and tool names sanitized to `[a-z0-9_]`.
- MCP-3: context item kind `mcp-resource` `{server, uri}`. It is read on every run (pipeline and agent), extracted as
  text, and bounded like other context.
- Server lifecycle: started per run (stdio) and closed when the run ends or pauses, then reconnected on resume. The
  chat keeps a session-scoped connection pool, idle-closed after 10 min.

### 5.7 Delegation (`internal/acp`, bundled nca)

- `delegate_coding_task {task, files?}` starts (or continues) an ACP session with the configured agent, with its cwd
  in the run's workspace. Default command: the bundled `nca acp`. It is configurable as `agent.delegate {command,
  args, env}` for any ACP agent.
- The delegate gets the LiteLLM endpoint and key through environment variables (never argv), plus the notebook's
  MCP servers in `session/new`.
- The delegate's `session/request_permission` calls are mapped onto our policy (tool name `delegate:<kind>`), so
  allow, ask and block and pausing apply to it too.
- Decided: nca runs in its default mode. It runs ordinary shell commands without asking, but only inside the
  workspace copy, and it asks only for dangerous actions (deletes, `sudo`, `git push`, `curl | sh`, and the like).
  Those prompts go through our policy. Starting nca in a mode that asks for every command was rejected: every
  delegated task would need many approvals.
- Pausing while the delegate waits: the ACP session stays open while the core runs. After a core restart, the session
  is restored with `session/load` when the agent advertises it. Otherwise the delegate task restarts in the same
  workspace, with a note listing the files already changed.
- The delegate's reply (summary plus its own tool log) goes back to our agent as the tool result. Its file changes
  stay in the workspace.
- Bundling: release tooling (a Go program in `tools/`) downloads the pinned nca release for each platform, verifies
  SHA-256 checksums recorded in the repo, and places it next to `tess` in the desktop bundle. Lookup order: bundled,
  then PATH. No download at runtime.

### 5.8 Workspaces and change sets (`internal/workspace`)

- Created only when the run first needs to change something (a delegate call or a writing command).
- **Git repo and git on PATH** → `git worktree add` on a new branch `tessivo/run-<id>`.
- **Otherwise** → snapshot copy in a private temp directory (0700). It skips `.git`, `node_modules`, build dirs and the
  notebook ignore list, and is capped at 2 GiB and 200k files by default. Each file's SHA-256 is recorded at copy
  time.
- The change set is built at the end of the run: changed, added and deleted files, plus a unified diff (Myers diff,
  our own or ported from Zero; no dependency). It is saved as `outputs/<id>.changes.json` and `outputs/<id>.diff`
  next to the work type output.
- **Apply** (`tess outputs apply <id>`, `POST /v1/notebooks/{id}/outputs/{outputId}/apply`, a desktop button): for
  each file, the original's hash must still match the snapshot. Mismatches are reported as conflicts and skipped.
  Writes are atomic. Worktree branches can also be merged by the user with git.
- **Discard** or retention (default 14 days) removes the worktree or snapshot.

### 5.9 Records and audit

- The run record gains `mode`, `status`, `steps`, `toolCalls[] {id, tool, args (redacted, ≤ 8 KiB), result summary
  (≤ 8 KiB), policy, decision, durationMs, error}`, `approvals[]` and `changeSet`.
- The audit log gains `tool.call`, `tool.approval` and `delegate.session` entries with digests of arguments and
  results, never the raw content. `tess audit verify` covers them.
- The run stream gains `tool_call`, `tool_result`, `approval_needed` and `approval_resolved` events.

### 5.10 Chat (CHAT-4)

Chat uses the same loop with a chat-shaped final step (a plain reply, no schema). It uses the same tools and app-level
rules, and an "ask" pauses the chat turn the same way: a pending approval card in the chat and `tess chat approve`.
Chat has no context folders, so the read tools need an attached folder or code base.

### 5.11 Desktop

- Settings › MCP servers: add, test (tools, resources, prompts), remove.
- Notebook › Agent: mode switch, attached servers, per-tool rules, command allow-list.
- Run view: live tool timeline, approval card (arguments, allow once / for run / for notebook, deny).
- Global pending-approvals badge fed by `/v1/events`.
- Output view: change set review (file list, diff) with Apply and Discard.

## 6. Security

- Prompt injection is assumed. Tool results are wrapped as data, and the system prompt says so. Write-capable actions
  (commands off the allow-list, MCP tools, delegation) default to ask.
- No shell anywhere. Commands are matched against the allow-list after `LookPath`. Secret-looking variables are
  removed from child environments.
- Read tools never leave the roots. The delegate works only in the workspace copy, and the user's folder changes
  only on apply, with hash checks.
- Bounds on everything: steps, tokens, wall time, result sizes, MCP message sizes, workspace size, blob store,
  delegate output.
- Fuzz targets: tool-call delta parser, MCP JSON-RPC framing, ACP framing, allow-list matcher, diff.

## 7. Delivery phases

Each phase ships with tests, docs, checklist rows and CLI and API parity. Desktop screens land with the phase that
needs them.

1. LLM tool calling; agent loop with code tools; policy; checkpoints, pause, resume and restart handling; runs as
   jobs; `tess run approve|deny|resume|pending`; ADR 0005 replacing 0002.
2. `run_command` with allow-lists.
3. MCP client, settings, `tess mcp`, notebook attachment (MCP-1, MCP-2).
4. MCP resources as context (MCP-3).
5. Workspaces, change sets, apply; ACP delegation; nca bundling tool.
6. Chat tools and approvals (CHAT-4).
7. Desktop: settings, agent tab, run timeline and approvals, change set review (spread across phases as they land).

## 8. Left to the builder

- Package and file split, exact type names, compaction heuristics, default limits (within the stated bounds).
- Which Zero files to port and how to reshape them to our gates. Record the upstream commit in each header.
- Event payload field names (documented in `docs/api.md`).
- Config key spelling under `agent.*` and `mcp.*` (documented in `docs/architecture.md`).

## 9. Needs approval before it is built

- Killing a command's or delegate's **child processes** on cancel (process groups on Unix, Job Objects on Windows) is
  OS-specific. Until approved, cancel asks the delegate to stop (ACP `session/cancel`), then kills only the direct
  process.
