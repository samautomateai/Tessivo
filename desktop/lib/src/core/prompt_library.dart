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
