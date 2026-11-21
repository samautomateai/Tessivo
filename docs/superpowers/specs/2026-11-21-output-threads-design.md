# Output threads: ask about an output, refine it, improve the notebook

Status: design, approved approach (2026-11-21).

## Intent

Today a run ends with an output, and that is the end of it. Users want to keep talking to that output inside its
notebook. They want to ask about it ("why is auth risk high?", "which file does point 3 come from?") and tune it
until it is right. What they learn should also make the notebook's next runs better. This loop of asking, refining
and improving is what sets Tessivo apart from a one-shot report tool. The layout borrows from Copilot Notebooks
(sources, conversation, outputs), but Tessivo keeps its own model: a notebook is a repeatable job that produces
validated outputs.

## Goals

- Each output has its own conversation (an **output thread**) inside the notebook. The thread sees the output,
  the notebook's enabled context and its pinned memory.
- **Refine the output.** A New version action produces a new validated output from the thread. It is linked to its
  parent, and the parent is kept unchanged.
- **Improve the notebook.** The thread produces suggestions to pin to memory or change the prompt. The user applies
  or dismisses each one. Nothing changes without a click. An applied suggestion takes effect from the next run,
  scheduled runs included.
- The notebook page becomes three panes: Sources (left), the run bar, selected output and thread (middle), and
  Outputs (right).

## Non-goals (later pieces)

- One-click "Creations" buttons (Summary and the like), the Library card grid, and the Recent list in the sidebar.
- A notebook-level thread for asking about sources before the first run.
- Threads on outputs saved from global chat (`chat-<workType>` notebooks). They work, but the UI does not
  highlight them.

## Anti-goals

- A second chat system. Threads reuse the chat core: message types, streaming, stop, model and thinking settings,
  context building and budget.
- Silent changes. A suggestion never edits memory, a prompt or a schedule by itself.
- Unvalidated versions. A new version passes the same schema validation as a normal run. If it fails, nothing is
  saved (`422`), as for CHAT-7.
- Rewriting history. The parent output and its run record are never modified.

## Decisions

