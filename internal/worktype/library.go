package worktype

// LibraryPrompt is a ready-made prompt for a work type. A notebook copies one
// into the prompt box or saves it as its own prompt; the library itself never
// changes at run time.
type LibraryPrompt struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	WorkType string `json:"workType"`
	Text     string `json:"text"`
}

// Library is the starter prompt library, in display order.
var Library = []LibraryPrompt{
	{
		ID: "change-risk", Name: "Change risk review", WorkType: Analyze,
		Text: "Review the change in the context. What could break, for whom, and how likely is it? " +
			"Rate the overall risk, name the riskiest parts with file and line where you can, and say what to " +
			"test or roll out carefully before merging.",
	},
	{
		ID: "triage-issue", Name: "Triage an issue", WorkType: Analyze,
		Text: "Triage the issue described in the context. State the likely cause, how severe it is and who is " +
			"affected, what is still unknown, and the next step that would confirm or rule out the cause.",
	},
	{
		ID: "security-review", Name: "Security review", WorkType: Analyze,
		Text: "Look for security problems in the context: untrusted input, missing checks, secrets, unsafe " +
			"defaults, injection and access-control gaps. For each finding give the evidence, the impact and " +
			"a fix. Say plainly when you found nothing serious.",
	},
	{
		ID: "summarize", Name: "Summarize", WorkType: Summarize,
		Text: "Summarize the context for a busy reader: what it is about, what matters most, and what changed " +
			"or was decided. Keep each source's own point separate.",
	},
	{
		ID: "action-items", Name: "Extract action items", WorkType: Summarize,
		Text: "List every action item in the context: what has to be done, by whom and by when, as stated. " +
			"Mark items with no owner or no date, and do not invent either.",
	},
	{
		ID: "meeting-notes", Name: "Meeting notes", WorkType: Summarize,
		Text: "Turn the meeting material in the context into notes: decisions made, open questions, and action " +
			"items with owners. Leave out small talk and repetition.",
	},
	{
		ID: "draft-doc", Name: "Draft documentation", WorkType: Document,
		Text: "Write documentation for what the context describes, for someone who has to use or maintain it: " +
			"what it does, how to set it up, how to use it, and what to watch out for. Use the context's own " +
			"names, and say where it leaves something unclear.",
	},
	{
		ID: "how-to", Name: "How-to guide", WorkType: Document,
		Text: "Write a step-by-step how-to for the task in the context. Start with what the reader needs first, " +
			"number the steps, and end with how to check that it worked.",
	},
	{
		ID: "status-report", Name: "Status report", WorkType: Report,
		Text: "Write a status report from the context: what was done, what is in progress, what is blocked and " +
			"why, and what comes next. Lead with anything the reader must act on.",
	},
	{
		ID: "incident-report", Name: "Incident report", WorkType: Report,
		Text: "Write an incident report from the context: what happened and when, the impact, the cause as far " +
			"as the evidence shows, what was done to recover, and the follow-ups that would stop it happening " +
			"again. Blameless and factual.",
	},
	{
		ID: "changes-since-last-time", Name: "Changes since last time", WorkType: Report,
		Text: "Compare the context with the previous report below and write what changed since then: what is " +
			"new, what got better or worse, and what is still open. Keep anything that did not change short.\n\n" +
			"Previous report:\n{{previous}}",
	},
}

// LibraryFor returns the library prompts for a work type, or all when id is empty.
func LibraryFor(id string) []LibraryPrompt {
	out := []LibraryPrompt{}
	for _, p := range Library {
		if id == "" || p.WorkType == id {
			out = append(out, p)
		}
	}
	return out
}
