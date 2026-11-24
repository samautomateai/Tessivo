# Organize and share: notebook archives, tags and pins, team packs

- Date: 2026-11-25
- Stories: NB-4, NB-5, PR-5, TPL-7, MCP-5
- Status: draft for approval

## Intent

Tessivo works well for one person. To roll it out to a team, people need to hand a notebook to a colleague, keep
many notebooks in order, and share a vetted setup (prompts, templates, MCP servers) without copying files around
by hand. The team has no practice for this yet, so this design sets the standard.

## Goals

1. **Notebook archive (NB-4).** Export a notebook (job or research) as one file and import it on another machine.
2. **Tags and pins (NB-5).** Tag notebooks, filter by tag, pin the ones in use so they stay on top.
3. **Packs (PR-5, TPL-7, MCP-5).** A pack is a folder or git repo holding prompts, templates and MCP server entries.
   People add a pack once; Tessivo keeps it current. Items can be marked recommended or required.

## Non-goals

- A team server, accounts, permissions or a pack marketplace. Everything stays local or in a repo the team owns.
- Packs that change providers, aliases, keys or limits (that is settings profiles, SET-10, later).
- Syncing notebooks between people. An archive is a hand-over, not a shared copy.
- Signing packs. Trust comes from where the pack lives (your repo, your share); signing can come later.

## Anti-goals (a failure even if it works)

- Anything in a pack or archive that runs on a machine without the person agreeing first.
- An archive that leaks run history, audit logs, threads, keys or paths from the sender's machine.
- A broken or hostile pack or archive that crashes Tessivo or writes outside its folder.

## Approaches considered

