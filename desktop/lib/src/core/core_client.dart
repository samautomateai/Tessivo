import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'checks.dart';
import 'core_api.dart';
import 'demo_chat.dart';
import 'demo_mcp.dart';
import 'demo_prompts.dart';
import 'demo_research.dart';
import 'demo_settings.dart';
import 'demo_templates.dart';
import 'demo_threads.dart';
import 'errors.dart';
import 'models.dart';

export 'chat_models.dart';
export 'checks.dart';
export 'core_api.dart';
export 'errors.dart';
export 'mcp_models.dart';
export 'research_models.dart';
export 'thread_models.dart';

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

/// In-memory core with sample data, used when no core can be reached.
class DemoCoreClient
    with DemoChat, DemoThreads, DemoMcp, DemoResearch, DemoPrompts, DemoSettings
    implements CoreClient {
  DemoCoreClient({Duration tick = const Duration(milliseconds: 40)}) : _tick = tick;

  final Duration _tick;

  @override
  Duration get chatTick => _tick;
  int _nextId = 100;
  final Map<String, List<ScheduleInfo>> _schedules = {
    'nb-4': const [
      ScheduleInfo(
        id: 's1',
        cron: '0 8 * * MON',
        missed: 'run_once',
        enabled: true,
        prompt: "Produce this week's ops report",
        nextRun: 'Mon 08:00',
        history: [ScheduleRun(status: 'ok', startedAt: '2026-11-16 08:00')],
      ),
    ],
  };
  final _events = StreamController<CoreEvent>.broadcast();
  final List<TemplateInfo> _templates = [...demoTemplates];

  final List<Notebook> _notebooks = [
    const Notebook(
      id: 'nb-1',
      name: 'Change risk review',
      workType: WorkTypeId.analyze,
      context: [
        ContextItem(id: 'c1', label: 'payments-service', kind: 'Code base', enabled: true, estimatedTokens: 38000),
        ContextItem(id: 'c2', label: 'PR #482 diff', kind: 'Diff', enabled: true, estimatedTokens: 9000),
        ContextItem(id: 'c3', label: 'incidents-2026.xlsx', kind: 'Spreadsheet', enabled: true, estimatedTokens: 4000),
      ],
    ),
    const Notebook(id: 'nb-2', name: 'Vendor contracts digest', workType: WorkTypeId.summarize, context: []),
    const Notebook(id: 'nb-3', name: 'Payments service docs', workType: WorkTypeId.document, context: []),
    const Notebook(
      id: 'nb-4',
      name: 'Weekly ops report',
      workType: WorkTypeId.report,
      context: [],
      schedule: 'Mon 08:00',
    ),
  ];

  @override
  Future<List<WorkType>> listWorkTypes() async => builtInWorkTypes;

  @override
  Future<List<Notebook>> listNotebooks() async => List.unmodifiable(_notebooks);

  @override
  Future<Notebook> createNotebook({required String name, required WorkTypeId workType}) async {
    final nb = Notebook(id: 'nb-${_nextId++}', name: name, workType: workType, context: const []);
    _notebooks.add(nb);
    return nb;
  }

  @override
  Future<Notebook> updateNotebook(String notebookId, {String? name, bool? archived}) async {
    if (name != null && name.trim().isEmpty) throw const CoreException(400, 'name is required');
    final i = _notebooks.indexWhere((n) => n.id == notebookId);
    if (i < 0) throw CoreException(404, 'notebook $notebookId not found');
    return _notebooks[i] = _notebooks[i].copyWith(name: name?.trim(), archived: archived);
  }

  @override
  Future<void> deleteNotebook(String notebookId) async {
    if (!_notebooks.any((n) => n.id == notebookId)) throw CoreException(404, 'notebook $notebookId not found');
    _notebooks.removeWhere((n) => n.id == notebookId);
  }

  @override
  Future<Notebook> setContextEnabled({
    required String notebookId,
    required String contextId,
    required bool enabled,
  }) async {
    final i = _notebooks.indexWhere((n) => n.id == notebookId);
    if (i < 0) {
      throw StateError('unknown notebook $notebookId');
    }
    final nb = _notebooks[i];
    final updated = nb.copyWith(
      context: [for (final c in nb.context) c.id == contextId ? c.copyWith(enabled: enabled) : c],
    );
    _notebooks[i] = updated;
    return updated;
  }

  @override
  Stream<RunEvent> startRun({
    required String notebookId,
    required String prompt,
    required Map<String, String> vars,
    String? promptName,
  }) async* {
    yield const RunStatus('Generating output');
    const parts = ['# Risk report\n\n', '## Risk score\n\n7 of 10, high\n\n', '## Mitigations\n\n- Add idempotency\n'];
    for (final p in parts) {
      await Future<void>.delayed(_tick);
      yield RunDelta(p);
    }
    yield RunFinished(
      RunOutcome(
        valid: true,
        modelUsed: 'litellm/model-b',
        totalTokens: 63000,
        markdown: parts.join(),
        fallbackNote: 'smart fell back to model B (rate limit)',
      ),
    );
  }

  @override
  Future<List<ScheduleInfo>> listSchedules(String notebookId) async =>
      List.unmodifiable(_schedules[notebookId] ?? const <ScheduleInfo>[]);

  @override
  Future<ScheduleInfo> addSchedule(
    String notebookId, {
    required String cron,
    String prompt = '',
    String missed = 'skip',
  }) async {
    if (cron.trim().split(RegExp(r'\s+')).length != 5 && !cron.startsWith('@')) {
      throw const FormatException('want 5 fields (minute hour day month weekday)');
    }
    final s = ScheduleInfo(
      id: 's${_nextId++}',
      cron: cron,
      missed: missed,
      enabled: true,
      prompt: prompt,
      nextRun: 'soon',
    );
    _schedules[notebookId] = [...?_schedules[notebookId], s];
    return s;
  }

  @override
  Future<ScheduleInfo> setScheduleEnabled(String notebookId, String scheduleId, {required bool enabled}) async {
    final list = [...?_schedules[notebookId]];
    final i = list.indexWhere((s) => s.id == scheduleId);
    if (i < 0) throw StateError('unknown schedule $scheduleId');
    list[i] = list[i].copyWith(enabled: enabled);
    _schedules[notebookId] = list;
    return list[i];
  }

  @override
  Future<void> deleteSchedule(String notebookId, String scheduleId) async {
    _schedules[notebookId] = [...?_schedules[notebookId]]..removeWhere((s) => s.id == scheduleId);
  }

  @override
  Stream<CoreEvent> events() => _events.stream;

  /// Emits a demo event (used by tests and the demo mode).
  void emit(CoreEvent e) => _events.add(e);

  // One sample approval so demo mode shows the approvals flow.
  final List<PendingApproval> _approvals = [
    const PendingApproval(
      runId: 'demo-run',
      notebookId: 'nb-1',
      notebookName: 'Change risk review',
      approvalId: 'demo-approval',
      tool: 'grep',
      args: '{"pattern":"TODO"}',
      requestedAt: '2026-11-21T10:00:00Z',
      expiresAt: '2026-11-28T10:00:00Z',
    ),
  ];

  @override
  Future<List<PendingApproval>> pendingApprovals() async => List.unmodifiable(_approvals);

  @override
  Future<void> answerApproval(String runId, String approvalId, {required bool allow, String? remember}) async {
    _approvals.removeWhere((a) => a.runId == runId && a.approvalId == approvalId);
  }

  @override
  Future<void> cancelRun(String runId) async => _approvals.removeWhere((a) => a.runId == runId);

  @override
  Future<void> resumeRun(String runId) async => _approvals.removeWhere((a) => a.runId == runId);

  Notebook _find(String id) =>
      _notebooks.firstWhere((n) => n.id == id, orElse: () => throw StateError('unknown notebook $id'));

  void _replace(Notebook nb) {
    _notebooks[_notebooks.indexWhere((n) => n.id == nb.id)] = nb;
  }

  @override
  Future<Notebook> getNotebook(String notebookId) async => _find(notebookId);

  @override
  Future<Notebook> addContext(String notebookId, String path) async {
    final nb = _find(notebookId);
    checkContextPath(path);
    final name = path.split(RegExp(r'[/\\]')).lastWhere((p) => p.isNotEmpty, orElse: () => path);
    final dot = name.lastIndexOf('.');
    final item = ContextItem(
      id: 'c${_nextId++}',
      label: name,
      kind: dot > 0 ? name.substring(dot + 1).toUpperCase() : 'Folder',
      enabled: true,
      estimatedTokens: 2000,
    );
    final updated = nb.copyWith(context: [...nb.context, item]);
    _replace(updated);
    return updated;
  }

  @override
  Future<Notebook> addUrlContext(String notebookId, String url, {String label = ''}) async {
    final u = Uri.tryParse(url);
    if (u == null || !(u.isScheme('http') || u.isScheme('https')) || u.host.isEmpty || u.userInfo.isNotEmpty) {
      throw const CoreException(400, 'not an http or https URL without credentials');
    }
    return _addItem(notebookId, label.isEmpty ? '${u.host}${u.path}' : label, 'Web page', 0);
  }

  @override
  Future<Notebook> addTextContext(String notebookId, String text, {String label = ''}) async {
    if (text.trim().isEmpty) throw const CoreException(400, 'text must not be empty');
    return _addItem(notebookId, label.isEmpty ? 'Pasted text' : label, 'MD', text.length ~/ 4);
  }

  Notebook _addItem(String notebookId, String label, String kind, int tokens) {
    final nb = _find(notebookId);
    final item = ContextItem(id: 'c${_nextId++}', label: label, kind: kind, enabled: true, estimatedTokens: tokens);
    final updated = nb.copyWith(context: [...nb.context, item]);
    _replace(updated);
    return updated;
  }

  @override
  Future<Notebook> setNotebookMcp(String notebookId, List<String> servers) async {
    for (final s in servers) {
      if (!demoMcpServers.any((m) => m.name == s)) throw CoreException(400, 'mcp server "$s" is not configured');
    }
    final updated = _find(notebookId).copyWith(mcpServers: List.unmodifiable(servers.toSet().toList()));
    _replace(updated);
    return updated;
  }

  @override
  Future<Notebook> addMcpContext(String notebookId, String server, String uri) async {
    if (!demoMcpServers.any((m) => m.name == server)) {
      throw CoreException(400, 'mcp server "$server" is not configured');
    }
    if (uri.trim().isEmpty) throw const CoreException(400, 'give a resource URI');
    final nb = _find(notebookId);
    final item = ContextItem(id: 'c${_nextId++}', label: uri, kind: 'MCP · $server', enabled: true, estimatedTokens: 0);
    final updated = nb.copyWith(context: [...nb.context, item]);
    _replace(updated);
    return updated;
  }

  @override
  Future<Notebook> setMemory(String notebookId, List<String> memory) async {
    checkMemory(memory);
    final updated = _find(notebookId).copyWith(memory: List.unmodifiable(memory));
    _replace(updated);
    return updated;
  }

  @override
  Future<OutputDetail> getOutput(String notebookId, String outputId) async {
    final o = (await listOutputs()).firstWhere(
      (o) => o.id == outputId && o.notebookId == notebookId,
      orElse: () => throw StateError('unknown output $outputId'),
    );
    return OutputDetail(id: o.id, title: o.title, markdown: '# ${o.title}\n\nSample output from ${o.notebookName}.');
  }

  @override
  Future<List<SearchHit>> search(String query) async {
    final words = query.toLowerCase().split(RegExp(r'\s+')).where((w) => w.isNotEmpty).toList();
    if (words.isEmpty) throw const CoreException(400, 'give words to search for');
    bool hit(String text) => words.every(text.toLowerCase().contains);
    return [
      for (final n in _notebooks)
        if (hit(n.name)) SearchHit(kind: 'notebook', notebookId: n.id, title: n.name, snippet: n.name),
      for (final o in await listOutputs())
        if (hit(o.title)) SearchHit(kind: 'output', notebookId: o.notebookId, id: o.id, title: o.title),
      for (final r in await listResearch())
        if (hit('${r.name} ${r.description}'))
          SearchHit(kind: 'research', notebookId: r.id, title: r.name, snippet: r.description),
    ];
  }

  @override
  Future<Uint8List> diagnostics() async => Uint8List.fromList(utf8.encode('PK demo diagnostics'));

  @override
  Future<Uint8List> exportOutput(String notebookId, String outputId, String format) async {
    if (format != 'md' && format != 'docx') throw CoreException(400, 'format "$format": want md or docx');
    final o = await getOutput(notebookId, outputId);
    return Uint8List.fromList(utf8.encode(format == 'md' ? o.markdown : 'PK demo docx of ${o.title}'));
  }

  @override
  Future<Map<String, List<String>>> listModels({String? provider}) async {
    const all = {
      'litellm': ['fallback-model', 'primary-model'],
    };
    return provider == null ? all : {provider: ?all[provider]};
  }

  @override
  Future<List<TemplateInfo>> listTemplates({WorkTypeId? workType}) async =>
      List.unmodifiable(_templates.where((t) => workType == null || t.workType == workType));

  @override
  Future<Notebook> createFromTemplate(String templateId, {String? name}) async {
    final t = _templates.firstWhere(
      (t) => t.id == templateId,
      orElse: () => throw StateError('unknown template $templateId'),
    );
    final nb = Notebook(id: 'nb-${_nextId++}', name: name ?? t.name, workType: t.workType, context: const []);
    _notebooks.add(nb);
    return nb;
  }

  @override
  Future<TemplateInfo> saveAsTemplate(String notebookId, {required String name, String description = ''}) async {
    final nb = _notebooks.firstWhere(
      (n) => n.id == notebookId,
      orElse: () => throw StateError('unknown notebook $notebookId'),
    );
    final id = templateIdFor(name);
    if (id.isEmpty) throw const FormatException('Enter a template name');
    final t = TemplateInfo(id: id, name: name, workType: nb.workType, source: 'user', description: description);
    _templates
      ..removeWhere((x) => x.id == id && x.isUser)
      ..add(t);
    return t;
  }

  @override
  Future<void> deleteTemplate(String templateId) async {
    final i = _templates.indexWhere((t) => t.id == templateId);
    if (i < 0 || !_templates[i].isUser) throw StateError('only user templates can be deleted');
    _templates.removeAt(i);
  }

  @override
  Future<List<OutputSummary>> listOutputs({WorkTypeId? workType, String? notebookId}) async => [
    for (final o in demoOutputs.reversed)
      if ((workType == null || o.workType == workType) && (notebookId == null || o.notebookId == notebookId)) o,
  ];
}
