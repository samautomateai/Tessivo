# Smarter runs: chains, previous output, prompt history, watched folders

- Date: 2026-11-25
- Stories: RUN-4, MEM-3, PR-4, SRC-8
- Status: the user asked Claude to choose ("dive in and choose sensible thing"); the choices below are Claude's.

## Intent

Make recurring work less manual: a report that knows what it said last time, prompts that can be improved without
losing what worked, sources read one by one when that gives better results, and folders that tell you when they
change.

## Decisions (Claude's calls)

1. **Chains (RUN-4).** A run setting `chain`: empty (today: one call; condense only when sources do not fit) or
   `per_source` (always condense each source on its own first, then write the output from those notes). It layers
   like the other settings (app, notebook, work type, prompt, run). The run records `strategy: per-source`. Desktop:
   a checkbox on the Prompt tab, "Read each source on its own first". No general step editor (non-goal).
2. **Previous output (MEM-3).** A built-in prompt variable `{{previous}}` filled with the notebook's latest output as
   Markdown, cut at 32,000 characters with a note, or "(there is no earlier output yet)". A value the run passes
   explicitly wins. A starter prompt "Changes since last time" uses it. The run records which output it used.
3. **Prompt history (PR-4).** Saving a prompt whose text or settings changed keeps the old one as
   `prompts/.history/<name>/<n>.json` (at most 50 kept, oldest dropped). The prompt gets a `version`; runs record
   `promptVersion`. API and CLI list versions and restore one (a restore is a new save, so history is never lost).
   Desktop: the Prompt tab shows "Version n" and *History…* (dates, text, *Restore*). *Compare* can pick any other
   output of the notebook (the diff API already takes `against`).
4. **Watched folders (SRC-8).** A notebook setting `watch`: `off` (default), `notify` or `run`. `tess serve`
   checks the in-place folder and file sources of watching notebooks every 60 s by fingerprint (paths, sizes and
   modification times of at most 5,000 files; no file contents read, no OS watcher, so it works the same on every
   OS). After a change it waits until two checks in a row agree (quiet for a minute), then publishes
   `sources.changed` on `/v1/events` (desktop notice: "Sources changed in <notebook>") and, in `run` mode, starts a
   run with the notebook's default prompt unless the notebook is busy or archived. The first check only records the
   baseline. Desktop: a Watch select on the Sources tab; CLI `tess notebook watch <nb> off|notify|run`.

## Non-goals

General multi-step pipelines with user-defined steps; watching web pages or MCP resources; OS file-system watchers.

## Constraints

OS independent; bounded work (file counts, text sizes, one watcher loop); every change documented (api.md, user
guide, checklist, architecture).
