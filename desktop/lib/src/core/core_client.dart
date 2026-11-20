import 'dart:async';

import 'models.dart';

/// The desktop app's view of the Go core. `HttpCoreClient` talks to the local
/// API (docs/api.md); [DemoCoreClient] serves sample data when no core runs.
abstract interface class CoreClient {
  Future<List<WorkType>> listWorkTypes();
  Future<List<Notebook>> listNotebooks();
  Future<Notebook> createNotebook({required String name, required WorkTypeId workType});
  Future<Notebook> setContextEnabled({required String notebookId, required String contextId, required bool enabled});
  Stream<RunEvent> startRun({required String notebookId, required String prompt, required Map<String, String> vars});
  Future<List<OutputSummary>> listOutputs({WorkTypeId? workType});
  Future<List<ScheduleInfo>> listSchedules(String notebookId);
  Future<ScheduleInfo> addSchedule(String notebookId, {required String cron, String prompt, String missed});
  Future<ScheduleInfo> setScheduleEnabled(String notebookId, String scheduleId, {required bool enabled});
  Future<void> deleteSchedule(String notebookId, String scheduleId);

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

/// In-memory core with sample data, used until the gRPC client lands.
class DemoCoreClient implements CoreClient {
  DemoCoreClient({Duration tick = const Duration(milliseconds: 40)}) : _tick = tick;

  final Duration _tick;
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

  @override
  Future<List<OutputSummary>> listOutputs({WorkTypeId? workType}) async {
    const all = [
      OutputSummary(
        id: 'o1',
        title: 'PR #482 risk report',
        workType: WorkTypeId.analyze,
        notebookName: 'Change risk review',
        trigger: 'Manual',
        createdAt: 'Today 22:01',
        versions: 2,
      ),
      OutputSummary(
        id: 'o2',
        title: 'Weekly ops report, week 47',
        workType: WorkTypeId.report,
        notebookName: 'Weekly ops report',
        trigger: 'Scheduled',
        createdAt: 'Mon 08:00',
        versions: 1,
      ),
      OutputSummary(
        id: 'o3',
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