| Approach | Trade-off | Decision |
| --- | --- | --- |
| A. Packs as folders or git repos that people subscribe to | Live updates and enforceable "required"; needs git for repo packs (already used for Git-changes sources) | **Chosen** (Claude's call; the user asked Claude to define the standard) |
| B. Packs as single files that people import (copy) | Simplest; no updates, "required" cannot hold over time | Covered by A: a zipped pack folder can be added from the zip |
| C. A team server | Accounts, hosting, a new trust model | Ruled out: Tessivo is local-first with no accounts |

## Design

### 1. Notebook archive

- **File:** `<notebook name>.tessivo.zip`. Inside: `archive.json` (`kind: "notebook"|"research"`, `formatVersion: 1`,
  `tessivoVersion`, `createdAt`, `includes`), then the notebook folder's own files under `notebook/`.
- **Always included:** the manifest (`notebook.json` or the research manifest), prompts, memory, copied sources
  (`context/`), research notes and source summaries, `schedules.json`, tags.
- **Included when asked (`--outputs`, a checkbox in the app):** `outputs/` (validated outputs and their Markdown).
- **Never included:** `runs/`, `audit.jsonl`, `threads/`, `suggestions/`, `schedule-state.json`, research chats,
  anything outside the notebook folder, secrets (there are none in a notebook; the existing secret scanner still
  checks every text file and refuses the export if it finds one, naming the file).
- **Sources referenced in place** (code bases, folders, git changes, MCP resources) are kept as entries with their
  original label and kind, marked `needsRelink: true`, and their path removed. Web pages keep their URL.
- **Import:** into the notebooks folder as a new notebook. A clashing ID gets a new one; a clashing name gets
  "(imported)". Schedules come in **disabled**. The app then shows the sources that need relinking, each with
  *Choose folder…*; runs work without them (they are left out until relinked).
- **Limits on import (hostile input):** at most 5,000 entries and 512 MB uncompressed in total, 64 MB per entry,
  declared sizes not trusted (count bytes while reading), no absolute paths, no `..`, no symlinks, only the
  expected top-level names; `archive.json` must parse and match `formatVersion` 1. Written atomically: extract to a
  temporary folder in the notebooks folder (0700), validate, then rename. Fuzz the reader.
- **CLI:** `tess notebook export <nb> [--outputs] [-o file]`, `tess notebook import <file>`.
- **API:** `GET /v1/notebooks/{id}/archive?outputs=1` (zip bytes), `POST /v1/notebooks/import` (zip body, size
  capped). Same for research: `/v1/research/{id}/archive`, `/v1/research/import`.
- **Desktop:** notebook menu → *Export…* (checkbox "Include outputs"); Notebooks side bar → *Import…*; a
  "Relink sources" notice on the notebook page while any source needs it.

### 2. Tags and pins

- `notebook.json` gains `tags: []string` and `pinned: bool` (research manifests too). Up to 10 tags per notebook,
  each 1–32 characters of letters, digits, space, `-` and `_`; compared case-insensitively, shown as typed.
- **Side bar:** a *Pinned* group first, then the rest; a tag filter at the top of the Notebooks side bar (a small
  field with the known tags as suggestions). Each row shows its tags on hover (tooltip) to stay one line high.
- **Notebook page header:** tags as Carbon tags after the work type, and *Pin* / *Unpin* plus *Edit tags…* in the
  notebook menu.
- **CLI:** `tess notebook tag <nb> add|remove <tag>`, `tess notebook pin|unpin <nb>`, `tess notebook list --tag x`.
- **API:** `PATCH /v1/notebooks/{id}` accepts `tags` and `pinned`; `GET /v1/notebooks?tag=x`.
- Search matches tags.

### 3. Packs

#### Format

A pack is a folder (or the root of a git repo, or a zip of that folder):

```
tessivo-pack.json      {formatVersion: 1, id, name, description, publisher, version,
                        recommended: [item refs], required: [item refs]}
templates/<id>.json    template files, the same format user templates use today
prompts/<name>.json    {name, text, workType?, settings?}: library prompts
mcp-servers.json       [{name, command?, args?, url?, headers?, env?}]: the same fields as the MCP config
README.md              optional; shown in the app
```

- `id` is `[a-z0-9-]{1,63}`. Item refs are `template:<id>`, `prompt:<name>`, `mcp:<name>`.
- Items keep their own IDs inside the pack and show as `<pack id>/<item id>` wherever two could clash.
- Every file is validated with today's rules (template limits, prompt text limits, MCP server validation). Values
  must not hold secrets: the secret scanner runs on every file, and MCP `env`/`headers` may only use
  `${secret:NAME}` references, as today. A file that fails is skipped and listed as a problem in the pack's status;
  the rest of the pack still loads.
- Limits: 200 templates, 500 prompts, 50 MCP servers, 4 MB total per pack, 20 packs per user.

#### Adding and refreshing

- **Settings → Packs** (new section) and `tess pack add <path|git URL|zip>`, `tess pack list`, `tess pack refresh`,
  `tess pack remove <id>`.
- A folder or zip path: a folder is read in place (read-only); a zip is unpacked (same hardening as archives) into
  `<config dir>/packs/<id>/`.
- A git URL (`https://…` or `git@…`; no `file://`, no `ext::`): cloned shallow into `<config dir>/packs/<id>/` with
  the hardened git runner Tessivo already uses (no hooks, no config from the repo, bounded time and size). The
  user's own git credentials are used; Tessivo stores none. Refresh = fetch + hard reset to the remote branch,
  at app start and on *Refresh*; a failed refresh keeps the last good copy and shows when it was fetched.
- Repo packs need git on the PATH; without it, adding or refreshing one fails with "git is not installed" and folder
  and zip packs still work.
- The list of packs lives in the core config (`packs: [{id, source, branch?}]`); pack content is never copied into
  the config.

#### What people see

- **Templates:** pack templates join the gallery with a tag naming the pack, *Recommended* or *Required* where the
  pack says so. Using one copies it into the notebook exactly as today.
- **Prompts:** pack prompts join *Saved prompts → From the library…*, grouped by pack.
- **MCP servers:** pack servers appear in Settings → MCP servers, grouped by pack, **off until trusted**. *Trust…*
  shows the exact command and arguments (or URL) and asks for a yes; the trust is recorded against a hash of that
  entry, so a changed command needs trust again. Trusted pack servers behave like your own. Trust is kept in the
  core config (`mcp.trusted: {"<pack id>/<server>": "<sha256 of the entry>"}`); removing the pack drops it.
- **Recommended** items are badged and sorted first. **Required** items are badged *Required* and sorted before
  recommended ones (Tessivo has no hiding, so "required" means "always offered, first"); a required MCP server
  shows a notice in Settings and on the status bar until it is trusted. Required never
  bypasses trust.

#### Publishing

- `tess pack init <dir>` writes an empty pack with a README.
- `tess pack export <dir> [--templates a,b] [--prompts …] [--mcp …]` writes chosen user templates, saved prompts
  and MCP servers into a pack folder (MCP entries keep `${secret:…}` references only). The team commits that folder
  to a repo or puts it on a share.

### Security summary

- Nothing from a pack runs without trust; templates and prompts are text, MCP servers need trust per entry hash.
- Archives and packs are hostile input: bounded sizes and counts, no path escapes or links, parse errors reported,
  fuzz tests for the archive reader, the pack loader and the manifest parsers.
- Git is run with the existing hardening; only https and ssh URLs.
- Files Tessivo writes are 0600, folders 0700; extraction and pack updates are atomic (temp folder + rename).
- Audit: export, import, pack add, refresh, remove and every MCP trust decision are written to the app audit log
  (no paths from the archive content, no secrets).

### Docs

ADR 0010 (pack format and trust), `docs/api.md`, user guide (Share and organize), architecture (pack and archive
packages), checklist rows for NB-4, NB-5, PR-5, TPL-7, MCP-5.

## Left to the builder

- Package layout (likely `internal/archive` and `internal/pack`, service logic in `internal/service`).
- Exact zip reading code (reuse the Office extractor's limits approach), git command lines within the existing
  hardened runner, and the desktop layout of Settings → Packs within the design system.
