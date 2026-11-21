# Output Threads Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Each output in a notebook gets its own conversation. From it the user can ask about the output, create a
new validated version, and apply suggested memory or prompt changes to the notebook.

**Architecture:** Threads reuse `internal/chat`: the same Session, Message, Build and streaming code, with a store
keyed by output ID inside the notebook folder. Suggestions are a notebook record and come from tool calls
(`suggest_memory` and `suggest_prompt`) or from one structured call. New versions go through
`executor.Pipeline`, using a new `Spec.Revise` that carries the parent output and the revision requests. The
service owns all logic. The server, CLI and desktop are thin layers over it.

**Tech Stack:** Go standard library, the existing `internal/llm` tool calling, Flutter (material_ui) for the
desktop.

**Spec:** `docs/superpowers/specs/2026-11-21-output-threads-design.md`

## Global Constraints

- OS independent. There is no `runtime.GOOS`, no build tags and no shell tools.
- Files are written 0600 and folders 0700 (threads and suggestions), and every write is atomic.
- Secrets never go in threads, suggestions or audit entries. Run `secrets.RedactPatterns` on suggestion text before
  storing it.
- Caps:
  - At most 3 suggestions per reply and at most 20 open suggestions per thread.
  - A memory suggestion is at most `service.MaxMemoryItem` (1000) characters.
  - A prompt suggestion is at most 16 KiB.
  - At most one extra model round per reply.
  - Threads keep the existing chat caps (`chat.MaxMessages` and `chat.MaxSessionFile`).
- Gates:
  - Lines are at most 120 characters, files at most 500 lines (tests at most 800).
  - funlen 80, gocyclo 15.
  - Coverage: Go at least 80%, desktop at least 80%.
  - The suggestion parser gets a fuzz test.
- Commits use `git -c user.name=Claude -c user.email=noreply@anthropic.com commit` and every message ends with
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Commit after each task. Push only when the user asks.
- Docs change in the same task as the code they describe: `docs/api.md`, `docs/architecture.md` and
  `docs/checklist.md` (stories OUT-1 to OUT-4).

## Review Focus

1. **Deleted output:** a thread, suggestions or a new version for an output that has been deleted, or whose run
   record is missing, returns `404` with the cause, never a panic. Covered in Task 3 and Task 5.
2. **Two applies racing:** applying a stale prompt suggestion while another apply has just replaced that prompt
   returns `409`, and the second change is not lost. Covered in Task 4.
3. **Hostile tool arguments:** tool calls with oversized, non-JSON, empty or unknown-kind arguments are dropped and
   the reply still finishes. Covered in Task 3 (including a fuzz test).
4. **Busy thread:** a second message, or New version, while a reply streams in the same thread returns `409`.
   Covered in Task 3 and Task 6.
5. **Model without tools:** the reply is retried without tools and still succeeds, and Suggest improvements works.
   Covered in Task 3.

---

### Task 1: Thread store and record fields (OUT-1 groundwork)

**Files:**
- Modify: `internal/chat/store.go`, `internal/chat/chat.go`, `internal/chat/prompt.go`
- Modify: `internal/notebook/notebook.go`, `internal/notebook/records.go`
- Test: `internal/chat/chat_test.go`, `internal/notebook/notebook_test.go` (or the existing records test file)

