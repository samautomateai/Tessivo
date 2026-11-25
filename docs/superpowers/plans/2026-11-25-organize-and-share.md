# Organize and Share Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Notebook archives (NB-4), tags and pins (NB-5), and team packs of prompts, templates and MCP servers with
explicit trust (PR-5, TPL-7, MCP-5).

**Architecture:** Two new core packages: `internal/archive` (write and safely read `.tessivo.zip` files) and
`internal/pack` (load and validate a pack folder). Business logic in `internal/service` (`archive.go`, `packs.go`,
`tags.go`), API routes in `internal/server`, CLI in `internal/cli`. The desktop gets tags and pins in the side bar,
Export/Import, a relink notice, and Settings → Packs, all on the design system (ADR 0009).

**Tech Stack:** Go standard library (`archive/zip`, `crypto/sha256`, `os/exec` for git through the existing
hardened runner), Flutter desktop.

**Spec:** `docs/superpowers/specs/2026-11-25-organize-and-share-design.md`

## Global Constraints

- OS independent; no `runtime.GOOS` branches; git through the hardened runner (no hooks, no repo config, bounded).
- Files Tessivo writes 0600, folders 0700; extraction and pack updates atomic (temp folder in the target's parent, then
  rename).
- Archive limits: ≤ 5,000 entries, ≤ 512 MB uncompressed total, ≤ 64 MB per entry, count bytes while reading, no
  absolute paths, no `..`, no symlinks, only expected top-level names, `archive.json` `formatVersion` 1.
- Archive never includes `runs/`, `audit.jsonl`, `threads/`, `suggestions/`, `schedule-state.json`, research chats;
  outputs only when asked; in-place sources lose their path and get `needsRelink: true`; schedules import disabled.
- Tags: ≤ 10 per notebook, 1–32 chars of letters, digits, space, `-`, `_`; case-insensitive compare, shown as typed.
- Pack limits: 200 templates, 500 prompts, 50 MCP servers, 4 MB total, 20 packs; pack id `[a-z0-9-]{1,63}`; git URLs
  `https://` or `git@…` only.
- Pack MCP servers are off until trusted; trust = `mcp.trusted["<pack>/<server>"]` = sha256 of the entry's canonical
  JSON; a changed entry needs trust again; required never bypasses trust.
- No secrets in packs or archives: secrets scanner (`secrets.RedactPatterns(s) != s`) on every text file.
- Audit: export, import, pack add/refresh/remove, trust decisions.
- Files ≤ 500 lines (tests ≤ 800), lines ≤ 120; coverage ≥ 80 % Go and desktop; fuzz tests for parsers.
- Every change updates docs; new dependency: none.

## Review Focus

- A zip with an entry named `notebook/../../x`, an absolute path, a symlink, or a declared size of 1 KB that
  inflates to 1 GB: import refuses with a clear error and writes nothing (Task 3 tests pin each).
- Importing the same archive twice: the second becomes a new notebook with a new ID and "(imported)" name (Task 4).
- A pack whose `mcp-servers.json` entry changes its command after trust: the server stops being usable until
  trusted again (Task 8).
- A git pack that fails to refresh (offline, deleted repo): the last good copy stays in use with its fetch time
  (Task 7).
- A pack file with a template that fails validation: the other items load and the problem is listed (Task 6).

---

### Task 1: Tags and pins in the core

**Files:**
- Modify: `internal/notebook/notebook.go` (Manifest), `internal/notebook/manage.go`, `internal/research/store.go`
  (Manifest), `internal/service/workspace.go`, `internal/service/research.go`, `internal/service/search.go`,
  `internal/server/server.go` (PATCH body, `?tag=`), `internal/cli/cmd_notebook_manage.go`
- Create: `internal/notebook/tags.go`
- Test: `internal/notebook/tags_test.go`, `internal/service/workspace_manage_test.go`, `internal/cli/` test

