import 'dart:async';

import 'package:material_ui/material_ui.dart';

import 'core/core_client.dart';

/// One research notebook open in the app: its sources, the context a
/// question would send, its notes and the open conversation with the reply
/// streaming in. Failures go to [onError] as one message.
class ResearchState extends ChangeNotifier {
  ResearchState(this.core, this.notebookId, {this.onError, this.onChanged});

  final CoreClient core;
  final String notebookId;

  /// Reports a failure (the app shows it as a notice).
  final void Function(String message)? onError;

  /// Called with the notebook after its sources changed (the sidebar list).
  final void Function(ResearchNotebook notebook)? onChanged;

  ResearchNotebook? notebook;
  ContextTotals context = const ContextTotals();
  List<ResearchNote> notes = const [];
  List<ChatSummary> chats = const [];

  /// The open conversation and its answers' citations, or null before the first question.
  ResearchChat? chat;

  /// True while an answer streams; [streamedText] is what arrived so far.
  bool streaming = false;
  String streamedText = '';

  /// Sources the last answer could not read.
  List<String> skipped = const [];

  StreamSubscription<ResearchEvent>? _reply;
  Completer<void>? _done;
  bool _disposed = false;

  void _changed() {
    if (!_disposed) notifyListeners();
  }

  /// Runs [action]; a failure is reported through [onError] and yields null.
  Future<T?> _guard<T>(Future<T> Function() action) async {
    try {
      return await action();
    } on Object catch (e) {
      onError?.call(switch (e) {
        CoreException(:final message) => message,
        FormatException(:final message) => message,
        _ => '$e',
      });
      return null;
    } finally {
      _changed();
    }
  }

  /// Loads the notebook, its context, notes and conversations; opens the newest conversation.
  Future<void> load() => _guard(() async {
    final (nb, ctx, list, sessions) = await (
      core.getResearch(notebookId),
      core.researchContext(notebookId),
      core.researchNotes(notebookId),
      core.listResearchChats(notebookId),
    ).wait;
    notebook = nb;
    context = ctx;
    notes = list;
    chats = sessions;
    if (chat == null && sessions.isNotEmpty) chat = await core.getResearchChat(notebookId, sessions.first.id);
  });

  /// Reloads the notebook and its context (for example after a summary event).
  Future<void> reload() => _guard(() async {
    _put(await core.getResearch(notebookId));
    context = await core.researchContext(notebookId);
  });

  void _put(ResearchNotebook nb) {
    notebook = nb;
    onChanged?.call(nb);
  }

  Future<void> _sources(Future<ResearchNotebook> Function() action) => _guard(() async {
    _put(await action());
    context = await core.researchContext(notebookId);
  });

  Future<void> setLevel(String sourceId, String level) =>
      _sources(() => core.setSourceLevel(notebookId, sourceId, level));

  Future<void> addSource(String path) => _sources(() => core.addResearchSource(notebookId, path: path));

  Future<void> addMcpSource(String server, String uri) =>
      _sources(() => core.addResearchSource(notebookId, server: server, uri: uri));

  Future<void> removeSource(String sourceId) => _sources(() => core.removeResearchSource(notebookId, sourceId));

  Future<void> refreshSummary(String sourceId) => _guard(() async {
    await core.refreshSummary(notebookId, sourceId);
    _put(await core.getResearch(notebookId));
  });

  /// A source's text and summary, or null when it could not be read.
  Future<ResearchSourceText?> sourceText(String sourceId) => _guard(() => core.researchSource(notebookId, sourceId));

  Future<void> _notes(Future<void> Function() action) => _guard(() async {
    await action();
    notes = await core.researchNotes(notebookId);
    context = await core.researchContext(notebookId);
  });

  Future<void> saveNote(ResearchNote note) => _notes(() => core.saveResearchNote(notebookId, note));

  Future<void> setNoteIncluded(ResearchNote n, bool include) =>
      saveNote(ResearchNote(id: n.id, title: n.title, text: n.text, origin: n.origin, include: include));

  Future<void> deleteNote(String noteId) => _notes(() => core.deleteResearchNote(notebookId, noteId));

  /// Saves an answer as an AI note.
  Future<void> noteFromMessage(String messageId) {
    final c = chat;
    if (c == null) return Future.value();
    return _notes(() => core.noteFromMessage(notebookId, c.session.id, messageId));
  }

  /// Opens a stored conversation.
  Future<void> openChat(String chatId) => _guard(() async {
    chat = await core.getResearchChat(notebookId, chatId);
    skipped = const [];
  });

  /// Starts an empty conversation.
  Future<void> newChat() => _guard(() async {
    final s = await core.createResearchChat(notebookId);
    chat = ResearchChat(session: s);
    chats = await core.listResearchChats(notebookId);
    skipped = const [];
  });

  /// Asks [text] and streams the answer. Returns false when nothing was sent.
  Future<bool> send(String text) async {
    if (streaming || text.trim().isEmpty) return false;
    if (chat == null) await newChat();
    final current = chat;
    if (current == null) return false;
    streaming = true;
    streamedText = '';
    skipped = const [];
    chat = ResearchChat(
      session: current.session.copyWith(
        messages: [
          ...current.session.messages,
          ChatMessage(id: '', role: 'user', content: text),
        ],
      ),
      citations: current.citations,
    );
    _changed();
    final done = _done = Completer<void>();
    _reply = core
        .sendResearch(notebookId, current.session.id, text)
        .listen(
          (e) {
            switch (e) {
              case ResearchDelta(text: final piece):
                streamedText += piece;
              case ResearchFinished(skipped: final s):
                skipped = s;
            }
            _changed();
          },
          onError: (Object e) {
            onError?.call(e is CoreException ? e.message : '$e');
            if (!done.isCompleted) done.complete();
          },
          onDone: () {
            if (!done.isCompleted) done.complete();
          },
          cancelOnError: true,
        );
    await done.future;
    _reply = null;
    streaming = false;
    streamedText = '';
    await openChatKeepingSkipped(current.session.id);
    return true;
  }

  /// Reloads the conversation after an answer without clearing [skipped].
  Future<void> openChatKeepingSkipped(String chatId) => _guard(() async {
    chat = await core.getResearchChat(notebookId, chatId);
    chats = await core.listResearchChats(notebookId);
  });

  /// Stops the answer; the core keeps what arrived so far.
  Future<void> stop() async {
    final sub = _reply;
    _reply = null;
    await sub?.cancel();
    final done = _done;
    if (done != null && !done.isCompleted) done.complete();
  }

  @override
  void dispose() {
    _disposed = true;
    unawaited(_reply?.cancel());
    super.dispose();
  }
}

/// The part of a folder source's text (files joined as `== <path>` sections)
/// that holds [path]; the whole text when [path] is null or not found.
String fileSection(String text, String? path) {
  if (path == null) return text;
  final head = '== $path\n';
  final start = text.startsWith(head) ? 0 : text.indexOf('\n$head');
  if (start < 0) return text;
  final from = start == 0 ? head.length : start + 1 + head.length;
  final end = text.indexOf('\n\n== ', from);
  return (end < 0 ? text.substring(from) : text.substring(from, end)).trimRight();
}
