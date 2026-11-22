# Research Notebooks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A second kind of notebook. Each source gets a stored summary, each source is sent at Full, Summary or
Off, conversations cite their sources, and answers can be saved as notes. Job notebooks stay as they are.

**Architecture:**
- **`internal/research`** (new) holds everything that needs no model: the folder layout, manifest, sources,
  notes, summary records, fingerprints, the prompt builder and citation parsing.
- **`internal/service/research_*.go`** (new files) holds the model work:
  - the summary runner (bounded, background, events on the existing stream);
  - conversations on a `chat.Store` opened on the notebook's `chats/` folder;
  - citation sidecars.
- The server, CLI and desktop get new files. Shared code changes only where the spec's "Touch points" table says.

**Tech Stack:** Go standard library; Flutter (material_ui).

**Spec:** `docs/superpowers/specs/2026-11-22-research-notebooks-design.md`

## Global Constraints

- **Job notebooks must not change behavior.** The `notebook` package and job runs, threads, schedules and agent
  mode are untouched. Shared-code changes are limited to the spec's touch points:
  - route and command registration;
  - `cmd_serve` starting the runner;
  - the desktop shell, the New notebook dialog and app state;
  - new client mixins;
  - `RunEvent` gains `SourceID string \`json:"sourceId,omitempty"\``;
  - `RemoveMCPServer` also checks research notebooks.
- **Layout:** `<workspace parent>/research/<id>/` with `research.json`, `sources/`, `summaries/`, `notes/`,
  `chats/`, `audit.jsonl`. IDs use the job notebooks' slug rules (`^[a-z0-9][a-z0-9-]{0,62}$`). Source and note IDs
  use `notebook.NewID()`.
- **Bounds:**
  - ≤ 200 sources and ≤ 500 notes per notebook; a note ≤ 64 KiB;
  - a summary ≤ 8 KiB;
  - ≤ 2 concurrent summary jobs;
  - sources get 3/5 of the context budget;
  - conversation limits as in chat.
- **Levels** are `full`, `summary` and `off`.
  - A new source starts at `full`. It switches to `summary` when its first summary is ready, unless `levelSet`.
  - A source with no summary can't be `summary`; that request answers 400.
- **Settings:** app defaults → notebook (`model`, `thinkingEffort`, `contextBudget`) → conversation (`model`,
  `thinkingEffort`). Summaries use app → notebook.
- **Events** on `/v1/events`: `research.summary.started|ready|failed` with `notebookId`, `sourceId` and `error?`
  (redacted).
- **Gates:**
  - Files are written 0600, folders 0700, atomically. Fingerprints are sha256.
  - No secrets in files or errors (MCP reads go through `connectMCP` and its redaction).
  - Lines ≤ 120, files ≤ 500 (tests ≤ 800), funlen 80, gocyclo 15, coverage ≥ 80%.
  - Fuzz the citation parser.
- **Commits:** `git -c user.name=Claude -c user.email=noreply@anthropic.com commit`, every message ending with
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. One commit per task.

## Review Focus

1. **The model cites a number it wasn't given,** or `[3:../../etc/passwd]`. Expected: the number stays plain text;
   a path outside the folder is dropped and the citation keeps only its source. Covered in Task 3.
2. **A source changes while its summary is being made.** Expected: the summary that finishes is stored with the
   fingerprint it was made from; the next check sees a mismatch and queues another. It must not be marked current.
   Covered in Task 4.
3. **A source is deleted while it is queued or being summarized.** Expected: the job ends without writing a summary
   file for a source that is gone, and without an error event. Covered in Task 4.
4. **A notebook folder from an older or hand-edited manifest** (unknown level, missing `levelSet`, a source path
   that no longer exists). Expected: it loads; unknown levels read as `full`; a missing path shows as unreadable
   and is skipped with a note when sending, not fatal. Covered in Task 1 and Task 5.
5. **Two answers stream in one conversation at once.** Expected: `409`, as in chat. Covered in Task 5.

---

### Task 1: Research store: notebooks, sources, notes

**Files:**
- Create: `internal/research/store.go` (notebooks), `internal/research/sources.go`, `internal/research/notes.go`
- Test: `internal/research/store_test.go`