**Interfaces:**
- Produces:
  - `chat.OpenKeyed(dir string, valid func(string) bool) (*Store, error)`: sessions are stored at
    `dir/<key>.json` with no `sessions/` subfolder, and the key is checked by `valid`. `Get`, `Update`,
    `Acquire` and `Delete` use the store's validator; `Open` keeps `ValidID`.
  - `(*Store).Ensure(key, model, effort string) (Session, error)`: returns the session, creating an empty one with
    `ID = key` if it is missing.
  - `chat.RoleNote = "note"`: a stored message that `Build` never sends to the model.
  - `notebook.ValidID(id string) bool` matches `^\d{8}T\d{6}-[0-9a-f]{6}$` (the `NewID` format).
  - `notebook.ThreadsDir = "threads"` and `notebook.SuggestionsDir = "suggestions"`, created by `Create` and
    created lazily for older notebooks (0700).
  - `Output` gains `ParentID string \`json:"parentId,omitempty"\`` and
    `FromThread string \`json:"fromThread,omitempty"\``.
  - `Run` gains `PromptName string \`json:"promptName,omitempty"\``.
  - `(*Notebook).GetOutput(id string) (Output, string, error)`: returns the output and its markdown, or an error
    wrapping `fs.ErrNotExist` when it is missing. The ID is checked with `ValidID` before any path is built.

- [ ] **Step 1: Write the failing tests**
  - `TestKeyedStoreEnsureAndGet`:
    - `Ensure("20261121T100000-abcdef", "m", "")` twice returns the same session.
    - The file exists at `dir/20261121T100000-abcdef.json`.
    - `Get("../x")` returns `ErrNotFound`.
  - `TestBuildSkipsNotes`: a session with a `RoleNote` message produces no `llm.Message` for it.
  - `TestGetOutputRejectsBadID`: `GetOutput("../outputs/x")` errors without touching the disk.
  - `TestGetOutputMissing`: `errors.Is(err, fs.ErrNotExist)`.
  - `TestOutputParentRoundTrip`: `ParentID` and `FromThread` survive `SaveOutput` → `GetOutput`.
- [ ] **Step 2: Run** `go test ./internal/chat ./internal/notebook`. Expected: FAIL (undefined symbols).
- [ ] **Step 3: Implement.** Give `Store` unexported `flat bool` and `valid func(string) bool` fields. `path(id)`
  and the validity checks branch on them.
- [ ] **Step 4: Run** `go test -race ./internal/chat ./internal/notebook`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(chat,notebook): keyed thread store, note messages, output lineage fields`

### Task 2: Suggestion records (OUT-3 groundwork)

**Files:**
- Create: `internal/notebook/suggestions.go`
- Test: `internal/notebook/suggestions_test.go`

**Interfaces:**
- Produces:
  ```go
  type Suggestion struct {
      ID        string `json:"id"`        // NewID()
      Kind      string `json:"kind"`      // SuggestMemory | SuggestPrompt
      Text      string `json:"text"`
      Reason    string `json:"reason,omitempty"`
      MessageID string `json:"messageId,omitempty"` // thread message that produced it
      Base      string `json:"base,omitempty"`      // sha256 hex of the target prompt text when made (prompt only)
      Status    string `json:"status"`              // SuggestOpen | SuggestApplied | SuggestDismissed
      CreatedAt string `json:"createdAt"`
      DecidedAt string `json:"decidedAt,omitempty"`
  }
  const (SuggestMemory = "memory"; SuggestPrompt = "prompt"
         SuggestOpen = "open"; SuggestApplied = "applied"; SuggestDismissed = "dismissed")
  const MaxOpenSuggestions = 20; const MaxPromptText = 16 << 10
  func (s Suggestion) Check(maxMemoryItem int) error // kind known, text non-empty valid UTF-8, size caps
  func (n *Notebook) Suggestions(outputID string) ([]Suggestion, error)        // missing file → empty
  func (n *Notebook) UpdateSuggestions(outputID string, edit func([]Suggestion) ([]Suggestion, error)) error
  ```
- `UpdateSuggestions` serializes per notebook folder with a package-level mutex map, like `chat.locks`. It rejects
  a result with more than `MaxOpenSuggestions` open items, and the file is capped at 1 MiB on read.

- [ ] **Step 1: Write the failing tests**
  - `TestSuggestionCheck`: table cases for an empty text, an unknown kind, a memory item of 1001 characters, a
    prompt over 16 KiB and invalid UTF-8 (each an error), plus two valid cases.
  - `TestUpdateSuggestionsCapsOpen`: adding the 21st open suggestion errors, and the file is unchanged.
  - `TestUpdateSuggestionsConcurrent`: 10 goroutines each append one suggestion, and the result has 10.
  - `TestSuggestionsBadOutputID`: errors.
- [ ] **Step 2: Run** `go test ./internal/notebook -run Suggest`. Expected: FAIL.
- [ ] **Step 3: Implement** in `suggestions.go`.
- [ ] **Step 4: Run** `go test -race ./internal/notebook`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(notebook): suggestion records with caps and serialized updates`

