package template

import "github.com/samautomateai/tessivo/internal/config"

func slot(name, kind, desc string, required bool) Slot {
	return Slot{Name: name, Kind: kind, Description: desc, Required: required}
}

func prompt(name, text string) Prompt { return Prompt{Name: name, Text: text} }

func builtinList() []Template {
	out := analyzeTemplates()
	out = append(out, documentTemplates()...)
	out = append(out, summarizeTemplates()...)
	return append(out, reportTemplates()...)
}

func analyzeTemplates() []Template {
	agent := config.RunSettings{Mode: config.ModeAgent, ThinkingEffort: config.ThinkingMedium}
	return []Template{
		{
			ID: "change-risk-review", Name: "Change risk review", WorkType: "analyze", Settings: agent,
			Description: "Score the risk of a code change, list impacted areas with evidence and suggest mitigations.",
			Prompts: []Prompt{
				prompt("full-review", "Assess the risk of merging this change into {{service}}. Focus on {{area}}. "+
					"Link each finding to evidence in the diff, the code base or past incidents."),
				prompt("quick-scan", "Give a fast risk scan of this change: top 3 risks and the one test to add."),
				prompt("security-focus", "Review this change for security risks: input handling, auth, secrets, "+
					"injection and unsafe defaults."),
			},
			Slots: []Slot{
				slot("Code base", "code base", "The repository the change applies to", true),
				slot("Change", "diff", "A diff, branch or PR", true),
				slot("Incident history", "spreadsheet", "Past incidents to correlate (sheet or MCP)", false),
			},
		},
		{
			ID: "dependency-upgrade-risk", Name: "Dependency upgrade risk", WorkType: "analyze",
			Description: "Assess upgrading a dependency: breaking changes, affected code and a safe rollout plan.",
			Prompts: []Prompt{prompt("assess", "Assess the risk of upgrading {{dependency}} from {{from}} to {{to}}. "+
				"Find affected code, likely breaking changes and a rollout plan.")},
			Slots: []Slot{
				slot("Code base", "code base", "The repository using the dependency", true),
				slot("Changelog", "documents", "Release notes or changelog of the dependency", false),
			},
		},
		{
			ID: "customer-feedback-themes", Name: "Customer feedback themes", WorkType: "analyze",
			Description: "Top themes, frequency and evidence from surveys and support conversations.",
			Prompts: []Prompt{prompt("themes", "Find the top themes in this customer feedback, how often "+
				"each appears, representative quotes and recommended actions.")},
			Slots: []Slot{
				slot("Survey", "spreadsheet", "Survey export", true),
				slot("Support", "documents", "Support emails or chat exports", false),
			},
		},
		{
			ID: "spreadsheet-insights", Name: "Spreadsheet insights", WorkType: "analyze",
			Description: "Patterns, outliers and drivers in a sheet, using statistics Tessivo computes.",
			Prompts: []Prompt{prompt("insights", "Analyze this data: notable patterns, outliers, likely "+
				"drivers and questions worth investigating. Use the computed statistics; do not guess numbers.")},
			Slots: []Slot{slot("Data", "spreadsheet", "XLSX or CSV", true)},
		},
	}
}

