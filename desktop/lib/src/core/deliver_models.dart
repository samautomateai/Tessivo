/// Integration kinds (docs/api.md, Integrations).
const integrationKinds = ['github', 'gitlab', 'jira', 'confluence', 'notion', 'folder'];

/// Whether an integration of [kind] takes issues (otherwise pages or files).
bool takesIssues(String kind) => kind == 'github' || kind == 'gitlab' || kind == 'jira';

/// A tracker, wiki or folder outputs are sent to. The token is never here:
/// [tokenRef] names the secret, and [tokenSet] says whether it is stored.
class IntegrationInfo {
  const IntegrationInfo({
    required this.name,
    required this.kind,
    this.baseUrl = '',
    this.project = '',
    this.tokenRef = '',
    this.user = '',
    this.issueType = '',
    this.space = '',
    this.parent = '',
    this.path = '',
    this.tokenSet = false,
  });

  factory IntegrationInfo.fromJson(Map<String, Object?> j) {
    String s(String k) => j[k] as String? ?? '';
    return IntegrationInfo(
      name: s('name'),
      kind: s('kind'),
      baseUrl: s('base_url'),
      project: s('project'),
      tokenRef: s('token_ref'),
      user: s('user'),
      issueType: s('issue_type'),
      space: s('space'),
      parent: s('parent'),
      path: s('path'),
      tokenSet: j['tokenSet'] as bool? ?? false,
    );
  }

  final String name;
  final String kind;
  final String baseUrl;
  final String project;
  final String tokenRef;
  final String user;
  final String issueType;
  final String space;
  final String parent;
  final String path;
  final bool tokenSet;

  bool get forIssues => takesIssues(kind);

  /// Where it sends to, for a list: the project, space, page or folder.
  String get target => [project, space, parent, path].where((s) => s.isNotEmpty).join(' · ');

  /// The POST body: only the fields that are set.
  Map<String, Object?> toBody() => {
    'name': name,
    'kind': kind,
    for (final (k, v) in [
      ('base_url', baseUrl),
      ('project', project),
      ('token_ref', tokenRef),
      ('user', user),
      ('issue_type', issueType),
      ('space', space),
      ('parent', parent),
      ('path', path),
    ])
      if (v.isNotEmpty) k: v,
  };
}

/// A tracker-ready issue made from an Analyze finding (WT-5).
class IssueInfo {
  const IssueInfo({
    required this.index,
    required this.title,
    this.severity = '',
    this.body = '',
    this.labels = const [],
  });

  factory IssueInfo.fromJson(Map<String, Object?> j) => IssueInfo(
    index: (j['index'] as num?)?.toInt() ?? 0,
    title: j['title'] as String? ?? '',
    severity: j['severity'] as String? ?? '',
    body: j['body'] as String? ?? '',
    labels: [for (final l in (j['labels'] as List<Object?>? ?? const [])) '$l'],
  );

  final int index;
  final String title;
  final String severity;
  final String body;
  final List<String> labels;
}

/// One thing sent from an output: an issue ([finding] ≥ 0) or a page (−1).
class Delivery {
  const Delivery({required this.integration, required this.finding, this.kind = '', this.url = '', this.at = ''});

  factory Delivery.fromJson(Map<String, Object?> j) => Delivery(
    integration: j['integration'] as String? ?? '',
    kind: j['kind'] as String? ?? '',
    finding: (j['finding'] as num?)?.toInt() ?? -1,
    url: j['url'] as String? ?? '',
    at: j['at'] as String? ?? '',
  );

  final String integration;
  final String kind;
  final int finding;
  final String url;
  final String at;
}

/// What a push sent, and the findings it skipped as sent before.
class PushResult {
  const PushResult({this.sent = const [], this.skipped = const []});

  factory PushResult.fromJson(Map<String, Object?> j) => PushResult(
    sent: [
      for (final d in (j['sent'] as List<Object?>? ?? const []))
        if (d is Map<String, Object?>) Delivery.fromJson(d),
    ],
    skipped: [for (final i in (j['skipped'] as List<Object?>? ?? const [])) (i as num).toInt()],
  );

  final List<Delivery> sent;
  final List<int> skipped;
}