### Task 3: Thread replies with suggestion tools (OUT-1, OUT-3)

**Files:**
- Create: `internal/service/thread.go`, `internal/service/thread_tools.go`
- Test: `internal/service/thread_test.go`, `internal/service/thread_fuzz_test.go`

**Interfaces:**
- Consumes: Task 1 (`OpenKeyed`, `Ensure`, `GetOutput`, `RoleNote`) and Task 2 (`Suggestion`,
  `UpdateSuggestions`).
- Produces:
  ```go
  func (s *Service) threadStore(nb *notebook.Notebook) (*chat.Store, error)
  func (s *Service) GetThread(notebookID, outputID string) (chat.Session, error)
  func (s *Service) SetThreadSettings(notebookID, outputID, model, effort string) (chat.Session, error)
  func (s *Service) CheckThreadSend(notebookID, outputID string, in ChatSend) error
  type ThreadReply struct {
      ChatReply
      Suggestions []notebook.Suggestion `json:"suggestions"`
  }
  func (s *Service) SendThread(ctx context.Context, notebookID, outputID string, in ChatSend,
      onDelta func(string), onSuggestion func(notebook.Suggestion)) (ThreadReply, error)
  func parseSuggestionCall(c llm.ToolCall) (notebook.Suggestion, error) // thread_tools.go
  var suggestionTools []llm.ToolDef                                      // suggest_memory, suggest_prompt
  ```
- How a reply is built:
  - The model settings resolve app → notebook → thread (session `Model` and `ThinkingEffort`).
  - The documents, in this order:
    1. the output markdown, as an `extract.Document` with `Source: "output"`;
    2. the run's prompt, as `Source: "prompt"`;
    3. the notebook's enabled context (`notebookDocs`);
    4. the message's attachments (`resolveAttachments`).
  - The memory passed to `chat.Build` is the notebook's `Manifest.Memory`. Global chat memory is not used.
  - The request has `Tools: suggestionTools`.
- After the model answers:
  - If the result has tool calls, parse each one. Store at most 3 valid ones (with redacted text; for a prompt,
    `Base` is the target prompt's digest from Task 4's `promptTarget`) and call `onSuggestion` for each.
  - Then make one more call with the assistant tool-call message, a tool result of `recorded` for each call, and
    no tools. The text from both rounds is streamed and stored.
  - `llm.ErrToolsUnsupported` retries once without tools.
- Audit kind `thread.message`, with digests only, like `auditChat`.
- Errors:
  - A missing notebook, output or run → `ErrNotFound`.
  - A busy thread → `chat.ErrBusy`, which maps to `409` through `chatErr`.

- [ ] **Step 1: Write the failing tests.** Use the existing fake model factory pattern from `chat_test.go`.
  - `TestSendThreadUsesOutputAndMemory`: the first request's messages contain the output markdown text and a
    notebook memory fact, and they do not contain the chat memory fact.
  - `TestSendThreadStoresToolSuggestions`:
    - The fake returns a `suggest_memory` call with `{"text":"cite file paths","reason":"r"}` and then the text
      `ok`.
    - The reply content is `ok`.
    - `Suggestions` holds one open memory suggestion.
    - The second request has no tools and includes a tool message `recorded`.
  - `TestSendThreadDropsBadCalls`: calls with an unknown name, non-JSON arguments, empty text and a fourth valid
    call give exactly 3 stored suggestions, and the reply still finishes.
  - `TestSendThreadToolsUnsupported`: the first call returns `llm.ErrToolsUnsupported` and the retry without
    tools succeeds.
  - `TestSendThreadBusy`: a second `SendThread` while the first holds the lock returns `chat.ErrBusy`.
  - `TestSendThreadMissingOutput`: `errors.Is(err, ErrNotFound)`.
  - `TestSendThreadRedactsSuggestion`: a suggestion containing a configured key value is stored redacted.
  - `FuzzParseSuggestionCall`: arbitrary names and arguments never panic, and every accepted suggestion passes
    `Check`.