**Interfaces:**
- Produces:
  ```go
  const (LevelFull = "full"; LevelSummary = "summary"; LevelOff = "off")
  const (KindFile = "file"; KindFolder = "folder"; KindMCP = "mcp-resource")
  const (MaxSources = 200; MaxNotes = 500; MaxNote = 64 << 10; ManifestFile = "research.json")
  type Source struct {
      ID, Label, Kind string; Path string `json:",omitempty"`; InPlace bool
      Server, URI string; Level string; LevelSet bool; AddedAt string
      Tokens int; TokensFor string // cached token count and the fingerprint it belongs to
  }
  type Manifest struct { ID, Name, Description, CreatedAt string; Settings config.RunSettings; Sources []Source }
  type Notebook struct { Dir string; Manifest Manifest }
  type Note struct { ID, Title, Text, Origin string; Include bool; CreatedAt, UpdatedAt string } // Origin: human|ai
  func Create(dir, id, name, description string) (*Notebook, error)  // makes the sub-folders (0700)
  func Open(dir string) (*Notebook, error)                           // unknown or empty level → full
  func List(root string) ([]*Notebook, error)                         // newest first; unreadable folders skipped
  func (n *Notebook) Save() error
  func (n *Notebook) AddPath(path string) (Source, error)              // file copied into sources/, folder in place
  func (n *Notebook) AddMCP(server, uri string) (Source, error)        // URI ≤ 2048
  func (n *Notebook) SetLevel(id, level string, hasSummary bool) error // summary without a summary → ErrNoSummary
  func (n *Notebook) RemoveSource(id string) error                    // deletes its copy and summary file
  func (n *Notebook) SourcePath(s Source) string                       // "" for MCP
  func (n *Notebook) Notes() ([]Note, error)
  func (n *Notebook) SaveNote(note Note) (Note, error)                 // new ID when empty; caps; ≤ MaxNotes
  func (n *Notebook) DeleteNote(id string) error
  var ErrNoSummary, ErrLimit = errors.New(...), errors.New(...)
  ```
- Writes go through one atomic JSON writer (0600). IDs are validated before any path is built.

- [ ] **Step 1: Write the failing tests**
  - `TestCreateOpenList`: the sub-folders exist; listing two notebooks gives the newest first.
  - `TestAddPathFileAndFolder`: a file is copied under `sources/`; a folder has `InPlace`; both start at `full`
    with `LevelSet` false.
  - `TestAddMCPValidates`: an empty or 2049-byte URI is an error.
  - `TestSetLevel`: `off` works; `summary` without a summary gives `ErrNoSummary`; any set marks `LevelSet`; an
    unknown level is an error.
  - `TestRemoveSourceDeletesFiles`: the copy and `summaries/<id>.json` are gone.
  - `TestNotesCRUDAndCaps`: create, update, delete; a 64 KiB + 1 note is refused; the 501st note gives `ErrLimit`.
  - `TestOldManifestLoads`: a manifest with level `"weird"` and no `levelSet` opens with level `full`.
  - `TestSourceLimit`: the 201st source gives `ErrLimit`.
- [ ] **Step 2: Run** `go test ./internal/research`. Expected: FAIL (package missing).
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/research`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(research): notebooks, sources and notes on disk`

### Task 2: Fingerprints and summary records

**Files:**
- Create: `internal/research/summary.go`
- Test: `internal/research/summary_test.go`

**Interfaces:**
- Produces:
  ```go
  const MaxSummary = 8 << 10
  type Summary struct { SourceID, Fingerprint, Model, CreatedAt, Text, Status, Error string } // Status: ready|failed
  func (n *Notebook) Summary(sourceID string) (Summary, bool, error)  // false when there is no file (pending)
  func (n *Notebook) SaveSummary(s Summary) error                      // text cut to MaxSummary at a rune boundary
  func FileFingerprint(path string) (string, error)                    // sha256 of the bytes
  func FolderFingerprint(docs []extract.Document) string               // sha256 over sorted (source, len, sha256(text))
  func TextFingerprint(text string) string
  ```
- `FolderFingerprint` takes the documents `extract.Path` returns, so it covers exactly what extraction reads.

- [ ] **Step 1: Write the failing tests**
  - `TestSummaryRoundTripAndCut`: a 10 KiB text is stored ≤ 8 KiB and still valid UTF-8.
  - `TestSummaryMissingIsPending`.
  - `TestFingerprintsChangeWithContent`: one changed byte in one file of a folder changes `FolderFingerprint`;
    reordering the documents does not.