**Interfaces:**
- Produces: `notebook.Manifest.Tags []string \`json:"tags,omitempty"\``, `Pinned bool \`json:"pinned,omitempty"\``
  (same two fields on `research.Manifest`); `notebook.NormalizeTags(in []string) ([]string, error)`;
  `NotebookUpdate.Tags *[]string`, `NotebookUpdate.Pinned *bool`; `NotebookInfo.Tags`, `.Pinned` (JSON `tags`,
  `pinned`); `ListNotebooks()` sorts pinned first, then as today.

- [ ] **Step 1: Tests:** `TestNormalizeTags`: `["Infra", "infra ", "a/b"]` → error naming `a/b`; `["Infra","infra"]`
  → `["Infra"]`; 11 tags → error; 33-char tag → error. Service: update sets tags and pinned, list puts pinned first,
  `ListNotebooks` with tag filter (`s.ListNotebooksTagged("infra")`) matches case-insensitively; search finds a
  notebook by tag. CLI: `tess notebook tag <nb> add infra`, `remove`, `tess notebook pin|unpin <nb>`,
  `tess notebook list --tag infra`.
- [ ] **Step 2:** Run `go test ./internal/notebook ./internal/service ./internal/cli` → FAIL.
- [ ] **Step 3:** Implement; API: `PATCH /v1/notebooks/{id}` and `PATCH /v1/research/{id}` accept `tags`, `pinned`;
  `GET /v1/notebooks?tag=` filters.
- [ ] **Step 4:** Run → PASS; `task lint`.
- [ ] **Step 5: Commit** `feat: tags and pins for notebooks`.

### Task 2: Tags and pins in the desktop

**Files:**
- Modify: `desktop/lib/src/core/models.dart` (Notebook `tags`, `pinned`), json mapping, `core_api.dart`
  (`updateNotebook(id, {tags, pinned})`), fake core, `screens/notebook/notebook_side_bar.dart`,
  `screens/notebook/notebook_page.dart` (header tags), `widgets/notebook_menu.dart` (Pin/Unpin, Edit tags…),
  `shell/side_bar.dart` (research pins)
- Test: `desktop/test/tags_test.dart`

**Interfaces:**
- Produces: `AppState.setTags(String id, List<String> tags)`, `AppState.setPinned(String id, bool pinned)`,
  `AppState.tagFilter` (String?, not remembered).

- [ ] **Step 1: Tests:** a pinned notebook is listed under a `PINNED` group label above the rest; typing `infra` in
  the side bar's tag filter (key `tag-filter`) leaves only notebooks with that tag; the header shows each tag as a
  `TvTag`; *Edit tags…* (key `notebook-tags`) saves through the core; Pin from the menu (key `notebook-pin`) moves the
  row; long tag lists do not overflow at 200 % zoom on 1366×768.
- [ ] **Step 2–4:** fail → implement → pass (`flutter test`, `flutter analyze --fatal-infos`).
- [ ] **Step 5: Commit** `feat(desktop): tags and pins`.

### Task 3: The archive format

**Files:**
- Create: `internal/archive/archive.go` (types, limits), `write.go`, `read.go`
- Test: `internal/archive/archive_test.go`, `internal/archive/fuzz_test.go`

**Interfaces:**
- Produces:
  - `type Meta struct { Kind string /* notebook|research */; FormatVersion int; TessivoVersion, CreatedAt string;
    Includes []string }` (`archive.json`).
  - `func Write(w io.Writer, dir string, meta Meta, include func(rel string) bool) error` — writes `archive.json`
    then every regular file under `dir` that `include` accepts, under `notebook/`; refuses (names the file) when a
    text file trips the secret scanner.
  - `func Extract(r io.ReaderAt, size int64, dest string) (Meta, error)` — validates every entry against the Global
    Constraints, extracts into `dest` (created 0700, files 0600), returns `archive.json`.
  - Errors: `ErrTooLarge`, `ErrUnsafePath`, `ErrFormat`.

