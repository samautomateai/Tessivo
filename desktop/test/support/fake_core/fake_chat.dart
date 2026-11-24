import 'dart:async';

import 'package:tessivo_desktop/src/core/chat_models.dart';
import 'package:tessivo_desktop/src/core/checks.dart';
import 'package:tessivo_desktop/src/core/errors.dart';
import 'package:tessivo_desktop/src/core/models.dart';

/// In-memory chat for the demo core: sessions, streamed sample replies, memory,
/// "make repeatable" and saving replies. Mixed into `FakeCoreClient`.
mixin DemoChat {
  final Map<String, Completer<bool>> _toolAnswers = {};

  Future<void> answerChatTool(String chatId, String callId, {required bool allow, bool remember = false}) async {
    final c = _toolAnswers.remove(callId);
    if (c == null) throw const CoreException(404, 'no tool call is waiting for that answer');
    c.complete(allow);
  }

  /// Delay between streamed words.
  Duration get chatTick;

  /// Provided by the host client.
  Future<Notebook> createNotebook({required String name, required WorkTypeId workType});

  final Map<String, ChatSession> _chats = {
    'c-1': const ChatSession(
      id: 'c-1',
      title: 'Rotating signing keys',
      pinned: true,
      messages: [
        ChatMessage(id: 'm1', role: 'user', content: 'How often should we rotate the payment signing keys?'),
        ChatMessage(
          id: 'm2',
          role: 'assistant',
          model: 'litellm/primary-model',
          content:
              '## Short answer\n\nRotate **every 90 days**, and immediately after any suspected exposure.\n\n'
              '| Key | Interval |\n| --- | --- |\n| Signing | 90 days |\n| API | 30 days |\n\n'
              '- Keep the old key valid for one overlap window\n- Record each rotation in the audit log',
        ),
      ],
    ),
    'c-2': const ChatSession(id: 'c-2', title: 'Q3 vendor spend', messages: []),
  };
  final Set<String> _archived = {};
  List<String> _chatMemory = const ['I lead the payments platform team'];
  int _chatSeq = 10;

  ChatSession _chat(String id) => _chats[id] ?? (throw StateError('unknown chat $id'));

  ChatSummary _summary(ChatSession s) => ChatSummary(
    id: s.id,
    title: s.title,
    pinned: s.pinned,
    archived: _archived.contains(s.id),
    messageCount: s.messages.length,
    updatedAt: DateTime.now().toUtc().toIso8601String(),
    preview: s.messages.isEmpty ? '' : s.messages.last.content.split('\n').first,
  );

  Future<List<ChatSummary>> listChats({String query = '', bool archived = false}) async {
    final q = query.toLowerCase();
    final out = [
      for (final s in _chats.values)
        if (_archived.contains(s.id) == archived &&
            (q.isEmpty ||
                s.title.toLowerCase().contains(q) ||
                s.messages.any((m) => m.content.toLowerCase().contains(q))))
          _summary(s),
    ]..sort((a, b) => a.pinned == b.pinned ? 0 : (a.pinned ? -1 : 1));
    return out;
  }

  Future<ChatSession> createChat({String title = '', String model = '', String thinkingEffort = ''}) async {
    final s = ChatSession(id: 'c-${_chatSeq++}', title: title, model: model, thinkingEffort: thinkingEffort);
    _chats[s.id] = s;
    return s;
  }

  Future<ChatSession> getChat(String chatId) async => _chat(chatId);

  Future<ChatSession> updateChat(
    String chatId, {
    String? title,
    bool? pinned,
    bool? archived,
    String? model,
    String? thinkingEffort,
    List<String>? mcpServers,
  }) async {
    final s = _chat(chatId);
    if (title != null && (title.contains('\n') || title.length > maxChatTitle)) {
      throw const FormatException('Titles are one line of at most $maxChatTitle characters');
    }
    if (archived == true) _archived.add(chatId);
    if (archived == false) _archived.remove(chatId);
    final updated = ChatSession(
      id: s.id,
      title: title?.trim() ?? s.title,
      pinned: pinned ?? s.pinned,
      archived: archived ?? s.archived,
      model: model ?? s.model,
      thinkingEffort: thinkingEffort ?? s.thinkingEffort,
      messages: s.messages,
      mcpServers: mcpServers ?? s.mcpServers,
    );
    _chats[chatId] = updated;
    return updated;
  }

  Future<void> deleteChat(String chatId) async {
    if (_chats.remove(chatId) == null) throw StateError('unknown chat $chatId');
    _archived.remove(chatId);
  }

  Stream<ChatEvent> sendChat(String chatId, String text, {List<ChatAttachment> attachments = const []}) async* {
    checkChatMessage(text, attachments);
    final s = _chat(chatId);
    if (s.mcpServers.isNotEmpty) yield* _demoTool(s.mcpServers.first);
    final user = ChatMessage(id: 'm${_chatSeq++}', role: 'user', content: text, attachments: attachments);
    final title = s.title.isEmpty ? _autoTitle(text) : s.title;
    _chats[chatId] = s.copyWith(title: title, messages: [...s.messages, user]);
    final reply = StringBuffer();
    final words = 'Here is a sample reply from the demo core. Connect a model to get real answers.'.split(' ');
    var completed = false;
    late ChatMessage assistant;
    try {
      for (final w in words) {
        if (chatTick > Duration.zero) await Future<void>.delayed(chatTick);
        final piece = reply.isEmpty ? w : ' $w';
        reply.write(piece);
        yield ChatDelta(piece);
      }
      completed = true;
    } finally {
      // Runs on completion and when the listener cancels (the stop button).
      assistant = ChatMessage(
        id: 'm${_chatSeq++}',
        role: 'assistant',
        content: reply.toString(),
        model: 'demo',
        stopped: !completed,
      );
      final current = _chat(chatId);
      _chats[chatId] = current.copyWith(messages: [...current.messages, assistant]);
    }
    yield ChatFinished(user, assistant);
  }

  static String _autoTitle(String text) {
    final line = text.split(RegExp(r'\s+')).where((w) => w.isNotEmpty).join(' ');
    return line.length <= 60 ? line : '${line.substring(0, 59).trim()}…';
  }

  Future<List<String>> chatMemory() async => List.unmodifiable(_chatMemory);

  Future<List<String>> setChatMemory(List<String> memory) async {
    checkMemory(memory);
    _chatMemory = List.unmodifiable(memory.map((m) => m.trim()));
    return _chatMemory;
  }

  Future<RepeatablePlan> planRepeatable(String chatId) async {
    final s = _chat(chatId);
    final last = s.messages.lastWhere(
      (m) => m.isUser,
      orElse: () => throw const FormatException('This chat has no messages to repeat yet'),
    );
    return RepeatablePlan(
      name: s.title.isEmpty ? 'From chat' : s.title,
      workType: WorkTypeId.summarize,
      prompt: last.content,
      context: [
        for (final m in s.messages)
          for (final a in m.attachments)
            if (a.kind == 'path' || a.kind == 'url') a.ref,
      ],
    );
  }

  Future<Notebook> makeRepeatable(String chatId, {String? name, WorkTypeId? workType, String? templateId}) async {
    final plan = await planRepeatable(chatId);
    return createNotebook(name: name ?? plan.name, workType: workType ?? plan.workType);
  }

  Future<ChatOutput> saveReply(String chatId, String messageId, WorkTypeId workType) async {
    final m = _chat(
      chatId,
    ).messages.firstWhere((m) => m.id == messageId, orElse: () => throw StateError('unknown message $messageId'));
    if (m.isUser || m.content.trim().isEmpty) {
      throw const FormatException('Only non-empty replies can be saved as outputs');
    }
    final title = _autoTitle(m.content.split('\n').first.replaceAll('#', ''));
    return ChatOutput(notebookId: 'chat-${workType.name}', outputId: 'o-${_chatSeq++}', title: title);
  }
}

/// The demo's one tool call: it waits for the user's answer, then reports.
extension on DemoChat {
  Stream<ChatEvent> _demoTool(String server) async* {
    final tool = 'mcp__${server}__list_issues';
    final answer = _toolAnswers['demo-call'] = Completer<bool>();
    yield ChatToolActivity(kind: 'call', tool: tool, args: '{"state":"open"}');
    yield ChatApprovalNeeded(id: 'demo-call', tool: tool, args: '{"state":"open"}');
    final allowed = await answer.future;
    yield allowed
        ? ChatToolActivity(kind: 'result', tool: tool, summary: '3 open issues', by: 'you')
        : ChatToolActivity(kind: 'denied', tool: tool);
  }
}