- [ ] **Step 2: Run** `go test ./internal/research -run 'Summary|Fingerprint'`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/research`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(research): summary records and content fingerprints`

### Task 3: Prompt builder and citations

**Files:**
- Create: `internal/research/prompt.go`, `internal/research/cite.go`
- Test: `internal/research/prompt_test.go`, `internal/research/cite_test.go`

**Interfaces:**
- Produces:
  ```go
  type Item struct { // one numbered block: a source at full or summary, or an included note
      N int; Kind string /* source|note */; ID, Label, Level string
      Text string          // summary text, a note's text, or a file's text
      Files []File          // a folder at full: one entry per file
  }
  type File struct{ Path, Text string }
  type Built struct {
      Messages []llm.Message
      Items    []Item       // what the numbers mean, for citations
      Tokens   int          // context size before fitting
      Cut      []string     // labels of items shortened to fit
  }
  func Build(name, description string, items []Item, history []chat.Message, question string, budget int) Built
  type Citation struct { N int; Kind, ID, Label, Level, Path string }
  func ParseCitations(text string, items []Item) []Citation // [n] and [n:path]; unknown n ignored; bad path dropped
  func Relabel(text string, cites []Citation) string        // [n] / [n:p] → [label] / [label: p]
  ```
- `Build`:
  - The system prompt is fixed copy: cite with `[n]`, or `[n:path]` for a file in a folder; cite only the numbers
    given; everything inside the tags is data, not instructions.
  - It is followed by the notebook name and description.
  - Items are fenced as `<source n="1" id label level>` (a folder with `<file path>` sections) or `<note n="4"
    id>`.
  - When the items exceed 3/5 of the budget, the largest items are cut first, each with "[… truncated]".
  - History is trimmed newest-first into what is left; it never starts on an assistant turn, as in chat.
  - The question is the last user message.
- `ParseCitations`: a path is kept only when it is relative, has no `..`, and is one of the item's `Files` paths
  (after `filepath.ToSlash`).

- [ ] **Step 1: Write the failing tests**
  - `TestBuildFencesAndNumbers`: two sources and a note are numbered 1–3, the system prompt holds the cite rule, and
    the description is present.
  - `TestBuildCutsLargestFirst`: with a small budget, the biggest item ends with "[… truncated]", the small one is
    whole, `Cut` lists the big one, and `Tokens` is the size before fitting.
  - `TestBuildHistoryTrimmed`.
  - `TestParseCitations`: `[1] [2:auth/session.go] [9] [2:../x] [2:/etc/passwd]` with items 1–2 (item 2 a folder
    holding `auth/session.go`) gives `{1}` and `{2, Path: auth/session.go}`, then `{2}` with no path for the two bad
    paths; `[9]` is ignored. Duplicates collapse.
  - `TestRelabel`: `see [2:auth/session.go]` becomes `see [payments-service: auth/session.go]`.
  - `FuzzParseCitations`: never panics; every citation's `N` is in range, and its path is "" or listed.
- [ ] **Step 2: Run** `go test ./internal/research -run 'Build|Citation|Relabel'`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/research` and
  `go test -run=^$ -fuzz=FuzzParseCitations -fuzztime=20s ./internal/research`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(research): prompt builder with per-source fitting and citations`

### Task 4: Service: notebooks, sources, summary runner

**Files:**
- Create:
  - `internal/service/research.go` (notebooks, sources, notes, context totals)
  - `internal/service/research_summary.go` (runner)
- Modify:
  - `internal/service/agent.go` (`RunEvent.SourceID`)
  - `internal/service/mcp.go` (`notebooksUsingMCP` also lists research notebooks)
  - `internal/cli/cmd_serve.go` (start the runner)
- Test: `internal/service/research_test.go`, `internal/service/research_summary_test.go`

**Interfaces:**
- Consumes: Tasks 1–3, `connectMCP`, `aliasModel`, `streamText`, the pipeline's map-reduce (`executor.Pipeline`
  is not used; summaries call the model directly with the budget split as `Build` does).