- [ ] **Step 1: Tests:** round trip of a small notebook folder; `include` excludes `runs/x.json`; a file holding
  `sk-live-…` refuses export naming it; Extract refuses `notebook/../../x`, `/etc/x`, `C:\x`, a symlink entry, an
  unexpected top-level `evil/`, a missing `archive.json`, `formatVersion` 2, 5,001 entries, an entry declaring 1 KB
  that inflates past 64 MB (each test asserts `dest` is left empty); `FuzzExtract` seeds with the round-trip zip.
- [ ] **Step 2–4:** fail → implement → pass (`go test -race ./internal/archive`, `go run ./tools/fuzzall` target).
- [ ] **Step 5: Commit** `feat(archive): hardened notebook archive format`.

### Task 4: Export and import in the service, API and CLI

**Files:**
- Create: `internal/service/archive.go`, `internal/server/archive.go`, `internal/cli/cmd_notebook_archive.go`
- Modify: `internal/notebook/notebook.go` (ContextItem `NeedsRelink bool \`json:"needsRelink,omitempty"\``),
  `internal/research/store.go` (Source same field), `internal/service/workspace.go` (relink)
- Test: `internal/service/archive_test.go`, `internal/server/archive_test.go`, CLI test

**Interfaces:**
- Consumes: `archive.Write`, `archive.Extract` (Task 3).
- Produces: `Service.ExportNotebook(id string, outputs bool, w io.Writer) error`,
  `Service.ImportNotebook(r io.ReaderAt, size int64) (NotebookInfo, error)`, `Service.ExportResearch`,
  `Service.ImportResearch` (same shapes, `ResearchInfo`), `Service.RelinkContext(nbID, ctxID, path string)
  (NotebookInfo, error)`; routes `GET /v1/notebooks/{id}/archive?outputs=1`, `POST /v1/notebooks/import`,
  `GET /v1/research/{id}/archive`, `POST /v1/research/import`, `POST /v1/notebooks/{id}/context/{ctx}/relink`
  (`{path}`); import bodies capped at 512 MB; CLI `tess notebook export <nb> [--outputs] [-o file]`,
  `tess notebook import <file>`, `tess context relink <nb> <ctx> <path>`.

- [ ] **Step 1: Tests:** export excludes runs, audit, threads, suggestions, schedule-state and (without `--outputs`)
  outputs; in-place sources lose `path` and get `needsRelink`; import twice → two notebooks, new ID, name
  "<name> (imported)"; imported schedules are disabled; a run skips sources that need relinking; relink checks the
  path like `addContext` and clears the flag; research round trip keeps notes and summaries and drops chats;
  audit has `notebook.export` / `notebook.import` entries without paths from the archive.
- [ ] **Step 2–4:** fail → implement → pass; `task lint`.
- [ ] **Step 5: Commit** `feat: export and import notebooks as archives`.

### Task 5: Archives in the desktop

**Files:**
- Modify: `desktop/lib/src/core/core_api.dart`, `http_core_client.dart` (`exportNotebook`, `importNotebook`,
  `relinkContext`, research equivalents), fake core, `widgets/notebook_menu.dart` (*Export…*, key
  `notebook-export`, modal with checkbox `export-outputs` and file path), `screens/notebook/notebook_side_bar.dart`
  (*Import…*, key `import-notebook`), `screens/notebook/notebook_page.dart` (relink notice)
- Test: `desktop/test/archive_test.dart`

- [ ] **Step 1: Tests:** Export writes the bytes from the core to the chosen path and shows "Saved …"; Import reads
  a file path, opens the new notebook and shows a `TvInlineNotice` (key `relink-notice`) "2 sources need relinking"
  with *Relink…* per source (`relink-<ctx>`) that calls the core; the research side bar has the same Import.
- [ ] **Step 2–4:** fail → implement → pass.
- [ ] **Step 5: Commit** `feat(desktop): export, import and relink`.

### Task 6: The pack format

**Files:**
- Create: `internal/pack/pack.go` (types, limits, `Load`), `internal/pack/write.go` (`Init`, `Export`)
- Test: `internal/pack/pack_test.go`, `internal/pack/fuzz_test.go`

