# Smarter Runs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Chains per source (RUN-4), `{{previous}}` (MEM-3), prompt history (PR-4), watched folders (SRC-8).

**Architecture:** Settings and executor changes in `internal/config` and `internal/executor`; prompt history in
`internal/notebook`; a new `internal/watch` package (fingerprints) driven by a loop in `tess serve`; service, API,
CLI and desktop on top.

**Spec:** `docs/superpowers/specs/2026-11-25-smarter-runs-design.md`

## Global Constraints

- OS independent; no OS watchers; fingerprints read metadata of at most 5,000 files per source.
- `{{previous}}` cut at 32,000 characters; prompt history keeps at most 50 versions per prompt.
- Files 0600, folders 0700, atomic writes; files ≤ 500 lines (tests ≤ 800), lines ≤ 120; coverage ≥ 80 %.
- Every change updates docs.

## Review Focus

- A prompt saved twice with the same text: no new version (Task 1).
- `{{previous}}` in a notebook with no outputs: the run still works (Task 2).
- A watched folder that is deleted or unreadable: the watcher logs it once and keeps going (Task 4).
- A `run`-mode notebook already running when its folder changes: no second run (Task 4).
- Restoring an old prompt version keeps the current one in history (Task 1).

---

### Task 1: Prompt history (core)
- `notebook.Prompt.Version int`; `SavePrompt` archives the old file when text or settings changed; keeps 50;
  `(*Notebook).PromptVersions(name) ([]PromptVersion, error)` (newest first: Version, SavedAt, Text, Settings);
  `Run.PromptVersion int`.
- Service `PromptVersions(nbID, name)`, `RestorePromptVersion(nbID, name, version) (PromptInfo, error)`; routes
  `GET /v1/notebooks/{id}/prompts/{name}/versions`, `POST …/versions/{v}/restore`; CLI `tess prompt history|restore`.
- Tests: same text → no version; 52 saves → 50 kept; restore keeps the current one; run records the version.
- Commit `feat: prompt version history`.

### Task 2: `{{previous}}` (core)
- `buildSpec` fills `previous` when the prompt uses it and the run did not set it: latest output Markdown (cut at
  32,000 chars + note) or "(there is no earlier output yet)"; the run record gets `previousOutput`.
- Starter prompt `changes-since-last-time` (report work type).
- Tests: filled from the newest output, cut, empty notebook, explicit value wins, library has the starter.
- Commit `feat: {{previous}} brings the latest output into a prompt`.

### Task 3: Per-source chains (core)
- `RunSettings.Chain` (`""` or `per_source`; validated; layered in `Resolve`); pipeline condenses each source first
  when set; strategy `per-source`.
- Tests: per_source makes one condense call per source even when everything fits; invalid value refused; layering.
- Commit `feat: per-source chains`.

### Task 4: Watched folders (core)
- `internal/watch`: `Fingerprint(path) (string, error)` (bounded walk, metadata only).
- `notebook.Manifest.Watch` (`off|notify|run`); `NotebookUpdate.Watch`; CLI `tess notebook watch`.
- `service.Watcher{Svc, Interval, OnEvent, Run}` with `Check(ctx)` one pass (testable) and `Run(ctx)` loop; started by
  `tess serve`; publishes `sources.changed`; run mode starts a run unless busy or archived.
- Tests: baseline then change → one event after the quiet check; busy notebook not re-run; missing folder tolerated;
  fingerprint ignores content-equal touch? (no: mtime counts) — and changes on add/remove/resize.
- Commit `feat: watched folders`.

### Task 5: Desktop
- Prompt tab: "Version n", *History…* (`prompt-history`, restore `prompt-restore-<v>`), checkbox `chain-per-source`
  (a run override, and saved with a prompt).
- Output toolbar: *Compare* is a menu of the other outputs (`compare`, items `compare-<id>`), using `against`.
- Sources tab: Watch select (`watch-mode`); `sources.changed` events become notices.
- Tests for each; 200 % zoom check of the Prompt tab.
- Commit `feat(desktop): prompt history, chains, compare any output, watch`.

### Task 6: Docs and checks
- api.md, user guide, checklist (RUN-4, MEM-3, PR-4, SRC-8), architecture (`watch`); `task ci`.
- Commit `docs: smarter runs`.