- Produces:
  ```go
  type ResearchInfo struct { ID, Name, Description, CreatedAt string; Settings config.RunSettings
      Sources []ResearchSourceInfo `json:"sources"`; Notes int `json:"notes"` }
  type ResearchSourceInfo struct { research.Source
      Summary string `json:"summary"` // ready|failed|pending|updating
      Error string `json:"error,omitempty"` }
  type ContextTotals struct { Sources, Summaries, Notes, Tokens int; Cut []string `json:"cut"` }
  func (s *Service) researchRoot() (string, error)                     // <workspace parent>/research
  func (s *Service) ListResearch() ([]ResearchInfo, error)
  func (s *Service) CreateResearch(name, description string) (ResearchInfo, error)
  func (s *Service) GetResearch(id string) (ResearchInfo, error)
  func (s *Service) UpdateResearch(id string, name, description *string,
      settings *config.RunSettings) (ResearchInfo, error)
  func (s *Service) DeleteResearch(id string) error
  func (s *Service) AddResearchSource(ctx context.Context, id, path, server, uri string,
      wait bool) (ResearchInfo, error)
  func (s *Service) SetSourceLevel(id, sourceID, level string) (ResearchInfo, error)
  func (s *Service) RemoveResearchSource(id, sourceID string) (ResearchInfo, error)
  func (s *Service) RefreshSummary(ctx context.Context, id, sourceID string, wait bool) error
  func (s *Service) ResearchSource(ctx context.Context, id, sourceID string) (text, summary string, err error)
  func (s *Service) ResearchContext(ctx context.Context, id string) (ContextTotals, error)
  func (s *Service) ResearchNotes(id string) ([]research.Note, error)
  func (s *Service) SaveResearchNote(id string, n research.Note) (research.Note, error)
  func (s *Service) DeleteResearchNote(id, noteID string) error
  // runner: ≤ 2 workers; queues the pending sources of every notebook
  func (s *Service) StartSummaries(ctx context.Context)
  ```
- Runner behaviour:
  - A job extracts the source and fingerprints it. A matching ready summary means no model call.
  - Otherwise it summarizes with the fixed prompt and saves with the fingerprint it read.
  - It emits `research.summary.started|ready|failed`, and switches the level to `summary` only when `!LevelSet`.
  - A source removed meanwhile ends the job silently.
  - With `wait` (the CLI), the job runs in the caller.
- The fingerprint check runs in `GetResearch` and in sending (Task 5) for sources at full or summary. A changed one
  is queued and reported as `updating`.

- [ ] **Step 1: Write the failing tests** (fake model; the fake MCP server from `mcptest` for a resource)
  - `TestCreateListDeleteResearch`: under `research/`; job notebooks unaffected (`ListNotebooks` is empty).
  - `TestAddSourceSummarizesAndSwitchesLevel`: with `wait`, the summary is stored, the level becomes `summary`, and
    there is one model call.
  - `TestSummaryNotRepeatedForSameContent`: a refresh with unchanged content makes no new model call.
  - `TestChangedSourceQueuedAndMarkedUpdating`: edit the file; `GetResearch` shows `updating`; after the runner,
    the new summary carries the new fingerprint.
  - `TestUserLevelKept`: set `off` before the summary finishes; it stays `off`.
  - `TestRemovedWhileQueued`: no summary file, no failed event.
  - `TestSourceChangedDuringSummary`: the stored fingerprint is the one read before the model call.
  - `TestRunnerQueuesPendingOnStart`: two sources with no summary get two jobs; there are never more than 2 at once.
  - `TestSummaryEventsCarrySource`: the `research.summary.ready` event has `NotebookID` and `SourceID`.
  - `TestMCPResourceSource`: reads through `connectMCP`; `RemoveMCPServer` gives 409 naming the research notebook.
  - `TestContextTotals`: a full source, a summary source and an included note give the counts and a token total
    equal to what `Build` reports.
  - `TestSetLevelSummaryWithoutSummary400`.
- [ ] **Step 2: Run** `go test ./internal/service -run Research`. Expected: FAIL.
- [ ] **Step 3: Implement.** `cmd_serve` calls `svc.StartSummaries(ctx)` in `startBackground` after the events are
  wired.
