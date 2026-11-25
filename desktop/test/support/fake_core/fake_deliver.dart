import 'package:tessivo_desktop/src/core/errors.dart';
import 'package:tessivo_desktop/src/core/models.dart';

import 'fake_threads.dart';

/// Integrations, issues and publishing for the demo core.
mixin DemoDeliver on DemoThreads {
  final List<IntegrationInfo> demoIntegrations = [];
  final Map<String, List<Delivery>> _sent = {};

  /// When set, pushes and publishes to this integration fail like a tracker answering 401.
  String? failingIntegration;

  Future<List<IssueInfo>> outputIssues(String notebookId, String outputId) async {
    final o = demoOutputs.where((o) => o.id == outputId).firstOrNull;
    if (o == null) throw CoreException(404, 'output $outputId not found');
    if (o.workType != WorkTypeId.analyze) {
      throw CoreException(400, 'only Analyze outputs become issues; this one is ${o.workType.name}');
    }
    return const [
      IssueInfo(index: 0, title: 'Retries are not idempotent', severity: 'high', labels: ['payments', 'severity:high']),
      IssueInfo(index: 1, title: 'No alert on double charge', severity: 'medium', labels: ['severity:medium']),
    ];
  }

  IntegrationInfo _integration(String name, {required bool issues}) {
    final i = demoIntegrations.where((i) => i.name == name).firstOrNull;
    if (i == null) throw CoreException(404, 'integration $name not found');
    if (i.forIssues != issues) throw CoreException(400, 'integration $name takes no ${issues ? 'issues' : 'pages'}');
    if (name == failingIntegration) throw const CoreException(502, 'github: 401 Unauthorized: bad credentials');
    return i;
  }

  Future<PushResult> pushIssues(
    String notebookId,
    String outputId,
    String integration, {
    List<int> findings = const [],
    bool again = false,
  }) async {
    final issues = await outputIssues(notebookId, outputId);
    final i = _integration(integration, issues: true);
    final list = _sent[outputId] ??= [];
    final sent = <Delivery>[];
    final skipped = <int>[];
    for (final is_ in issues.where((x) => findings.isEmpty || findings.contains(x.index))) {
      if (!again && list.any((d) => d.integration == integration && d.finding == is_.index)) {
        skipped.add(is_.index);
        continue;
      }
      final d = Delivery(
        integration: integration,
        kind: i.kind,
        finding: is_.index,
        url: 'https://tracker.test/${list.length + sent.length + 1}',
      );
      sent.add(d);
    }
    list.addAll(sent);
    return PushResult(sent: sent, skipped: skipped);
  }

  Future<Delivery> publishOutput(String notebookId, String outputId, String integration) async {
    final i = _integration(integration, issues: false);
    final d = Delivery(integration: integration, kind: i.kind, finding: -1, url: 'https://wiki.test/$outputId');
    (_sent[outputId] ??= []).add(d);
    return d;
  }

  Future<List<Delivery>> deliveries(String notebookId, String outputId) async => List.of(_sent[outputId] ?? const []);

  /// Supplied by the demo settings (the fake keychain).
  Future<bool> secretSet(String ref);

  Future<List<IntegrationInfo>> listIntegrations() async => [
    for (final i in demoIntegrations)
      IntegrationInfo(
        name: i.name,
        kind: i.kind,
        baseUrl: i.baseUrl,
        project: i.project,
        tokenRef: i.tokenRef,
        space: i.space,
        parent: i.parent,
        path: i.path,
        tokenSet: i.kind == 'folder' || await secretSet(i.tokenRef),
      ),
  ];

  Future<IntegrationInfo> addIntegration(IntegrationInfo integration) async {
    if (!RegExp(r'^[a-z0-9][a-z0-9_-]{0,31}$').hasMatch(integration.name)) {
      throw const CoreException(400, 'integration name: use 1-32 lower-case letters, digits, - or _');
    }
    if (demoIntegrations.any((i) => i.name == integration.name)) {
      throw CoreException(409, 'integration ${integration.name} already exists');
    }
    demoIntegrations.add(integration);
    return integration;
  }

  Future<void> removeIntegration(String name) async {
    final before = demoIntegrations.length;
    demoIntegrations.removeWhere((i) => i.name == name);
    if (demoIntegrations.length == before) throw CoreException(404, 'integration $name not found');
  }

  Future<void> testIntegration(String name) async {
    if (name == failingIntegration) throw const CoreException(502, 'github: 401 Unauthorized: bad credentials');
    if (!demoIntegrations.any((i) => i.name == name)) throw CoreException(404, 'integration $name not found');
  }
}
