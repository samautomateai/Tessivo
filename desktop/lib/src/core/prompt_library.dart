/// A starter prompt from the core's library (`GET /v1/prompt-library`).
class LibraryPrompt {
  const LibraryPrompt({required this.id, required this.name, required this.workType, required this.text});

  factory LibraryPrompt.fromJson(Map<String, Object?> j) => LibraryPrompt(
    id: j['id'] as String? ?? '',
    name: j['name'] as String? ?? '',
    workType: j['workType'] as String? ?? '',
    text: j['text'] as String? ?? '',
  );

  final String id;
  final String name;

  /// analyze, summarize, document or report.
  final String workType;
  final String text;
}

/// The prompts in a `{prompts: [...]}` reply.
List<LibraryPrompt> libraryFromJson(Object? j) => [
  if (j is Map && j['prompts'] is List)
    for (final p in j['prompts'] as List)
      if (p is Map<String, Object?>) LibraryPrompt.fromJson(p),
];

/// A few entries of the library, for the demo client.
const demoLibrary = [
  LibraryPrompt(
    id: 'change-risk',
    name: 'Change risk review',
    workType: 'analyze',
    text: 'Review the change in the context. What could break, for whom, and how likely is it?',
  ),
  LibraryPrompt(
    id: 'action-items',
    name: 'Extract action items',
    workType: 'summarize',
    text: 'List every action item in the context: what has to be done, by whom and by when, as stated.',
  ),
  LibraryPrompt(
    id: 'status-report',
    name: 'Status report',
    workType: 'report',
    text: 'Write a status report from the context: done, in progress, blocked and next.',
  ),
];