- [ ] **Step 2: Run** `go test ./internal/service -run Thread`. Expected: FAIL.
- [ ] **Step 3: Implement** `thread.go` (the store, Get, Set, Check and Send, split so funlen stays at most 80) and
  `thread_tools.go` (tool defs, the parser and storing).
- [ ] **Step 4: Run** `go test -race ./internal/service` and `go test -run=^$ -fuzz=FuzzParseSuggestionCall
  -fuzztime=20s ./internal/service`. Expected: PASS, with no crashers.
- [ ] **Step 5: Commit** `feat(service): output threads with suggest_memory/suggest_prompt tools`

### Task 4: Generate, list, apply and dismiss suggestions (OUT-3)

**Files:**
- Create: `internal/service/suggestions.go`
- Test: `internal/service/suggestions_test.go`

**Interfaces:**
- Consumes: Tasks 2 and 3.
- Produces:
  ```go
  type PromptTarget struct {
      Kind       string   `json:"kind"`                 // "saved" | "schedule" | "new"
      PromptName string   `json:"promptName,omitempty"` // saved
      ScheduleID string   `json:"scheduleId,omitempty"` // schedule (inline prompt)
      Schedules  []string `json:"schedules,omitempty"`  // saved: IDs of schedules using it
      Current    string   `json:"current"`              // current prompt text (for before/after)
  }
  type SuggestionInfo struct {
      notebook.Suggestion
      Target *PromptTarget `json:"target,omitempty"` // prompt suggestions only
  }
  func (s *Service) promptTarget(nb *notebook.Notebook, run notebook.Run) (PromptTarget, error)
  func (s *Service) ListSuggestions(notebookID, outputID string) ([]SuggestionInfo, error)
  func (s *Service) GenerateSuggestions(ctx context.Context, notebookID, outputID string) ([]SuggestionInfo, error)
  type ApplyResult struct {
      Suggestion SuggestionInfo `json:"suggestion"`
      PromptName string         `json:"promptName,omitempty"` // set when a new saved prompt was created
  }
  func (s *Service) ApplySuggestion(notebookID, outputID, suggestionID string) (ApplyResult, error)
  func (s *Service) DismissSuggestion(notebookID, outputID, suggestionID string) (SuggestionInfo, error)
  ```
- `promptTarget` resolution, in order:
  1. `run.PromptName` → `saved`.
  2. Otherwise `run.ScheduleID`. If that schedule has a `PromptName` → `saved`; otherwise → `schedule`.
  3. Otherwise → `new`.
  - `Schedules` lists every schedule whose `PromptName` equals the saved name.
- `GenerateSuggestions`:
  - One `llm.Request` with `Format` set to the schema
    `{"type":"object","required":["suggestions"],"properties":{"suggestions":{"type":"array","maxItems":3,
    "items":{"type":"object","required":["kind","text"],"properties":{"kind":{"enum":["memory","prompt"]},
    "text":{"type":"string"},"reason":{"type":"string"}}}}}}`.
  - Validated through `schema`. Results are stored like tool suggestions.
