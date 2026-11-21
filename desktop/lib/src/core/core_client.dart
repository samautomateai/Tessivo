import 'dart:async';

import 'chat_models.dart';
import 'checks.dart';
import 'demo_chat.dart';
import 'demo_templates.dart';
import 'models.dart';

export 'chat_models.dart';
export 'checks.dart';
export 'errors.dart';

/// The desktop app's view of the Go core. `HttpCoreClient` talks to the local
/// API (docs/api.md); [DemoCoreClient] serves sample data when no core runs.
abstract interface class CoreClient {
  Future<List<WorkType>> listWorkTypes();
  Future<List<Notebook>> listNotebooks();
  Future<Notebook> createNotebook({required String name, required WorkTypeId workType});
  Future<Notebook> setContextEnabled({required String notebookId, required String contextId, required bool enabled});
  Stream<RunEvent> startRun({required String notebookId, required String prompt, required Map<String, String> vars});
  Future<List<OutputSummary>> listOutputs({WorkTypeId? workType});
  Future<Notebook> getNotebook(String notebookId);
  Future<Notebook> addContext(String notebookId, String path);
  Future<Notebook> setMemory(String notebookId, List<String> memory);
  Future<OutputDetail> getOutput(String notebookId, String outputId);
  Future<SettingsInfo> getSettings();
  Future<Map<String, List<String>>> listModels({String? provider});
  Future<List<ScheduleInfo>> listSchedules(String notebookId);
  Future<ScheduleInfo> addSchedule(String notebookId, {required String cron, String prompt, String missed});
  Future<ScheduleInfo> setScheduleEnabled(String notebookId, String scheduleId, {required bool enabled});
  Future<void> deleteSchedule(String notebookId, String scheduleId);

  Future<List<TemplateInfo>> listTemplates({WorkTypeId? workType});
  Future<Notebook> createFromTemplate(String templateId, {String? name});
  Future<TemplateInfo> saveAsTemplate(String notebookId, {required String name, String description});
  Future<void> deleteTemplate(String templateId);

  Future<List<ChatSummary>> listChats({String query, bool archived});
  Future<ChatSession> createChat({String title, String model, String thinkingEffort});
  Future<ChatSession> getChat(String chatId);
  Future<ChatSession> updateChat(
    String chatId, {
    String? title,
    bool? pinned,
    bool? archived,
    String? model,
    String? thinkingEffort,
  });
  Future<void> deleteChat(String chatId);

  /// Streams a reply. Cancelling the subscription stops it; the core keeps the partial reply.
  Stream<ChatEvent> sendChat(String chatId, String text, {List<ChatAttachment> attachments});
  Future<List<String>> chatMemory();
  Future<List<String>> setChatMemory(List<String> memory);
  Future<RepeatablePlan> planRepeatable(String chatId);
  Future<Notebook> makeRepeatable(String chatId, {String? name, WorkTypeId? workType, String? templateId});
  Future<ChatOutput> saveReply(String chatId, String messageId, WorkTypeId workType);

  /// Live notifications (scheduled runs). Never completes while the core runs.
  Stream<CoreEvent> events();
}

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
class DemoCoreClient with DemoChat implements CoreClient {
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
  Future<SettingsInfo> getSettings() async => const SettingsInfo(
    providers: [ProviderInfo(name: 'litellm', baseUrl: 'http://localhost:4000/v1', keyRef: 'TESSIVO_LITELLM_KEY')],
    aliases: [
      AliasInfo(
        name: 'smart',
        chain: ['litellm/primary-model', 'litellm/fallback-model'],
        fallbackOn: ['timeout', 'rate_limit', 'server_error'],
      ),
    ],
    defaultModel: 'smart',
    thinkingEffort: 'medium',
    maxOutputTokens: 8000,
    maxTokensPerDay: 2000000,
  );

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
  Future<List<OutputSummary>> listOutputs({WorkTypeId? workType}) async {
    const all = [
      OutputSummary(
        id: 'o1',
        notebookId: 'nb-1',
        title: 'PR #482 risk report',
        workType: WorkTypeId.analyze,
        notebookName: 'Change risk review',
        trigger: 'Manual',
        createdAt: 'Today 22:01',
        versions: 2,
      ),
      OutputSummary(
        id: 'o2',
        notebookId: 'nb-4',
        title: 'Weekly ops report, week 47',
        workType: WorkTypeId.report,
        notebookName: 'Weekly ops report',
        trigger: 'Scheduled',
        createdAt: 'Mon 08:00',
        versions: 1,
      ),
      OutputSummary(
        id: 'o3',
        notebookId: 'nb-2',
        title: 'Vendor contracts digest',
        workType: WorkTypeId.summarize,
        notebookName: 'Vendor contracts digest',
        trigger: 'Manual',
        createdAt: 'Nov 18',
        versions: 3,
      ),
    ];
    return workType == null ? all : all.where((o) => o.workType == workType).toList();
  }
}