- [ ] **Step 4: Run** `go test -race ./internal/service ./internal/cli`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(service): research notebooks, sources and the summary runner`

### Task 5: Service: conversations with citations, and notes from answers

**Files:**
- Create: `internal/service/research_chat.go`
- Test: `internal/service/research_chat_test.go`

**Interfaces:**
- Produces:
  ```go
  type ResearchReply struct {
      ChatReply; Citations []research.Citation `json:"citations"`; Skipped []string `json:"skipped,omitempty"`
  }
  func (s *Service) researchChats(id string) (*chat.Store, error)       // chat.Open(<nb>/chats)
  func (s *Service) ListResearchChats(id string) ([]chat.Summary, error)
  func (s *Service) CreateResearchChat(id, title string) (chat.Session, error)
  func (s *Service) GetResearchChat(id, chatID string) (chat.Session, map[string][]research.Citation, error)
  func (s *Service) UpdateResearchChat(id, chatID string, p ChatPatch) (chat.Session, error)
  func (s *Service) DeleteResearchChat(id, chatID string) error
  func (s *Service) SendResearch(ctx context.Context, id, chatID, text string,
      onDelta func(string)) (ResearchReply, error)
  func (s *Service) NoteFromMessage(id, chatID, messageID string) (research.Note, error)
  ```
- Citations are stored in `chats/<chatID>.citations.json` (message ID → citations, 0600, atomic).
- Before history is sent, earlier assistant replies are passed through `research.Relabel` with their stored
  citations.
- An MCP or path source that cannot be read is skipped. Its label goes in `Skipped`, and the stored reply starts
  with a one-line note naming it.
- Audit kind `research.message`, digests only.

- [ ] **Step 1: Write the failing tests**
  - `TestSendResearchCites`: the fake model answers `Ship in March [1].`; the reply has a citation to source 1,
    and the sidecar holds it.
  - `TestSendResearchUsesLevels`: a summary-level source sends its summary text, not its full text; an `off` source
    is absent; an excluded note is absent.
  - `TestHistoryRelabelled`: the second question's request contains `[minutes.md]` and no `[1]` from the first
    answer.
  - `TestUnreadableSourceSkipped`: delete a referenced folder; the reply succeeds with `Skipped` naming it, and the
    stored reply starts with the note.
  - `TestResearchBusy409`: a second send while one streams gives `ErrConflict`.
  - `TestNoteFromMessage`: the note has origin `ai`, `Include` true, a title from the question (≤ 80 characters),
    and relabelled references.
  - `TestResearchChatModelPrecedence`: the conversation model beats the notebook model, which beats the app
    default.
- [ ] **Step 2: Run** `go test ./internal/service -run 'Research.*(Send|History|Skipped|Busy|Note|Model)'`.
  Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/service`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(service): research conversations with citations and notes`

### Task 6: HTTP API

**Files:**
- Create: `internal/server/research.go`
- Modify: `internal/server/server.go` (register), `docs/api.md` (new section)
- Test: `internal/server/research_test.go`

**Interfaces:** the routes in the spec's API table.
- The `messages` SSE stream sends `delta` and `finished` (`ResearchReply`), or `error`. It takes a run slot.
- Summary refresh answers `202`.

- [ ] **Step 1: Write the failing tests**
  - `TestResearchAPIFlow`: create, add a source, wait for the `research.summary.ready` event on `/v1/events`, set
    the level, check the context totals, create a chat, send (the SSE `finished` has citations), save a note.
  - `TestResearchAPIErrors`: 404 unknown notebook, 400 summary level without a summary, 409 busy chat.
- [ ] **Step 2: Run** `go test ./internal/server -run Research`. Expected: FAIL.
- [ ] **Step 3: Implement.** Document every route, body and event.
- [ ] **Step 4: Run** `go test -race ./internal/server`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(server): research notebook routes`

### Task 7: CLI `tess research`

**Files:**
- Create: `internal/cli/cmd_research.go`
- Modify: `internal/cli/cli.go` (command, usage)
- Test: `internal/cli/research_test.go`

**Interfaces:**
- `tess research new <name> [--description D]`, `list`, `show <id>`, `delete <id>`.
- `tess research source add <id> <path|mcp://server/uri> [--no-summary]`, `source list|level|remove|refresh`.
- `tess research ask <id> "question" [--chat ID]`: prints the answer, then `References:` lines `[1] label (level)`.
- `tess research note add <id> --title T "text"`, `note list|rm`.
- Research notebooks are addressed by ID inside the research root.

- [ ] **Step 1: Write the failing tests:** `TestResearchCLIFlow` (new, add, list shows `summary`, ask shows
  references, note add and list) and `TestResearchCLIUsage`.
- [ ] **Step 2: Run** `go test ./internal/cli -run Research`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/cli`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(cli): tess research`

### Task 8: Desktop client layer

**Files:**
- Create:
  - `desktop/lib/src/core/research_models.dart`
  - `desktop/lib/src/core/http_research.dart` (part, mixin on `_HttpChat`)
  - `desktop/lib/src/core/demo_research.dart`