- `ApplySuggestion`:
  - Memory: appends through `SetMemory`, whose limits give `InvalidError` → `422`; a full memory gives `409`.
  - Prompt:
    - Return `409` (`ErrConflict`) if `sha256(target.Current)` is not `Base`.
    - Return `422` if `executor.Variables(text)` includes a name that is not in the run's `Vars` (or the
      schedule's `Vars`).
    - `saved` → `SavePrompt` with the same name and settings. `schedule` → `SaveSchedules` with that schedule's
      `Prompt` replaced. `new` → `SavePrompt{Name: "refined-<n>"}`, using the lowest free n starting at 1.
  - Then the status becomes `applied` and the audit entry `suggestion.apply` records the IDs and kind.
  - An already applied or dismissed suggestion → `409`.
  - The whole apply runs under the notebook's suggestion lock, so two applies cannot interleave.

- [ ] **Step 1: Write the failing tests**
  - `TestPromptTargetSaved`, `TestPromptTargetScheduleInline`, `TestPromptTargetScheduleNamed` and
    `TestPromptTargetNew`: each checks `Kind`, the name or ID, and `Schedules` (`TestPromptTargetSaved` lists
    the 2 schedules using it).
  - `TestApplyMemory`: memory gains the text and the status is `applied`.
  - `TestApplyPromptSavedUpdatesSharedPrompt`: the saved prompt text changes and its settings are unchanged.
  - `TestApplyPromptScheduleInline`: the schedule's `Prompt` changes and nothing else does.
  - `TestApplyPromptNewName`: with `refined-1` already present, the new prompt is `refined-2`.
  - `TestApplyStaleIs409`: the prompt is edited after the suggestion is made, the apply returns
    `errors.Is(err, ErrConflict)`, and the prompt is unchanged.
  - `TestApplyNewVariableIs422`: the text adds `{{team}}` when the run vars lack it, and `IsInvalid(err)` holds.
  - `TestApplyTwiceIs409`.
  - `TestDismiss`.
  - `TestGenerateSuggestionsStoresValid`: the fake returns 2 valid items and they are stored open.
  - `TestGenerateSuggestionsInvalidJSON`: the call returns an error wrapping `llm.ErrInvalidOutput`, and nothing is
    stored.
- [ ] **Step 2: Run** `go test ./internal/service -run 'Suggest|Apply|PromptTarget|Dismiss'`. Expected: FAIL.
- [ ] **Step 3: Implement** `suggestions.go`. Record `PromptName` on runs: in `Service.Run`, pass a decorate func
  to `saveRun` that sets `run.PromptName = req.PromptName`.
- [ ] **Step 4: Run** `go test -race ./internal/service`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(service): suggest, apply and dismiss notebook improvements`

### Task 5: New version (OUT-2)

**Files:**
- Modify: `internal/executor/executor.go`, `internal/executor/pipeline.go`
- Modify: `internal/service/records.go`, `internal/service/workspace.go`
- Create: `internal/service/versions.go`
- Test: `internal/executor/pipeline_test.go`, `internal/service/versions_test.go`

**Interfaces:**
- Produces:
  ```go
  // executor
  type Revision struct {
      Parent    json.RawMessage // the output being revised
      Requests  string          // the thread conversation, labelled as revision requests
      ParentID  string
      MessageID string
  }
  // Spec gains: Revise *Revision
  // service
  func (s *Service) NewVersion(ctx context.Context, notebookID, outputID, fromMessageID string) (RunOutcome, error)
  // OutputInfo gains: Version int `json:"version"` (1 + number of ancestors via ParentID; cycles and missing
  // parents stop the walk)
  ```
- Pipeline:
  - When `spec.Revise != nil`, the reduce step's user message is `userMessage(...)` followed by
    `"\n\nDocument to revise (JSON):\n" + parent + "\n\nRevision requests:\n" + requests + "\n\nReturn the full
    revised document. Change only what the requests ask for."`.
  - `budget` is reduced by `extract.EstimateTokens(parent+requests)` before the map-reduce decision, with a floor
    of 1/4 of the budget.
