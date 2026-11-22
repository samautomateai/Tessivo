import 'chat_models.dart';
import 'json_mapping.dart';

/// A research notebook (docs/api.md, "Research notebooks").
class ResearchNotebook {
  const ResearchNotebook({
    required this.id,
    required this.name,
    this.description = '',
    this.sources = const [],
    this.notes = 0,
  });

  factory ResearchNotebook.fromJson(Map<String, Object?> j) => ResearchNotebook(
    id: j['id'] as String? ?? '',
    name: j['name'] as String? ?? '',
    description: j['description'] as String? ?? '',
    sources: jsonList(j['sources']).map(ResearchSource.fromJson).toList(),
    notes: (j['notes'] as num?)?.toInt() ?? 0,
  );

  final String id;
  final String name;
  final String description;
  final List<ResearchSource> sources;
  final int notes;
}

/// A source and its summary state: ready, failed, pending or updating.
class ResearchSource {
  const ResearchSource({
    required this.id,
    required this.label,
    required this.kind,
    required this.level,
    required this.summary,
    this.tokens = 0,
    this.error,
  });

  factory ResearchSource.fromJson(Map<String, Object?> j) => ResearchSource(
    id: j['id'] as String? ?? '',
    label: j['label'] as String? ?? '',
    kind: j['kind'] as String? ?? 'file',
    level: j['level'] as String? ?? 'full',
    summary: j['summary'] as String? ?? 'pending',
    tokens: (j['tokens'] as num?)?.toInt() ?? 0,
    error: j['error'] as String?,
  );

  final String id;
  final String label;
  final String kind; // file, folder or mcp-resource
  final String level; // full, summary or off
  final String summary;
  final int tokens;
  final String? error;

  /// A summary exists to send at the summary level.
  bool get hasSummary => summary == 'ready' || summary == 'updating';

  ResearchSource copyWith({String? level, String? summary}) => ResearchSource(
    id: id,
    label: label,
    kind: kind,
    level: level ?? this.level,
    summary: summary ?? this.summary,
    tokens: tokens,
    error: error,
  );
}

/// A note kept in a research notebook; origin is human or ai.
class ResearchNote {
  const ResearchNote({
    required this.id,
    required this.title,
    required this.text,
    this.origin = 'human',
    this.include = true,
  });

  factory ResearchNote.fromJson(Map<String, Object?> j) => ResearchNote(
    id: j['id'] as String? ?? '',
    title: j['title'] as String? ?? '',
    text: j['text'] as String? ?? '',
    origin: j['origin'] == 'ai' ? 'ai' : 'human',
    include: j['include'] as bool? ?? false,
  );

  final String id;
  final String title;
  final String text;
  final String origin;
  final bool include;

  bool get isAi => origin == 'ai';
}

/// A resolved [n] or [n:path] in an answer.
class Citation {
  const Citation({required this.n, required this.kind, required this.id, required this.label, this.level, this.path});

  factory Citation.fromJson(Map<String, Object?> j) => Citation(
    n: (j['n'] as num?)?.toInt() ?? 0,
    kind: j['kind'] as String? ?? 'source',
    id: j['id'] as String? ?? '',
    label: j['label'] as String? ?? '',
    level: j['level'] as String?,
    path: j['path'] as String?,
  );

  final int n;
  final String kind;
  final String id;
  final String label;
  final String? level;
  final String? path;

  String get reference => [label, if (path != null) ': $path', if (level != null) ' ($level)'].join();
}

/// What a question would send now.
class ContextTotals {
  const ContextTotals({this.sources = 0, this.summaries = 0, this.notes = 0, this.tokens = 0, this.cut = const []});

  factory ContextTotals.fromJson(Map<String, Object?> j) => ContextTotals(
    sources: (j['sources'] as num?)?.toInt() ?? 0,
    summaries: (j['summaries'] as num?)?.toInt() ?? 0,
    notes: (j['notes'] as num?)?.toInt() ?? 0,
    tokens: (j['tokens'] as num?)?.toInt() ?? 0,
    cut: [for (final c in j['cut'] is List ? j['cut']! as List : const []) ?c as String?],
  );

  final int sources;
  final int summaries;
  final int notes;
  final int tokens;
  final List<String> cut;

  String get label {
    final base = '$sources sources · $summaries summaries · ≈ ${tokens ~/ 1000}k tokens';
    return cut.isEmpty ? base : '$base · ${cut.length} will be shortened';
  }
}

/// A source's text and summary.
class ResearchSourceText {
  const ResearchSourceText({required this.text, required this.summary});
  final String text;
  final String summary;
}

/// A conversation with its answers' citations.
class ResearchChat {
  const ResearchChat({required this.session, this.citations = const {}});
  final ChatSession session;
  final Map<String, List<Citation>> citations;
}

/// Events while an answer streams.
sealed class ResearchEvent {
  const ResearchEvent();
}

class ResearchDelta extends ResearchEvent {
  const ResearchDelta(this.text);
  final String text;
}

class ResearchFinished extends ResearchEvent {
  const ResearchFinished(this.user, this.assistant, {this.citations = const [], this.skipped = const []});
  final ChatMessage user;
  final ChatMessage assistant;
  final List<Citation> citations;
  final List<String> skipped;
}