1. **One thread per output** (user's choice). There are no notebook-wide threads. A scheduled run's output gets a
   thread the same way, on first use.
2. **Both refine and improve** (user's choice).
3. **Suggestions: tools first, button as fallback** (user's choice).
   - The thread's model gets two tools, `suggest_memory` and `suggest_prompt`, through the OpenAI tool calling
     added for agent mode (`internal/llm/tools.go`).
   - If the model rejects tools (`llm.ErrToolsUnsupported`), the reply is retried without tools. The desktop then
     shows a **Suggest improvements** button, which runs one structured-output call over the thread and returns a
     validated list. The button is always available, not only as a fallback.

## Approaches considered (suggestions)

| Approach | Trade-off | Outcome |
|---|---|---|
| Tools during the reply | Natural and adds no cost, but needs a model that supports tools | Chosen (primary) |
| On-demand button, structured output | Works with any model that can produce outputs; user has to ask | Chosen (fallback) |
| Automatic check after every reply | Doubles the cost and delay of every reply, usually for nothing | Rejected |

Storage and refinement have one sensible approach each:

- **Threads** reuse `chat.Session` and its store, rooted in the notebook folder. A separate system would duplicate
  streaming and stop handling.
- **New version** reuses the run pipeline (`executor` NewRequest, Generate and validation). Anything else would skip
  validation.

## Design

### Storage (notebook folder)

- `threads/<outputId>.json` is a `chat.Session` (files 0600, folder 0700, atomic writes, the same size caps as
  chat). It is created on the first message.
  - The chat package's store assumes `Dir/sessions/<16-hex id>.json`.
  - The builder adapts it: a store rooted at the notebook's `threads/` folder, keyed by output ID with that ID
    validated. Threads stay out of the global chat list and search.
- `suggestions/<outputId>.json` holds suggestions with an ID, kind (`memory` or `prompt`), text, the reason given by
  the model, the source message ID, status (`open`, `applied` or `dismissed`) and timestamps.
  - At most 20 open suggestions per thread.
  - A memory item is at most 1000 characters.
  - A prompt is at most the existing prompt size cap.
- `Output` gains `parentId` (omitempty) and `fromThread` (the thread message the version was made from). The
  version number is derived from the parent chain for display (`v1`, `v2`, ...). It is not stored.
- `Run` gains `promptName` (omitempty), so a prompt suggestion knows which saved prompt the output came from. Runs
  without it are treated as inline prompts (see Apply).

### Thread replies

- Built like `Service.SendChat`, but the context comes from the notebook rather than from attachments:
  - The system part includes the output (its markdown rendering), its run's prompt, the notebook's pinned memory
    and the enabled context, within the chat budget (map-reduce is not needed: the context is trimmed by priority
    to fit, as chat does).
  - The user can still attach extra files to a message (CHAT-3 rules).
- Global chat memory is **not** included. Notebook memory is.
- The model, thinking effort, stop and SSE events are as in chat. The defaults come from the notebook's settings and
  can be overridden per thread.
- Tools: only `suggest_memory(text, reason)` and `suggest_prompt(text, reason)`.
  - Each call is validated (sizes, non-empty) and stored as an open suggestion. The model gets a short tool result
    ("recorded").
  - At most one extra model round per reply and at most 3 suggestions per reply. Anything beyond that is dropped.
  - Suggestion text is model output and therefore hostile: show it verbatim, never execute it, redact it with
    `secrets.RedactPatterns` before storing.
- An audit entry `thread.message` records digests only, like `chat.message`.

### Suggest improvements (button)

- One structured-output call over the thread, the output and the current memory and prompt. It returns
  `{suggestions: [{kind, text, reason}]}`, validated against a schema with the same caps. Results are stored like
  tool suggestions.

### Apply and dismiss

- **Memory:** appends to `Manifest.Memory` and respects the existing memory limits. If it is full, the response is
  `409` with a clear message.
- **Prompt:**
  - **Apply changes whatever the output's run actually used** (user's choice):
    - If the run used a saved prompt, Apply replaces that prompt's text and keeps its settings. Every schedule that
      uses that prompt changes with it.
    - If the run came from a schedule with its own inline prompt (`Schedule.Prompt`, found through
      `Run.ScheduleID`), Apply updates that schedule's prompt.
    - If the run was an ad-hoc run with an inline prompt and no schedule, Apply saves the text as a new saved prompt
      `refined-<n>` and returns its name.
  - The desktop shows a before/after view before the user applies. It names everything the change reaches, for
    example "Updates prompt `weekly-risk`, used by 2 schedules". `GET /suggestions` returns that reach with each
    open prompt suggestion.
  - For older runs without `promptName`, Apply looks up the prompt name through `Run.ScheduleID` →
    `Schedule.PromptName` before treating the prompt as inline (call made for the builder check).
  - Apply rejects the suggestion with `422` if it introduces a `{{var}}` the run or schedule does not set, because
    `executor.Fill` would make later runs fail (call made for the builder check).
  - Apply rejects the suggestion with `409` if the target prompt changed after the suggestion was made (call made
    for the builder check).
- Applying records `suggestion.apply` in the audit log (IDs and kind only) and marks the suggestion applied.
  Dismiss marks it dismissed. Neither changes the thread.

### New version

- `POST …/outputs/{oid}/versions` takes `{fromMessageId?}`, which defaults to the latest assistant message.
- **It revises the parent; it does not regenerate from scratch** (call made for the builder check). The model gets:
  - the parent output's JSON as the document to revise, with the instruction to change only what the requests ask
    for;
  - the thread's conversation up to that message, labelled as revision requests;
  - the run's work type and vars.
  The parent's tokens count toward the context budget.
- **It uses the notebook's current state** (call made for the builder check): enabled context, pinned memory, the
  current prompt and the notebook's model settings, the same as a fresh run. The thread's model override does not
  apply. Run records do not snapshot memory or context content, so an exact replay is not possible. Including the
  parent anchors the revision anyway.
  - For CHAT-7 outputs (`chat-<workType>` notebooks), the parent is the document being revised. Those notebooks need
    no special handling.
- **Versions always use the pipeline, even in agent-mode notebooks** (call made for the builder check). Revising a
  document needs no tools. This keeps the reply synchronous (`201` or `422`) with no approvals to wait for.
- It writes a normal run record and an output with `parentId` and `fromThread`. Validation failure returns `422` and
  saves nothing. The runs semaphore and limits are the same as for runs.
- The new output gets its own empty thread. The parent thread posts a system note "Saved as v2" (not sent to the
  model).

### API (additions to `docs/api.md`)

Each route is relative to `/v1/notebooks/{id}/outputs/{oid}`:

| Route | Body | Returns |
|---|---|---|
| `GET /thread` | — | `ChatSession` (empty if none yet) |
| `PATCH /thread` | `{model?, thinkingEffort?}` | `ChatSession` |
| `POST /thread/messages` | `{text, attachments?}` | SSE as chat, plus a `suggestion` event |
| `POST /suggestions/generate` | — | `{suggestions}` |
| `GET /suggestions` | — | `{suggestions}` |
| `POST /suggestions/{sid}/apply` | — | `{suggestion, promptName?}` |
| `POST /suggestions/{sid}/dismiss` | — | `{suggestion}` |
| `POST /versions` | `{fromMessageId?}` | `201 Output` (`422` if invalid) |

- Errors map as elsewhere: `404` for an unknown notebook, output or suggestion, `409` for a busy thread or a
  suggestion that is already resolved, `429` for the limits.
- `GET /v1/outputs` and `Output` gain `parentId`.

### CLI (parity, builder's call on exact flags)

- `tess output ask <notebook> <outputId> "question"` streams a reply and prints new suggestions.
- `tess output suggest|suggestions|apply|dismiss`.
- `tess output refine <notebook> <outputId>` creates a new version and prints its ID.

### Desktop

- The notebook page has three panes, replacing the stacked cards in `workspace_screen.dart`:
  - **Sources (left):** context with toggles and Add, pinned memory, and the token estimate.
  - **Middle:**
    - The run bar (prompt, model, Run).
    - The selected output (latest by default) with its version chip and validation badge.
    - Below the output, its thread, reusing `chat_widgets.dart` (messages, composer, stop).
    - Suggestion cards under the replies with Apply and Dismiss, a before/after view for prompts, and the
      **Suggest improvements** and **New version** buttons in the thread header.
    - The tool log and approvals for agent runs stay in this pane while a run is active.
  - **Outputs (right):** the notebook's outputs, newest first, with version lineage (`v2 ← v1`), validation state
    and a scheduled marker. Selecting one loads its output and thread.
- Narrow windows collapse the side panes into toggles.
- `CoreClient`, the demo client and the HTTP client get the new calls. The tests cover the thread, suggestion
  apply and dismiss, new version, and the fallback button.

## Constraints

- OS independent. There is no OS-specific code.
- Bound everything: thread size and reply caps as in chat, suggestion counts and sizes above, one extra tool round.
- Secrets: no keys in threads, suggestions or audit entries. Redact before storing.
- Quality gates as in `CLAUDE.md`. Fuzz the suggestion parser (tool arguments and structured list).
- Docs: `docs/api.md`, `docs/architecture.md` and `docs/checklist.md`. Add a story ID (for example `OUT-1` to
  `OUT-4`) to the spec in `docs/README.md`.

## Left to the builder

- How the chat store is generalised (a root option or a small wrapper) and how output IDs are validated as thread
  keys.
- Exact wording of the system prompt and tool descriptions.
- The precise priority order for trimming context in a thread.
- CLI flag names. The desktop widget split, as long as no file exceeds 500 lines.