- `saveRun` sets `Output.ParentID` and `Output.FromThread` from `spec.Revise`.
- `NewVersion`:
  - Loads the parent output and its run (`ErrNotFound` if missing). `fromMessageID` defaults to the latest
    assistant message. An unknown ID or an empty thread → `InvalidError`.
  - Requests are the thread's user and assistant turns up to and including that message, as `role: text` lines,
    capped at 32 KiB from the end.
  - Builds the spec with `buildSpec(RunRequest{Notebook: nb, PromptName: run.PromptName, Prompt: run.Prompt,
    Vars: run.Vars, Save: true})` and forces `Settings.Mode = config.ModePipeline`.
  - Runs the pipeline with the same audit, save and finish steps as `Service.Run`.
  - Holds the thread's `Acquire` lock while it runs (busy → `chat.ErrBusy`).
  - On success, appends a `RoleNote` "Saved as v<N> (<outputID>)" to the parent thread.

- [ ] **Step 1: Write the failing tests**
  - `TestPipelineReviseIncludesParent`: the last request contains the parent JSON and "Revision requests:".
  - `TestPipelineReviseBudget`: with a budget of 1000 and a 700-token parent, documents of 400 tokens trigger
    map-reduce.
  - `TestNewVersionSavesLinkedOutput`: the new output has `ParentID` set to the parent and `FromThread` set to the
    message ID, `GetOutput` shows `Version` 2, the parent file is byte-identical, and the parent thread ends with a
    note.
  - `TestNewVersionInvalidSavesNothing`: the fake returns invalid JSON 3 times, the error wraps
    `llm.ErrInvalidOutput`, and the output count is unchanged.
  - `TestNewVersionAgentNotebookUsesPipeline`: with the notebook's `Mode` set to agent, there are no tool calls in
    the requests and `RunOutcome.Paused` is empty.
  - `TestNewVersionEmptyThread`: `IsInvalid(err)`.
  - `TestNewVersionMissingRun`: `errors.Is(err, ErrNotFound)`.
  - `TestVersionChainCycle`: two outputs whose `ParentID`s point at each other give a finite `Version`.
- [ ] **Step 2: Run** `go test ./internal/executor ./internal/service -run 'Revise|Version'`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/executor ./internal/service`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(service): new output versions revise the parent through the pipeline`

### Task 6: HTTP API (OUT-1 to OUT-3)

**Files:**
- Create: `internal/server/threads.go`
- Modify: `internal/server/server.go` (routes), `docs/api.md`
- Test: `internal/server/threads_test.go`

**Interfaces:**
- Consumes: Tasks 3 to 5.
- Routes (prefix `/v1/notebooks/{id}/outputs/{out}`):
  - `GET /thread` and `PATCH /thread` (`{model?, thinkingEffort?}`).
  - `POST /thread/messages`: SSE with `delta {text}`, `suggestion SuggestionInfo`, `finished ThreadReply`, and
    `error {status, error, reply}`, like `sendChat`. It takes a slot from `s.runs`.
  - `GET /suggestions`.
  - `POST /suggestions/generate` (takes a slot from `s.runs`).
  - `POST /suggestions/{sid}/apply` and `POST /suggestions/{sid}/dismiss`.
  - `POST /versions` (`{fromMessageId?}`): `201 OutputInfo`, `422` when invalid, and it takes a slot from `s.runs`.
- `GET /v1/outputs` and the output detail include `parentId` and `version`.

- [ ] **Step 1: Write the failing tests** (`httptest`, fake model):
  - `TestThreadMessageStreams`: the events are `delta`, `suggestion` and `finished`, in order.
  - `TestThreadBusy409`.
  - `TestApplyStale409`.
  - `TestApplyNewVar422`.
  - `TestVersions201And422`.
  - `TestThreadUnknownOutput404`.
  - `TestOutputsListHasParentAndVersion`.