**Interfaces:**
- Produces:
  - `type Manifest struct { FormatVersion int; ID, Name, Description, Publisher, Version string; Recommended,
    Required []string }` (`tessivo-pack.json`, JSON camelCase).
  - `type Prompt struct { Name, Text, WorkType string; Settings config.RunSettings }`.
  - `type Pack struct { Manifest Manifest; Templates []template.Template; Prompts []Prompt;
    MCP []config.MCPServer; Problems []string; README string }`.
  - `func Load(dir string) (Pack, error)` — error only for a missing or invalid manifest; bad items go to
    `Problems` (file name + reason).
  - `func (p Pack) Mark(ref string) string` → `"required"`, `"recommended"` or `""` for `template:<id>`,
    `prompt:<name>`, `mcp:<name>`.
  - `func Init(dir, id, name string) error`; `func Export(dir string, m Manifest, ts []template.Template,
    ps []Prompt, servers []config.MCPServer) error` (MCP env/headers must be `${secret:…}` references).

- [ ] **Step 1: Tests:** a good pack loads all three kinds; a template that fails `Validate` lands in `Problems` and
  the rest load; a prompt with `ghp_…` text is a problem; an MCP header with a literal token is a problem; 201
  templates → the first 200 load and a problem says so; total > 4 MB → error; manifest id `Bad_ID` → error; `Mark`
  answers for each list; `Init` + `Export` round-trip through `Load`; `FuzzLoadManifest`.
- [ ] **Step 2–4:** fail → implement → pass.
- [ ] **Step 5: Commit** `feat(pack): pack format and loader`.

### Task 7: Adding, refreshing and removing packs

**Files:**
- Create: `internal/pack/source.go` (folder, zip, git), `internal/service/packs.go`, `internal/server/packs.go`,
  `internal/cli/cmd_pack.go`
- Modify: `internal/config/file.go` (`Packs []PackSource \`json:"packs,omitempty"\``), `internal/config/mcp.go`
  (`Trusted map[string]string \`json:"trusted,omitempty"\``), `internal/gitdiff/gitdiff.go` (export the hardened
  runner as `gitdiff.Run(ctx, dir string, args ...string) (string, error)`)
- Test: `internal/pack/source_test.go`, `internal/service/packs_test.go`, CLI and server tests

**Interfaces:**
- Produces: `config.PackSource{ID, Source, Kind /* folder|zip|git */, Branch string}`;
  `Service.AddPack(source, branch string) (PackInfo, error)`, `Service.RefreshPacks(ctx) []PackInfo`,
  `Service.RemovePack(id string) error`, `Service.Packs() []PackInfo`; `PackInfo{ID, Name, Description, Publisher,
  Version, Source, Kind, FetchedAt, Error string; Templates, Prompts, Servers int; Problems []string}`; routes
  `GET /v1/packs`, `POST /v1/packs` (`{source, branch?}`), `POST /v1/packs/refresh`, `DELETE /v1/packs/{id}`;
  CLI `tess pack add|list|refresh|remove|init|export`.

- [ ] **Step 1: Tests:** a folder pack is read in place; a zip pack is extracted into `<config>/packs/<id>/` with the
  archive limits; a git pack is cloned shallow from a local bare repo created in the test (`git init --bare` via the
  hardened runner; skip with a message when git is absent) and refreshed after a new commit; a refresh that fails
  keeps the last copy and sets `Error` and the old `FetchedAt`; `file://` and `ext::` URLs are refused; the 21st
  pack is refused; removing a pack deletes its copy and its `mcp.trusted` entries; audit entries for add, refresh,
  remove; `tess serve` refreshes packs once at start (in the background, bounded by a 60 s timeout per pack).
- [ ] **Step 2–4:** fail → implement → pass; `go test -race`.
- [ ] **Step 5: Commit** `feat: add, refresh and remove packs`.

### Task 8: Pack contents and MCP trust

