# Profiles and smart defaults: settings profiles, work type suggestions

- Date: 2026-11-25
- Stories: SET-10, WT-7 (RUN-5 checked: covered by output threads, OUT-1 to OUT-4)
- Status: the user asked Claude to continue with the next items; the choices below are Claude's.

## Intent

People move between setups: a work LiteLLM proxy, a personal key, a laptop with only a local model on a plane.
Switching should be one step, not re-typing providers. And a new notebook should not have to guess its work type
when the sources already say what kind of work it is.

## Decisions (Claude's calls)

1. **Profiles (SET-10).** A profile is a named snapshot of providers, aliases, run defaults and token limits
   (`profiles: [{name, providers, aliases, defaults, limits}]`, `profile: <active name>` in the config; at most 10,
   names like MCP servers). *Save as profile* captures the current settings. *Use* first stores the current settings
   back into the active profile, so edits made since are kept, then applies the chosen one; it is validated like any
   settings change and refused as a whole if invalid. Keys stay in the keychain or environment: a profile only holds
   each provider's `api_key_ref`. MCP servers, integrations, agent rules and desktop preferences are not part of a
   profile. Runs already going keep the settings they started with. `tess profile list|save|use|delete`,
   `/v1/profiles`, Settings → Profiles, and the active profile shown in the status bar.
2. **Work type suggestion (WT-7).** From the enabled sources, by fixed rules (no model call): git changes, `.diff`
   or `.patch` files, or a code folder → Analyze; spreadsheets and CSV files make up most sources → Report; meeting
   transcripts (`.vtt`, `.srt`) or names with "meeting", "minutes" or "notes" → Summarize; Markdown or Word docs in
   a folder named like docs → Document. A suggestion is offered only while the notebook has no outputs and differs
   from its work type. The work type of a notebook can be changed only while it has no outputs (409 otherwise).
   `GET /v1/notebooks/{id}/worktype-suggestion`, `PATCH` `workType`, `tess notebook worktype <nb> [type]`, and a
   notice on the Sources tab with *Switch to …* and *Dismiss*.

## Non-goals

Offline enforcement per profile, profiles for MCP servers or integrations, model-based work type guessing.

## Constraints

OS independent; bounded (10 profiles, rules over at most the notebook's sources); every change documented.