- [ ] **Step 2: Run** `go test ./internal/server -run 'Thread|Apply|Versions|Outputs'`. Expected: FAIL.
- [ ] **Step 3: Implement** the handlers. Document the routes, bodies, events and errors in `docs/api.md`.
- [ ] **Step 4: Run** `go test -race ./internal/server`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(server): output thread, suggestion and version routes`

### Task 7: CLI `tess output` (OUT-1 to OUT-3)

**Files:**
- Create: `internal/cli/cmd_output.go`
- Modify: `internal/cli/cli.go` (commands map and usage)
- Test: `internal/cli/output_test.go`

**Interfaces:**
- Subcommands, each with `--notebook DIR` as for `tess run`:
  - `tess output ask <outputId> "question"` streams the reply and then prints `suggestion <id> <kind>: <text>`
    lines.
  - `tess output suggest <outputId>`.
  - `tess output suggestions <outputId>` prints a table: ID, KIND, STATUS, TARGET, TEXT (truncated to 60).
  - `tess output apply <outputId> <suggestionId>`.
  - `tess output dismiss <outputId> <suggestionId>`.
  - `tess output refine <outputId> [--from MESSAGE_ID]` prints the new output ID and its version.
- Exit codes:
  - Usage errors: `ExitUsage`.
  - `409` and `422` errors: the existing `cliErr` mapping.

- [ ] **Step 1: Write the failing tests:**
  - `TestOutputAskPrintsSuggestions`.
  - `TestOutputApplyAndDismiss`.
  - `TestOutputRefinePrintsVersion`.
  - `TestOutputUsage`: no subcommand, or an unknown one, gives `ExitUsage`.
- [ ] **Step 2: Run** `go test ./internal/cli -run Output`. Expected: FAIL.
- [ ] **Step 3: Implement.** Update the usage text and the CLI section of `docs/checklist.md`.
- [ ] **Step 4: Run** `go test -race ./internal/cli`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(cli): tess output ask, suggest, apply, dismiss and refine`

### Task 8: Desktop client layer (OUT-4)

**Files:**
- Modify: `desktop/lib/src/core/models.dart` (or create `desktop/lib/src/core/thread_models.dart`)
- Modify: `desktop/lib/src/core/core_client.dart` (interface and demo), `desktop/lib/src/core/http_core_client.dart`,
  `desktop/lib/src/core/json_mapping.dart`
- Test: `desktop/test/thread_client_test.dart`

**Interfaces:**
- Produces:
  ```dart
  class PromptTarget { final String kind; final String? promptName; final String? scheduleId;
    final List<String> schedules; final String current; }
  class Suggestion { final String id, kind, text, status; final String? reason; final PromptTarget? target; }
  sealed class ThreadEvent; ThreadDelta(String text); ThreadSuggestion(Suggestion s); ThreadFinished(ChatMessage reply)
  // OutputSummary gains: String? parentId; int version
  // CoreClient gains:
  Future<ChatSession> getThread(String notebookId, String outputId);
  Future<ChatSession> setThreadSettings(String notebookId, String outputId, {String? model, String? thinkingEffort});
  Stream<ThreadEvent> sendThread(String notebookId, String outputId, String text, {List<ChatAttachment> attachments});
  Future<List<Suggestion>> listSuggestions(String notebookId, String outputId);
  Future<List<Suggestion>> generateSuggestions(String notebookId, String outputId);
  Future<({Suggestion suggestion, String? promptName})> applySuggestion(String notebookId, String outputId, String id);
  Future<Suggestion> dismissSuggestion(String notebookId, String outputId, String id);
  Future<OutputSummary> newVersion(String notebookId, String outputId, {String? fromMessageId});
  ```
- The demo client seeds one output with a two-message thread and one open prompt suggestion that has a `saved`
  target.

- [ ] **Step 1: Write the failing tests:**
  - JSON mapping for `Suggestion`, including `target`, and for `OutputSummary` (`parentId` and `version`).
  - The HTTP client parses the SSE `delta`, `suggestion` and `finished` events.
  - The HTTP client maps `409` and `422` to `CoreException` with the server message.
  - The demo client applies a suggestion and its status becomes `applied`.