- Modify: `core_client.dart` (interface and demo mixin), `http_core_client.dart` (part and mixin),
  `json_mapping.dart` (or mapping inside `research_models.dart`), `models.dart` (`CoreEvent` gains `sourceId`)
- Test: `desktop/test/research_client_test.dart`

**Interfaces:**
- Models:
  - `ResearchNotebook {id, name, description, sources, notes}`
  - `ResearchSource {id, label, kind, level, summaryStatus, tokens}`
  - `ResearchNote {id, title, text, origin, include}`
  - `Citation {n, kind, id, label, level, path?}`
  - `ContextTotals {sources, summaries, notes, tokens, cut}`
- Events: `ResearchDelta`, `ResearchFinished(user, assistant, citations, skipped)`.
- `CoreClient` gains a method per route: list, create, get, update and delete notebooks; add and remove sources;
  set levels; refresh; read a source; context; the chat routes; send; notes; note from message.
- The demo seeds one research notebook: two sources (one with a summary), one note, and a conversation with a
  cited answer.

- [ ] **Step 1: Write the failing tests:** JSON mapping, HTTP SSE parsing with citations, 409 → `CoreException`,
  and the demo flow (add a source, its summary becomes ready, send with a citation).
- [ ] **Step 2: Run** `flutter test test/research_client_test.dart`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `flutter test` and `flutter analyze --fatal-infos`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(desktop): core client for research notebooks`

### Task 9: Desktop research screen

**Files:**
- Create:
  - `desktop/lib/src/screens/research_screen.dart` (layout)
  - `desktop/lib/src/widgets/research_sources.dart`
  - `desktop/lib/src/widgets/research_chat.dart`
  - `desktop/lib/src/widgets/research_notes.dart`
  - `desktop/lib/src/research_state.dart`
- Modify:
  - `desktop/lib/src/shell.dart` (a RESEARCH group with items and `Section.research`)
  - `desktop/lib/src/screens/new_notebook_dialog.dart` (a first step: Job or Research)
  - `desktop/lib/src/app_state.dart` (research list, selection, `ResearchState`)
- Test: `desktop/test/research_screen_test.dart`

**Interfaces:**
- Keys:
  - `research-sources`, `research-chat`, `research-notes`
  - `level-<sourceId>-full|summary|off`
  - `research-input`, `research-send`, `cite-<n>`
  - `save-note-<messageId>`, `note-include-<noteId>`
  - `kind-job`, `kind-research`, `research-context-bar`
- The context bar text is `"<sources> sources · <summaries> summaries · ≈ <tokens/1000>k tokens"`. When `cut` is
  not empty it adds `" · <n> will be shortened"`.
- Below 1100 px the three columns become tabs (`tab-sources`, `tab-chat`, `tab-notes`).
- Tapping `cite-<n>` opens a dialog with the source's text at the cited file, or its summary when the level was
  summary.

- [ ] **Step 1: Write the failing widget tests** (demo client):
  - the dialog creates a research notebook;
  - the sidebar lists it under RESEARCH;
  - switching a source to Off updates the context bar;
  - sending shows `[1]` as a link and a References list;
  - tapping it opens the source dialog;
  - Save to note adds a note card marked AI;
  - narrow width shows tabs;
  - job notebooks still open the job page (an existing app test keeps passing).
- [ ] **Step 2: Run** `flutter test`. Expected: FAIL.
- [ ] **Step 3: Implement.** Keep every file ≤ 500 lines.
- [ ] **Step 4: Run** `task desktop:check`. Expected: PASS, coverage ≥ 80%.
- [ ] **Step 5: Commit** `feat(desktop): research notebook screen`

### Task 10: Docs and full gate

**Files:**
- `docs/architecture.md`: the `research` package row, the folder layout, the runner.
- `docs/security-model.md`: rows for citations as untrusted model output, and summaries.
- `docs/checklist.md`: new stories RES-1 (notebooks and sources), RES-2 (summaries and levels), RES-3 (cited chat
  and notes).
- `docs/README.md`: links to the design and plan.
- `CLAUDE.md`: layout line `internal/research`.

- [ ] **Step 1: Update the docs.**
- [ ] **Step 2: Run** `task ci`. Expected: all gates green.
- [ ] **Step 3: Commit** `docs: research notebooks`
