import 'dart:async';

import 'package:material_ui/material_ui.dart';

import 'core/core_client.dart';
import 'core/models.dart';

/// One output's thread: the output, its conversation, the reply streaming in
/// and the suggestions it produced. Failures go to [onError] as one message.
class ThreadState extends ChangeNotifier {
  ThreadState(this.core, this.notebookId, this.outputId, {this.onError, this.onApplied});

  final CoreClient core;
  final String notebookId;
  final String outputId;

  /// Reports a failure (the app shows it as a notice).
  final void Function(String message)? onError;

  /// Called after a suggestion changed the notebook (memory or prompt).
  final Future<void> Function()? onApplied;

  ChatSession? session;
  OutputDetail? detail;
  List<Suggestion> suggestions = const [];

  /// True while a reply streams; [streamedText] is what arrived so far.
  bool streaming = false;
  String streamedText = '';

  /// True while suggestions are generated or a version is made.
  bool busy = false;

  StreamSubscription<ThreadEvent>? _reply;
  Completer<void>? _done;
  bool _disposed = false;

  List<Suggestion> get openSuggestions => [
    for (final s in suggestions)
      if (s.isOpen) s,
  ];

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
        StateError(:final message) => message,
        _ => '$e',
      });
      return null;
    }
  }

  Future<void> load() async {
    await _guard(() async {
      final (s, d, list) = await (
        core.getThread(notebookId, outputId),
        core.getOutput(notebookId, outputId),
        core.listSuggestions(notebookId, outputId),
      ).wait;
      session = s;
      detail = d;
      suggestions = list;
    });
    _changed();
  }

  /// Sends [text] and streams the reply. Returns false when nothing was sent.
  Future<bool> send(String text) async {
    final current = session;
    if (streaming || current == null || text.trim().isEmpty) return false;
    streaming = true;
    streamedText = '';
    session = current.copyWith(
      messages: [
        ...current.messages,
        ChatMessage(id: '', role: 'user', content: text),
      ],
    );
    _changed();
    final done = _done = Completer<void>();
    _reply = core
        .sendThread(notebookId, outputId, text)
        .listen(
          (e) {
            switch (e) {
              case ThreadDelta(text: final piece):
                streamedText += piece;
              case ThreadSuggestion(:final suggestion):
                suggestions = [...suggestions, suggestion];
              case ThreadFinished():
                break;
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
    await load(); // the stored turns, including a stopped reply
    return true;
  }

  /// Stops the reply; the core keeps what arrived so far.
  Future<void> stop() async {
    final sub = _reply;
    _reply = null;
    await sub?.cancel();
    final done = _done;
    if (done != null && !done.isCompleted) done.complete();
  }

  Future<T?> _busy<T>(Future<T> Function() action) async {
    busy = true;
    _changed();
    try {
      return await _guard(action);
    } finally {
      busy = false;
      _changed();
    }
  }

  /// Asks the model for improvements from the conversation so far.
  Future<void> suggest() => _busy(() async {
    await core.generateSuggestions(notebookId, outputId);
    suggestions = await core.listSuggestions(notebookId, outputId);
  });

  Future<void> apply(String id) => _busy(() async {
    await core.applySuggestion(notebookId, outputId, id);
    suggestions = await core.listSuggestions(notebookId, outputId);
    await onApplied?.call();
  });

  Future<void> dismiss(String id) => _busy(() async {
    await core.dismissSuggestion(notebookId, outputId, id);
    suggestions = await core.listSuggestions(notebookId, outputId);
  });

  /// Saves a revised version of the output from this thread.
  Future<OutputSummary?> newVersion() => _busy(() async {
    final v = await core.newVersion(notebookId, outputId);
    session = await core.getThread(notebookId, outputId);
    return v;
  });

  @override
  void dispose() {
    _disposed = true;
    unawaited(_reply?.cancel());
    super.dispose();
  }
}
