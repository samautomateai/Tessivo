/// A starter prompt from the core's library (`GET /v1/prompt-library`).
class LibraryPrompt {
  const LibraryPrompt({
    required this.id,
    required this.name,
    required this.workType,
    required this.text,
    this.pack = '',
    this.mark = '',
  });

  factory LibraryPrompt.fromJson(Map<String, Object?> j) => LibraryPrompt(
    id: j['id'] as String? ?? '',
    name: j['name'] as String? ?? '',
    workType: j['workType'] as String? ?? '',
    text: j['text'] as String? ?? '',
    pack: j['pack'] as String? ?? '',
    mark: j['mark'] as String? ?? '',
  );

  final String id;
  final String name;

  /// analyze, summarize, document or report.
  final String workType;
  final String text;

  /// The team pack it comes from, and whether the pack marks it required or recommended.
  final String pack;
  final String mark;
}

/// The prompts in a `{prompts: [...]}` reply.
List<LibraryPrompt> libraryFromJson(Object? j) => [
  if (j is Map && j['prompts'] is List)
    for (final p in j['prompts'] as List)
      if (p is Map<String, Object?>) LibraryPrompt.fromJson(p),
];
