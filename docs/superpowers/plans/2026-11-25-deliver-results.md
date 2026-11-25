# Deliver Results Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Findings become tracker-ready issues (WT-5), pushed to GitHub, GitLab or Jira (REP-4); outputs publish to
a folder, Confluence or Notion (REP-5).

**Architecture:** Findings in the Analyze schema and `internal/service/issues.go`; integration config in
`internal/config`; a new `internal/deliver` package (HTTP client, trackers, publishers, Markdown converters);
service, API, CLI and desktop on top.

**Spec:** `docs/superpowers/specs/2026-11-25-deliver-results-design.md`

## Global Constraints

- No new dependencies; OS independent; https or loopback http only; no credentials in URLs.
- HTTP: 30 s per request, bodies ≤ 1 MiB, no redirects, error text ≤ 300 chars, redacted; tokens only in headers.
- Files 0600, atomic; lines ≤ 120; files ≤ 500 lines (tests ≤ 800); coverage ≥ 80 %; every change updates docs.

## Review Focus

- An Analyze output saved before findings existed: still becomes an issue, and still edits (Task 1).
- A tracker answering with an error page or a huge body: bounded, redacted, no token in the message (Task 3).
- Pushing the same findings twice: skipped unless asked again (Task 5).
- Markdown with HTML, scripts or `]]>` in it published to Confluence: escaped, never live markup (Task 4).
- A folder target outside a writable place, or a title with path characters: refused or made safe (Task 4).

---

### Task 1: Findings and issues (core, WT-5)
- Analyze schema `findings` (required); `RequiredOrder` puts it after `risk_level`; old outputs edit with `[]` filled.
- `service.Issue{Index, Title, Severity, Body, Labels}`; `(*Service).Issues(nbID, outputID) ([]Issue, error)`.
- API `GET /v1/notebooks/{id}/outputs/{outputId}/issues` → `{issues}`; CLI `tess output issues <nb> <id> [--json]`.
- Tests: from findings; legacy output → one issue; non-Analyze refused; edit of a legacy output works.
- Commit `feat: findings become tracker-ready issues`.

### Task 2: Integrations config
- `config.Integration` + `File.Integrations`; `Check()` (kind, name, URL rules, required fields per kind).
- Service `ListIntegrations`, `AddIntegration`, `RemoveIntegration`; API `GET/POST /v1/integrations`,
  `DELETE /v1/integrations/{name}`; CLI `tess integration add|list|remove`.
- Tests: validation table; add/remove; duplicate refused.
- Commit `feat: integrations config`.

### Task 3: deliver client and trackers
- `deliver.Client` (bounded, no redirects, redacted errors); `deliver.Tracker` for github, gitlab, jira:
  `CreateIssue(ctx, Issue) (Ref, error)`, `Test(ctx) error`.
- Tests with httptest: request shape and auth per kind; error body cut and redacted; oversized body bounded.
- Commit `feat: GitHub, GitLab and Jira issue clients`.

### Task 4: publishers and converters
- `deliver.HTML(md)` (escaped storage XHTML), `deliver.NotionBlocks(md)` (≤ 2000-char rich text);
  publishers folder, confluence (create or update by page id), notion (create, children in chunks of 100).
- Tests: converters escape; folder writes 0600 safe name; confluence update bumps the version; notion chunks.
- Commit `feat: publish to a folder, Confluence or Notion`.

### Task 5: service, API and CLI for push and publish
- `PushIssues(ctx, nbID, outID, integration, indexes, again)`, `Publish(ctx, nbID, outID, integration)`,
  `TestIntegration`; delivery records `outputs/<id>.delivered.json`; audit entries.
- API `POST …/outputs/{id}/issues/push`, `POST …/outputs/{id}/publish`, `GET …/outputs/{id}/deliveries`,
  `POST /v1/integrations/{name}/test`; CLI `tess output push|publish`, `tess integration test`.
- Tests: push records and skips; again re-sends; publish records; audit has no token.
- Commit `feat: push issues and publish outputs`.

### Task 6: Desktop
- Client methods; output toolbar *Send* menu (`send`, `send-issues`, `send-publish`); issues dialog
  (`issue-<n>` checkboxes, `issue-target`, `issues-push`); publish dialog (`publish-target`, `publish-go`);
  Settings → Integrations (list, add, remove, test, token).
- Tests for each; 200 % zoom on the dialogs.
- Commit `feat(desktop): send issues and publish outputs`.

### Task 7: Docs and checks
- api.md, user guide, checklist (WT-5, REP-4, REP-5), architecture (`deliver`), security model; `task ci`.
- Commit `docs: deliver results`.
