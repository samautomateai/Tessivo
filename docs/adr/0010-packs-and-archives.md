# ADR 0010: notebook archives and team packs

- Status: accepted (2026-11-25)
- Stories: NB-4, NB-5, PR-5, TPL-7, MCP-5
- Spec: `docs/superpowers/specs/2026-11-25-organize-and-share-design.md`

## Context

Tessivo worked well for one person. Rolling it out to a team needs three things it did not have: handing a notebook
to a colleague, keeping many notebooks in order, and sharing a vetted setup (prompts, templates, MCP servers)
without copying files by hand. There was no existing team practice to follow, so this decision sets the standard.
Everything stays local-first: no accounts, no server.

## Decision

**Notebook archives.** A notebook (job or research) exports to one `<name>.tessivo.zip`: `archive.json`
(`kind`, `formatVersion` 1, `createdAt`, `includes`) and the notebook's files under `notebook/`.
- In: the manifest, prompts, memory, copied sources, schedules, tags; research notes and source summaries; outputs
  only when asked.
- Never in: run records, the audit log, threads, suggestions, schedule state, research chats, secrets (the secret
  scanner refuses an export that holds one, naming the file).
- Sources that point at the sender's computer (folders, code bases, git changes, any absolute or non-local path)
  keep their label and kind but lose their path, marked `needsRelink` and off. Import does this again, so an edited
  archive can never point a run at the importer's files. Runs skip such sources until relinked.
- Import creates a new notebook (new ID, "(imported)" after a name in use) with its schedules turned off. Archives
  are hostile input: ≤ 5,000 entries, ≤ 64 MB an entry, ≤ 512 MB in all, counted while reading; no absolute names,
  `..`, backslashes, drives or links; unpacked into a temporary folder renamed at the end; fuzzed.

**Tags and pins.** `tags` (up to 10, 1–32 letters, digits, spaces, `-`, `_`, compared ignoring case) and `pinned`
in the notebook and research manifests. Pinned notebooks list first; search matches tags.

**Team packs.** A pack is a folder, a zip of that folder, or a git repository: `tessivo-pack.json` (`id`, `name`,
`publisher`, `version`, `recommended` and `required` item refs), `templates/`, `prompts/`, `mcp-servers.json`,
`README.md`.
- Added once (Settings → Packs, `tess pack add`): folders are read in place; zips are unpacked and git repositories
  cloned shallow into `<config dir>/packs/<id>/` with the hardened git runner (no hooks, no repository config, no
  `ext::`; `https://` and `git@` URLs only; the user's own git sign-in; 60 s per fetch; ≤ 64 MB with `.git`).
  `tess serve` refreshes git packs at start; a failed refresh keeps the last good copy and reports it.
- Every item is checked with the same rules as the user's own (template limits, prompt limits, MCP validation that
  refuses literal credentials) plus the secret scanner; a bad item is skipped and listed as a problem, the rest load.
  Limits: 200 templates, 500 prompts, 50 servers, 4 MB, 20 packs.
- Pack templates join the gallery and pack prompts the library as `<pack>/<item>`, marked and sorted
  required → recommended → the rest.
- **MCP trust.** A pack's MCP servers are listed as `<pack>/<server>` and stay unusable until the user trusts each
  one after seeing its exact command or URL. Trust is stored as `mcp.trusted["<pack>/<server>"]` = sha256 of the
  entry, so any change to the command, arguments, URL, environment or headers needs trust again. "Required" never
  bypasses trust; a required untrusted server shows a banner instead. Trust decisions are audited.

## Alternatives considered

| Option | Why not |
| --- | --- |
| Packs as single files imported (copied) | No updates; "required" cannot hold over time. Covered anyway: a zipped pack folder can be added |
| A team server with accounts | Hosting, accounts and a new trust model; Tessivo is local-first |
| Signed packs | Key management for every team; trust comes from where the pack lives (the team's repo or share) for now |
| Archives as the notebook folder zipped as is | Would carry run history, the audit log and paths from the sender's disk |
| Trusting a pack server by name | A pack update could change what runs without asking |

## Consequences

- Teams share setup through an ordinary git repository or shared folder they already control.
- Every pack and archive is read as hostile input; the secret scanner now also recognises GitHub, AWS and Slack
  tokens, which also improves redaction in logs and audit entries.
- Pack content is loaded from disk on each listing and run (packs are small); a cache can come later if needed.
- Imported notebooks may need their folders relinked before their runs are complete.
