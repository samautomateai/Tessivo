import 'dart:async';

import 'chat_models.dart';
import 'checks.dart';
import 'errors.dart';
import 'models.dart';
import 'thread_models.dart';

/// In-memory output threads for the demo core: sample outputs (oldest first),
/// one seeded thread and suggestion, streamed replies and new versions. Mixed
/// into `DemoCoreClient`.
mixin DemoThreads {
  /// Delay between streamed words.
  Duration get chatTick;

  final List<OutputSummary> demoOutputs = [
    const OutputSummary(
      id: 'o3',
      notebookId: 'nb-2',
      title: 'Vendor contracts digest',
      workType: WorkTypeId.summarize,
      notebookName: 'Vendor contracts digest',
      trigger: 'Manual',
      createdAt: 'Nov 18',
    ),
    const OutputSummary(
      id: 'o2',
      notebookId: 'nb-4',
      title: 'Weekly ops report, week 47',
      workType: WorkTypeId.report,
      notebookName: 'Weekly ops report',
      trigger: 'Scheduled',
      createdAt: 'Mon 08:00',
    ),
    const OutputSummary(
      id: 'o1',
      notebookId: 'nb-1',
      title: 'PR #482 risk report',
      workType: WorkTypeId.analyze,
      notebookName: 'Change risk review',
      trigger: 'Manual',
      createdAt: 'Today 22:01',
    ),
  ];

  final Map<String, ChatSession> _threads = {
    'o1': const ChatSession(
      id: 'o1',
      title: '',
      messages: [
        ChatMessage(id: 't1', role: 'user', content: 'Why is the auth risk rated high?'),
        ChatMessage(
          id: 't2',
          role: 'assistant',
          model: 'litellm/primary-model',
          content: 'The diff removes the token expiry check in `auth/session.go` (line 88) with no replacement test.',
        ),
      ],
    ),
  };

  final Map<String, List<Suggestion>> _suggestions = {
    'o1': const [
      Suggestion(
        id: 'sg1',
        kind: 'prompt',
        text: 'Assess the risk and impact described by the context. Cite the file and line behind each risk.',
        reason: 'You asked where a claim came from.',
        status: 'open',
        target: PromptTarget(
          kind: 'saved',
          promptName: 'review',
          schedules: ['s1'],
          current: 'Assess the risk and impact described by the context.',
        ),
      ),
    ],
  };
  int _threadSeq = 0;

  OutputSummary _output(String notebookId, String outputId) => demoOutputs.firstWhere(
    (o) => o.id == outputId && o.notebookId == notebookId,
    orElse: () => throw CoreException(404, 'output $outputId not found'),
  );

  Future<ChatSession> getThread(String notebookId, String outputId) async {
    _output(notebookId, outputId);
    return _threads[outputId] ?? ChatSession(id: outputId, title: '');
  }

  Future<ChatSession> setThreadSettings(
    String notebookId,
    String outputId, {
    String? model,
    String? thinkingEffort,
  }) async {
    final t = await getThread(notebookId, outputId);
    return _threads[outputId] = ChatSession(
      id: t.id,
      title: t.title,
      model: model ?? t.model,
      thinkingEffort: thinkingEffort ?? t.thinkingEffort,
      messages: t.messages,
    );
  }

  Stream<ThreadEvent> sendThread(
    String notebookId,
    String outputId,
    String text, {
    List<ChatAttachment> attachments = const [],
  }) async* {
    checkChatMessage(text, attachments);
    final t = await getThread(notebookId, outputId);
    final user = ChatMessage(id: 'tu${++_threadSeq}', role: 'user', content: text, attachments: attachments);
    final words = 'In this sample, the answer points back to the sources behind the output.'.split(' ');
    for (final w in words) {
      await Future<void>.delayed(chatTick);
      yield ThreadDelta('$w ');
    }
    final reply = ChatMessage(id: 'ta$_threadSeq', role: 'assistant', content: words.join(' '));
    _threads[outputId] = t.copyWith(messages: [...t.messages, user, reply]);
    yield ThreadFinished(user, reply);
  }

  Future<List<Suggestion>> listSuggestions(String notebookId, String outputId) async {
    _output(notebookId, outputId);
    return List.unmodifiable(_suggestions[outputId] ?? const <Suggestion>[]);
  }

  Future<List<Suggestion>> generateSuggestions(String notebookId, String outputId) async {
    _output(notebookId, outputId);
    final s = Suggestion(
      id: 'sg${++_threadSeq}',
      kind: 'memory',
      text: 'Cite the file and line behind each claim.',
      status: 'open',
    );
    _suggestions[outputId] = [...?_suggestions[outputId], s];
    return [s];
  }

  Future<({Suggestion suggestion, String? promptName})> applySuggestion(
    String notebookId,
    String outputId,
    String suggestionId,
  ) async => (suggestion: _decide(notebookId, outputId, suggestionId, 'applied'), promptName: null);

  Future<Suggestion> dismissSuggestion(String notebookId, String outputId, String suggestionId) async =>
      _decide(notebookId, outputId, suggestionId, 'dismissed');

  Suggestion _decide(String notebookId, String outputId, String id, String status) {
    _output(notebookId, outputId);
    final list = [...?_suggestions[outputId]];
    final i = list.indexWhere((s) => s.id == id);
    if (i < 0) throw CoreException(404, 'suggestion $id not found');
    final s = list[i];
    if (!s.isOpen) throw CoreException(409, 'suggestion $id is already ${s.status}');
    list[i] = Suggestion(id: s.id, kind: s.kind, text: s.text, status: status, reason: s.reason, target: s.target);
    _suggestions[outputId] = list;
    return list[i];
  }

  Future<OutputSummary> newVersion(String notebookId, String outputId, {String? fromMessageId}) async {
    final parent = _output(notebookId, outputId);
    final v = OutputSummary(
      id: '${parent.id}-v${parent.version + 1}',
      notebookId: parent.notebookId,
      title: '${parent.title} (revised)',
      workType: parent.workType,
      notebookName: parent.notebookName,
      trigger: 'Revision',
      createdAt: 'Just now',
      version: parent.version + 1,
      parentId: parent.id,
    );
    demoOutputs.add(v);
    final t = await getThread(notebookId, outputId);
    final note = ChatMessage(id: 'tn${++_threadSeq}', role: 'note', content: 'Saved as v${v.version} (${v.id})');
    _threads[outputId] = t.copyWith(messages: [...t.messages, note]);
    return v;
  }
}
