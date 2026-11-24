import 'models.dart';

export 'chat_models.dart';
export 'checks.dart';
export 'core_api.dart';
export 'errors.dart';
export 'mcp_models.dart';
export 'model_check.dart';
export 'prompt_library.dart';
export 'research_models.dart';
export 'thread_models.dart';
export 'usage.dart';

const builtInWorkTypes = <WorkType>[
  WorkType(
    id: WorkTypeId.analyze,
    name: 'Analyze',
    question: "What does this mean, and what's the risk?",
    starterPrompt: 'Assess the risk and impact described by the context.',
    suggestedContext: ['Code base', 'Diff or PR', 'Documents', 'Spreadsheet or CSV', 'MCP server'],
  ),
  WorkType(
    id: WorkTypeId.summarize,
    name: 'Summarize',
    question: 'What do I need to know?',
    starterPrompt: 'Summarize these sources for a busy reader.',
    suggestedContext: ['Documents', 'Transcripts', 'Threads', 'URLs'],
  ),
  WorkType(
    id: WorkTypeId.document,
    name: 'Document',
    question: 'How does this work?',
    starterPrompt: 'Explain what this is and how it fits together.',
    suggestedContext: ['Code base', 'Existing docs', 'Notes'],
  ),
  WorkType(
    id: WorkTypeId.report,
    name: 'Report',
    question: "What happened, and what's next?",
    starterPrompt: "Produce this period's report.",
    suggestedContext: ['Logs', 'Tickets CSV', 'Metrics spreadsheet', 'Previous report'],
  ),
];
