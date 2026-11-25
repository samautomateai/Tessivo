import 'json_mapping.dart';

/// A team pack as the core lists it (docs/api.md, "Packs").
class PackInfo {
  const PackInfo({
    required this.id,
    required this.name,
    required this.source,
    required this.kind,
    this.description = '',
    this.publisher = '',
    this.version = '',
    this.branch = '',
    this.fetchedAt = '',
    this.error = '',
    this.templates = 0,
    this.prompts = 0,
    this.servers = 0,
    this.problems = const [],
    this.readme = '',
  });

  factory PackInfo.fromJson(Map<String, Object?> j) => PackInfo(
    id: j['id'] as String? ?? '',
    name: j['name'] as String? ?? '',
    source: j['source'] as String? ?? '',
    kind: j['kind'] as String? ?? '',
    description: j['description'] as String? ?? '',
    publisher: j['publisher'] as String? ?? '',
    version: j['version'] as String? ?? '',
    branch: j['branch'] as String? ?? '',
    fetchedAt: j['fetchedAt'] as String? ?? '',
    error: j['error'] as String? ?? '',
    templates: (j['templates'] as num?)?.toInt() ?? 0,
    prompts: (j['prompts'] as num?)?.toInt() ?? 0,
    servers: (j['servers'] as num?)?.toInt() ?? 0,
    problems: [
      for (final p in j['problems'] as List? ?? const [])
        if (p is String) p,
    ],
    readme: j['readme'] as String? ?? '',
  );

  final String id;
  final String name;
  final String source;

  /// folder (read in place), zip (unpacked) or git (cloned and refreshed).
  final String kind;
  final String description;
  final String publisher;
  final String version;
  final String branch;
  final String fetchedAt;

  /// The last refresh or load error; the last good copy is still used.
  final String error;
  final int templates;
  final int prompts;
  final int servers;

  /// Items the pack holds that were skipped, with why.
  final List<String> problems;
  final String readme;
}

/// The packs in a `{packs: [...]}` reply.
List<PackInfo> packsFromJson(Map<String, Object?> j) => jsonList(j['packs']).map(PackInfo.fromJson).toList();