- [ ] **Step 2: Run** `flutter test test/thread_client_test.dart` in `desktop/`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `flutter test` and `flutter analyze --fatal-infos`. Expected: PASS with no issues.
- [ ] **Step 5: Commit** `feat(desktop): core client for output threads, suggestions and versions`

### Task 9: Three-pane notebook page (OUT-4)

**Files:**
- Modify: `desktop/lib/src/screens/workspace_screen.dart` (becomes the layout only)
- Create:
  - `desktop/lib/src/screens/notebook_sources_pane.dart` (context and memory; moved from `_ContextCard` and
    `MemoryCard`)
  - `desktop/lib/src/screens/notebook_outputs_pane.dart`
  - `desktop/lib/src/widgets/output_thread.dart` (output view, thread, composer, suggestion cards)
  - `desktop/lib/src/thread_state.dart`
- Modify: `desktop/lib/src/app_state.dart` (selected output, `ThreadState` per selection)
- Test: `desktop/test/notebook_page_test.dart`, `desktop/test/thread_state_test.dart`

**Interfaces:**
- Consumes: Task 8.
- `ThreadState extends ChangeNotifier`:
  - Fields: `session`, `suggestions`, `streaming`, `streamedText`.
  - Methods: `load()`, `send(text)`, `stop()`, `suggest()`, `apply(id)`, `dismiss(id)`, `newVersion()`.
  - Errors go to `AppState` notices, like `_act`.
- Layout:
  - **Sources** pane: 280 px, on the left.
  - **Middle** pane: the run bar, then the selected output with a `v<N>` chip and the validation badge, then the
    thread. The tool log and approvals stay here during agent runs.
  - **Outputs** pane: 300 px, on the right, newest first, with `v2 ← v1` lineage and a scheduled marker.
  - Below a window width of 1100 px, the side panes collapse behind icon toggles in the header.
- Widget keys: `output-thread`, `thread-composer`, `suggest-improvements`, `new-version`, `suggestion-apply-<id>`,
  `suggestion-dismiss-<id>`, `outputs-pane`, `sources-pane`.
- The prompt suggestion card shows the before and after text, with the line "Updates prompt `<name>`, used by
  <n> schedule(s)" (or "Updates schedule `<id>`", or "Saves new prompt"). The line comes from `target`.
- Reuse `MessageBubble` and `Composer` from `chat_widgets.dart`. `RoleNote` messages render as small muted text.

- [ ] **Step 1: Write the failing widget tests** (demo client):
  - Selecting an output loads its thread.
  - Sending a message shows streamed text.
  - Apply on the seeded suggestion shows "applied" and refreshes memory or the prompt.
  - Dismiss removes the card.
  - New version adds `v2` to the outputs pane and selects it.
  - At 1000 px wide, the sources pane is hidden and toggled by its icon.
- [ ] **Step 2: Run** `flutter test`. Expected: FAIL.
- [ ] **Step 3: Implement.** Keep every file at most 500 lines.
- [ ] **Step 4: Run** `task desktop:check`. Expected: PASS with coverage of at least 80%.
- [ ] **Step 5: Commit** `feat(desktop): three-pane notebook page with output threads and suggestions`

### Task 10: Docs and full gate

**Files:**
- Modify: `docs/architecture.md` (threads, suggestions and versions in the service and notebook layout),
  `docs/checklist.md` (OUT-1 ask about an output, OUT-2 new version, OUT-3 suggestions, OUT-4 desktop page),
  `docs/README.md` (story IDs), and `CLAUDE.md` (layout line, only if a new package was added)

- [ ] **Step 1: Update the docs.** The checklist marks OUT-1 to OUT-4 done.
- [ ] **Step 2: Run** `task ci`. Expected: all gates green (fmt, lengths, text, lint, tests with race, Go coverage
  of at least 80%, fuzz, vuln, and the desktop checks).
- [ ] **Step 3: Commit** `docs: output threads, suggestions and versions`