**Files:**
- Modify: `internal/service/templates.go` (pack templates in `ListTemplates`, ids `<pack>/<id>`, `Mark`),
  `internal/server/server.go` (`/v1/prompt-library` adds pack prompts with `pack`, `mark`),
  `internal/service/mcp.go` (pack servers listed with `pack`, `trusted`, `mark`; only trusted ones usable by runs
  and chats), new trust routes in `internal/server/mcp.go`, `internal/cli/cmd_mcp.go` (`tess mcp trust|untrust`)
- Test: `internal/service/packs_contents_test.go`, server and CLI tests

**Interfaces:**
- Produces: `TemplateInfo.Pack`, `.Mark`; library prompt fields `pack`, `mark`; `MCPServerInfo.Pack`, `.Trusted`,
  `.Mark`; `Service.TrustMCP(name string, trust bool) error` (`name` is `<pack>/<server>`), routes
  `POST /v1/mcp/servers/{name}/trust` and `DELETE …/trust`; `func entryHash(config.MCPServer) string` (sha256 of
  canonical JSON).

- [ ] **Step 1: Tests:** a pack template is listed as `team/risk-review` with its mark and can create a notebook;
  required items sort before recommended before the rest; pack prompts appear in the library under the pack; an
  untrusted pack server is listed but a notebook run or chat that names it gets "trust it in Settings first"; after
  trust it works; after the pack changes that server's command (refresh) it is untrusted again; trust and untrust are
  audited.
- [ ] **Step 2–4:** fail → implement → pass.
- [ ] **Step 5: Commit** `feat: pack templates, prompts and trusted MCP servers`.

### Task 9: Packs in the desktop

**Files:**
- Create: `desktop/lib/src/screens/settings/packs_section.dart`
- Modify: `app_state.dart` (`SettingsSection.packs`), `settings_side_bar.dart`, `settings_page.dart`, core API and
  models (`PackInfo`, template/prompt/MCP fields), fake core, `screens/templates_screen.dart` (pack and mark tags),
  `widgets/prompt_library_dialog.dart` (grouped by pack), `screens/mcp_settings.dart` (pack servers, *Trust…*),
  `shell/banners.dart` + status bar (required untrusted notice)
- Test: `desktop/test/packs_test.dart`

- [ ] **Step 1: Tests:** Settings → Packs (key `settings-packs`) lists packs as a `TvDataTable` (name, source,
  fetched, items, problems) with *Add pack…* (`pack-add`, modal with source field and optional branch),
  *Refresh* (`packs-refresh`) and *Remove* (`pack-remove-<id>`); a pack with problems shows them; the gallery shows
  `Required` / `Recommended` tags and the pack name; *Trust…* (`mcp-trust-<pack>-<name>`) opens a `TvModal` showing
  the exact command and args before trusting; a required untrusted server shows a notice (`required-untrusted`)
  that opens Settings → MCP servers; selecting a pack shows its README as Markdown; no overflow at 200 %.
- [ ] **Step 2–4:** fail → implement → pass.
- [ ] **Step 5: Commit** `feat(desktop): packs and trusted MCP servers`.

### Task 10: Docs and final checks

**Files:**
- Create: `docs/adr/0010-packs-and-archives.md`
- Modify: `docs/README.md`, `docs/api.md`, `docs/user-guide.md` (Share and organize), `docs/architecture.md`
  (`archive`, `pack` packages; notebook folder fields), `docs/checklist.md` (NB-4, NB-5, PR-5, TPL-7, MCP-5 rows),
  `docs/security-model.md` (archives and packs as hostile input, MCP trust)

- [ ] **Step 1:** ADR 0010: decision (folder/git/zip packs, trust per entry hash, archive contents), alternatives
  (single-file import only, team server, signed packs), consequences.
- [ ] **Step 2:** Update the docs; `task text lengths`.
- [ ] **Step 3:** `task ci` → green; coverage ≥ 80 % Go and desktop.
- [ ] **Step 4: Commit** `docs: organize and share (ADR 0010)`.
