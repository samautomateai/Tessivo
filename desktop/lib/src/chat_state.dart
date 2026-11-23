import 'dart:async';

import 'package:material_ui/material_ui.dart';

import 'core/core_client.dart';
import 'core/models.dart';

/// Chat view state: the session list, the open session, the reply streaming
/// into it, global memory and the last error to show.
class ChatState extends ChangeNotifier {
  ChatState(this.core);

  final CoreClient core;

  List<ChatSummary> sessions = const [];
  ChatSession? open;
  String query = '';
  bool showArchived = false;
  List<String> memory = const [];
  String? error;

  /// Text of the reply streaming now ('' when idle).
  String streaming = '';
  bool sending = false;

  /// Attachments queued for the next message.
  List<ChatAttachment> pending = const [];

  StreamSubscription<ChatEvent>? _reply;
  Completer<void>? _done;
  int _listRequest = 0;
  bool _disposed = false;

  void _changed() {
    if (!_disposed) notifyListeners();
  }

  /// Runs [action], keeping the message of a failure in [error].
  Future<T?> _guard<T>(Future<T> Function() action) async {
    error = null;
    try {
      return await action();
    } on Object catch (e) {
      error = e is FormatException ? e.message : (e is StateError ? e.message : '$e');
      _changed();
      return null;
    }
  }

  Future<void> load() async {
    await _guard(() async {
      memory = await core.chatMemory();
      await refresh();
    });
  }

  /// Reloads the list for the current search and archive filter.
  Future<void> refresh() async {
    final req = ++_listRequest;
    final list = await core.listChats(query: query, archived: showArchived);
    if (req == _listRequest) {
      sessions = list;
      _changed();
    }
  }

  Future<void> search(String q) async {
    query = q.trim();
    await _guard(refresh);
  }

  Future<void> toggleArchived() async {
    showArchived = !showArchived;
    await _guard(refresh);
  }

  Future<void> select(String id) async {
    if (sending) await stop();
    pending = const [];
    await _guard(() async {
      open = await core.getChat(id);
      _changed();
    });
  }

  Future<void> newChat() async {
    if (sending) await stop();
    pending = const [];
    await _guard(() async {
      open = await core.createChat();
      await refresh();
    });
  }

  void attach(ChatAttachment a) {
    if (pending.length >= maxChatAttachments) {
      error = 'At most $maxChatAttachments attachments per message';
    } else if (!pending.any((p) => p.kind == a.kind && p.ref == a.ref)) {
      pending = [...pending, a];
    }
    _changed();
  }

  void detach(ChatAttachment a) {
    pending = [
      for (final p in pending)
        if (p.kind != a.kind || p.ref != a.ref) p,
    ];
    _changed();
  }

  /// Sends [text] in the open chat (creating one if none is open) and streams
  /// the reply. Returns false when nothing was sent.
  Future<bool> send(String text) async {
    if (sending) return false;
    error = null;
    try {
      checkChatMessage(text, pending);
    } on FormatException catch (e) {
      error = e.message;
      _changed();
      return false;
    }
    final session = open ?? await _guard(core.createChat);
    if (session == null) return false;
    final atts = pending;
    final user = ChatMessage(id: '', role: 'user', content: text, attachments: atts);
    open = session.copyWith(messages: [...session.messages, user]);
    pending = const [];
    sending = true;
    streaming = '';
    toolLog = const [];
    approval = null;
    _changed();
    final done = _done = Completer<void>();
    _reply = core
        .sendChat(session.id, text, attachments: atts)
        .listen(
          (e) {
            switch (e) {
              case ChatDelta(text: final piece):
                streaming += piece;
              case ChatToolActivity():
                toolLog = [...toolLog, e];
              case ChatApprovalNeeded():
                approval = e;
              case ChatFinished():
                break;
            }
            _changed();
          },
          onError: (Object e) {
            error = e is CoreException ? e.message : (e is FormatException ? e.message : '$e');
            if (!done.isCompleted) done.complete();
          },
          onDone: () {
            if (!done.isCompleted) done.complete();
          },
          cancelOnError: true,
        );
    await done.future;
    await _finish(session.id);
    return true;
  }

  /// Stops the reply; the core keeps what arrived so far. [send] then reloads
  /// the stored turns.
  Future<void> stop() async {
    final sub = _reply;
    _reply = null;
    await sub?.cancel();
    final done = _done;
    if (done != null && !done.isCompleted) done.complete();
  }

  /// Reloads the stored turns once a reply ends or stops.
  Future<void> _finish(String id) async {
    _reply = null;
    _done = null;
    sending = false;
    streaming = '';
    try {
      final fresh = await core.getChat(id);
      if (open?.id == id) open = fresh;
      await refresh();
    } on Object {
      // Keep what is on screen; the next action reports real errors.
    }
    _changed();
  }

  Future<void> rename(String id, String title) => _update(id, title: title);
  Future<void> setPinned(String id, bool pinned) => _update(id, pinned: pinned);
  Future<void> setArchived(String id, bool archived) => _update(id, archived: archived);
  Future<void> setModel(String model) => _update(open!.id, model: model);
  Future<void> setThinking(String effort) => _update(open!.id, thinkingEffort: effort);

  /// Sets the MCP servers this chat may use.
  Future<void> setMcpServers(List<String> servers) => _guard(() async {
    final s = await core.updateChat(open!.id, mcpServers: servers);
    if (open?.id == s.id) open = s;
  });

  /// Tool activity of the reply in progress.
  List<ChatToolActivity> toolLog = const [];

  /// A tool call the reply waits on, or null.
  ChatApprovalNeeded? approval;

  /// Answers [approval]; [remember] keeps the answer for this chat.
  Future<void> answerTool({required bool allow, bool remember = false}) async {
    final a = approval;
    final chatId = open?.id;
    if (a == null || chatId == null) return;
    approval = null;
    _changed();
    await _guard(() => core.answerChatTool(chatId, a.id, allow: allow, remember: remember));
  }

  Future<void> _update(
    String id, {
    String? title,
    bool? pinned,
    bool? archived,
    String? model,
    String? thinkingEffort,
  }) => _guard(() async {
    final s = await core.updateChat(
      id,
      title: title,
      pinned: pinned,
      archived: archived,
      model: model,
      thinkingEffort: thinkingEffort,
    );
    if (open?.id == id) open = s;
    await refresh();
  });

  Future<void> delete(String id) => _guard(() async {
    if (sending && open?.id == id) await stop();
    await core.deleteChat(id);
    if (open?.id == id) open = null;
    await refresh();
  });

  Future<void> setMemory(List<String> facts) => _guard(() async {
    memory = await core.setChatMemory(facts);
    _changed();
  });

  Future<RepeatablePlan?> plan() => _guard(() => core.planRepeatable(open!.id));

  Future<Notebook?> makeRepeatable({String? name, WorkTypeId? workType, String? templateId}) =>
      _guard(() => core.makeRepeatable(open!.id, name: name, workType: workType, templateId: templateId));

  Future<ChatOutput?> saveReply(String messageId, WorkTypeId workType) =>
      _guard(() => core.saveReply(open!.id, messageId, workType));

  void clearError() {
    error = null;
    _changed();
  }

  @override
  void dispose() {
    _disposed = true;
    unawaited(_reply?.cancel());
    super.dispose();
  }
}