func documentTemplates() []Template {
	return []Template{
		{
			ID: "code-base-overview", Name: "Code base overview", WorkType: "document",
			Description: "Explain what a repository is, its architecture, modules and how to run it.",
			Prompts: []Prompt{
				prompt("overview", "Explain what this code base is and how it fits together, for a new engineer."),
				prompt("module", "Document the {{module}} module: purpose, key types, flows and gotchas."),
			},
			Slots: []Slot{slot("Code base", "code base", "The repository to document", true)},
		},
		{
			ID: "onboarding-guide", Name: "Onboarding guide", WorkType: "document",
			Description: "Turn docs, notes and code into a first-week guide for a new team member.",
			Prompts: []Prompt{
				prompt("guide", "Write a first-week onboarding guide for a new {{role}} on this team."),
			},
			Slots: []Slot{
				slot("Team docs", "documents", "Wiki pages, READMEs, runbooks", true),
				slot("Code base", "code base", "Main repository (optional)", false),
			},
		},
		{
			ID: "incident-postmortem", Name: "Incident postmortem", WorkType: "document",
			Description: "Timeline, root cause, impact and follow-ups from incident data.",
			Prompts: []Prompt{prompt("postmortem", "Write a blameless postmortem for {{incident}}: timeline, "+
				"impact, root cause, what went well, follow-ups with owners.")},
			Slots: []Slot{
				slot("Incident data", "documents", "Logs, chat exports, tickets", true),
				slot("Metrics", "spreadsheet", "Error rates or latency during the incident", false),
			},
		},
		{
			ID: "release-notes", Name: "Release notes", WorkType: "document",
			Description: "User-facing release notes from merged changes and tickets.",
			Prompts: []Prompt{prompt("notes", "Write user-facing release notes for {{version}}. Group by "+
				"feature, fix and breaking change; skip internal refactors.")},
			Slots: []Slot{
				slot("Changes", "diff", "Diff range or merged PR list", true),
				slot("Tickets", "spreadsheet", "Ticket export for the release", false),
			},
		},
	}
}

func summarizeTemplates() []Template {
	return []Template{
		{
			ID: "document-set-summary", Name: "Document set summary", WorkType: "summarize",
			Description: "TL;DR, key points and per-source notes for a folder of documents.",
			Prompts: []Prompt{prompt("summary", "Summarize these documents for a busy reader. "+
				"Note where sources disagree.")},
			Slots: []Slot{slot("Documents", "documents", "PDF, Word, slides, notes", true)},
		},
		{
			ID: "meeting-notes-to-actions", Name: "Meeting notes to actions", WorkType: "summarize",
			Description: "Decisions, owners and action items from notes or transcripts.",
			Prompts: []Prompt{prompt("actions", "List the decisions, open questions and action items "+
				"(owner, due date if stated) from these meeting notes.")},
			Slots: []Slot{slot("Notes", "documents", "Meeting notes or transcripts", true)},
		},
		{
			ID: "contract-digest", Name: "Contract digest", WorkType: "summarize",
			Description: "Terms, renewals, costs and obligations across a set of contracts.",
			Prompts: []Prompt{prompt("digest", "Summarize these contracts per vendor: term, renewal date and "+
				"notice period, costs, key obligations and risks.")},
			Slots: []Slot{
				slot("Contracts", "documents", "Contract PDFs or Word files", true),
				slot("Pricing", "spreadsheet", "Pricing or spend sheet", false),
			},
			Memory: []string{"Flag any auto-renewal with a notice period under 60 days."},
		},
	}
}

func reportTemplates() []Template {
	return []Template{
		{
			ID: "weekly-ops-report", Name: "Weekly ops report", WorkType: "report",
			Description: "Highlights, issues and trends versus last week. " +
				"Ships with a Monday 08:00 schedule.",
			Prompts: []Prompt{prompt("weekly", "Produce this week's ops report: highlights, issues with owners, "+
				"trends versus last week and next steps.")},
			Slots: []Slot{
				slot("Tickets", "spreadsheet", "Ticket export for the week", true),
				slot("Metrics", "spreadsheet", "Key metrics sheet", false),
				slot("Last report", "documents", "Previous week's report", false),
			},
			Schedule: &Schedule{Cron: "0 8 * * MON", PromptName: "weekly", Missed: "run_once"},
		},
	}
}

var builtins = func() map[string]Template {
	m := map[string]Template{}
	for _, t := range builtinList() {
		t.Version, t.Source = 1, SourceBuiltin
		m[t.ID] = t
	}
	return m
}()

// Builtins returns copies of the built-in templates.
func Builtins() []Template {
	out := make([]Template, 0, len(builtins))
	for _, t := range builtinList() {
		out = append(out, builtins[t.ID])
	}
	return out
}

func builtinByID(id string) (Template, bool) {
	t, ok := builtins[id]
	return t, ok
}
