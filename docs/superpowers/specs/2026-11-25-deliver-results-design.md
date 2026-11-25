# Deliver results: issues from findings, push to trackers, publish to wikis

- Date: 2026-11-25
- Stories: WT-5 (MVP), REP-4, REP-5
- Status: the user asked Claude to continue with the next items and to choose sensibly; the choices below are
  Claude's.

## Intent

An output is only useful once it reaches the place where people work. Analyze findings should become issues in the
team's tracker without copy and paste, and a document should land in the team's wiki or a shared folder. Every send
is something the user starts and confirms; nothing leaves the machine on its own.

## Decisions (Claude's calls)

1. **Findings (WT-5).** The Analyze schema gains a required `findings` list: `{title, severity (low, medium, high,
   critical), detail, steps: [string], labels: [string]}` (strict structured output needs every field required).
   A tracker-ready issue is built from each finding: title, severity, a Markdown body (detail, numbered steps, the
   output's evidence and a footer naming the notebook and output) and labels (the finding's, plus
   `severity:<level>`). An Analyze output saved before findings existed becomes one issue made from the whole output
   (title, risk level, summary, evidence, mitigations as steps, impacted areas as labels). Editing such an output
   fills the missing list with `[]`. Other work types are refused with a clear message.
   `tess output issues`, `GET …/outputs/{id}/issues`.
2. **Integrations.** A config section `integrations`: `{name, kind, base_url, project, token_ref, user?, issue_type?,
   space?, parent?, path?}` with kinds `github`, `gitlab`, `jira` (issues), `confluence`, `notion` (pages) and
   `folder` (files). Tokens never sit in config: `token_ref` names a secret in the OS keychain or the environment,
   like provider keys. URLs must be https, or http to a loopback host; no credentials in URLs. Managed with
   `tess integration add|list|remove|test`, `/v1/integrations`, and Settings → Integrations (the token is set there
   through the existing secrets API).
3. **Push issues (REP-4).** The user picks findings and a tracker; Tessivo creates one issue per finding (GitHub
   REST, GitLab REST v4, Jira REST v2 with Bearer or, when `user` is set, Basic auth). Each sent finding is recorded
   in `outputs/<id>.delivered.json` (integration, finding index, URL, time) and skipped on a later push unless asked
   again. Each push is audited (integration, kind, count, URLs; never the token).
4. **Publish (REP-5).** Any output can be published: to a folder (`<title>.md`, written atomically, 0600), to
   Confluence (a page in `space`, under `parent` if set, body converted from Markdown to storage XHTML with
   everything escaped; publishing the same output again updates its page) or to Notion (a child page of `parent`,
   Markdown converted to blocks, sent 100 blocks per request). Recorded and audited like pushes.
5. **HTTP.** One small client for all of them: 30 s per request, response bodies read up to 1 MiB, no redirects,
   error text cut to 300 characters and redacted, the token only in headers. Standard library only; no new
   dependencies.
6. **Desktop.** The output toolbar gets *Send*: *Create issues…* (Analyze outputs: the issues with checkboxes, the
   tracker, already-sent ones marked) and *Publish…* (the target). Settings gets an Integrations section.

## Non-goals

Syncing issue status back, editing issues after creation, OAuth flows, attachments, publishing to other wikis.

## Constraints

OS independent; bounded sizes, counts and time; no secrets in argv, logs, errors or audit entries; files 0600;
every change documented (api.md, user guide, checklist, architecture, ADR 0003 unchanged: no new dependency).
