import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/core_client.dart';
import '../theme.dart';
import '../thread_state.dart';
import 'chat_widgets.dart';
import 'export_menu.dart';
import 'markdown_view.dart';

/// The open output, its conversation, the suggestions it produced and the
/// box to ask about it.
class OutputThread extends StatelessWidget {
  const OutputThread({super.key, required this.state, required this.thread, required this.version});

  final AppState state;
  final ThreadState thread;
  final int version;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      key: const Key('output-thread'),
      listenable: thread,
      builder: (context, _) {
        final session = thread.session;
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            _Header(state: state, thread: thread, version: version),
            const Divider(height: 1),
            Expanded(
              child: ListView(
                padding: const EdgeInsets.all(20),
                children: [
                  if (thread.detail case final d?)
                    Card(
                      child: Padding(padding: const EdgeInsets.all(16), child: MarkdownView(d.markdown)),
                    ),
                  const SizedBox(height: 16),
                  Text('Ask about this output', style: Theme.of(context).textTheme.titleSmall),
                  if (session != null && session.messages.isEmpty && !thread.streaming)
                    const Padding(
                      padding: EdgeInsets.only(top: 8),
                      child: Text(
                        'Ask why it says something, where a claim comes from, or what to change.',
                        style: TextStyle(color: Tokens.textMuted),
                      ),
                    ),
                  for (final m in session?.messages ?? const <ChatMessage>[])
                    if (m.role == 'note') _Note(m.content) else MessageBubble(message: m),
                  if (thread.streaming)
                    MessageBubble(
                      message: ChatMessage(
                        id: '',
                        role: 'assistant',
                        content: thread.streamedText.isEmpty ? '…' : thread.streamedText,
                      ),
                    ),
                  for (final s in thread.openSuggestions) SuggestionCard(thread: thread, suggestion: s),
                ],
              ),
            ),
            ThreadComposer(thread: thread),
          ],
        );
      },
    );
  }
}

class _Header extends StatelessWidget {
  const _Header({required this.state, required this.thread, required this.version});

  final AppState state;
  final ThreadState thread;
  final int version;

  @override
  Widget build(BuildContext context) {
    final idle = !thread.busy && !thread.streaming;
    final hasReply = thread.session?.messages.any((m) => m.role == 'assistant') ?? false;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 8),
      child: Wrap(
        spacing: 8,
        runSpacing: 6,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          Chip(label: Text('v$version')),
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 360),
            child: Text(
              thread.detail?.title ?? '',
              style: Theme.of(context).textTheme.titleMedium,
              overflow: TextOverflow.ellipsis,
            ),
          ),
          ExportMenu(
            core: state.core,
            notebookId: thread.notebookId,
            outputId: thread.outputId,
            title: thread.detail?.title ?? 'output',
          ),
          if (thread.busy) const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2)),
          TextButton.icon(
            key: const Key('suggest-improvements'),
            onPressed: idle && hasReply ? () => unawaited(thread.suggest()) : null,
            icon: const Icon(Icons.lightbulb_outline, size: 18),
            label: const Text('Suggest improvements'),
          ),
          FilledButton.icon(
            key: const Key('new-version'),
            onPressed: idle && hasReply ? () => unawaited(state.newVersion()) : null,
            style: FilledButton.styleFrom(backgroundColor: Tokens.accent),
            icon: const Icon(Icons.auto_fix_high, size: 18),
            label: const Text('New version'),
          ),
        ],
      ),
    );
  }
}

class _Note extends StatelessWidget {
  const _Note(this.text);

  final String text;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: 6),
    child: Text(
      text,
      textAlign: TextAlign.center,
      style: const TextStyle(color: Tokens.textMuted, fontSize: 12),
    ),
  );
}

/// An open suggestion with Apply and Dismiss; a prompt change shows what it
/// reaches and the text before and after.
class SuggestionCard extends StatelessWidget {
  const SuggestionCard({super.key, required this.thread, required this.suggestion});

  final ThreadState thread;
  final Suggestion suggestion;

  @override
  Widget build(BuildContext context) {
    final s = suggestion;
    final muted = Theme.of(context).textTheme.bodySmall?.copyWith(color: Tokens.textMuted);
    return Card(
      key: Key('suggestion-${s.id}'),
      color: Theme.of(context).colorScheme.secondaryContainer.withValues(alpha: 0.4),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              s.isPrompt ? 'Suggested prompt change' : 'Suggested memory',
              style: Theme.of(context).textTheme.titleSmall,
            ),
            if (s.reason case final r?) Text(r, style: muted),
            if (s.target case final t?) Text(t.reach, style: muted),
            const SizedBox(height: 8),
            if (s.isPrompt && s.target != null) ...[
              Text('Before', style: muted),
              SelectableText(s.target!.current),
              const SizedBox(height: 6),
              Text('After', style: muted),
            ],
            SelectableText(s.text),
            const SizedBox(height: 8),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                TextButton(
                  key: Key('suggestion-dismiss-${s.id}'),
                  onPressed: thread.busy ? null : () => unawaited(thread.dismiss(s.id)),
                  child: const Text('Dismiss'),
                ),
                const SizedBox(width: 8),
                FilledButton(
                  key: Key('suggestion-apply-${s.id}'),
                  onPressed: thread.busy ? null : () => unawaited(thread.apply(s.id)),
                  child: const Text('Apply'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

/// Message box with send and stop for a thread.
class ThreadComposer extends StatefulWidget {
  const ThreadComposer({super.key, required this.thread});

  final ThreadState thread;

  @override
  State<ThreadComposer> createState() => _ThreadComposerState();
}

class _ThreadComposerState extends State<ThreadComposer> {
  final _text = TextEditingController();

  @override
  void dispose() {
    _text.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final text = _text.text;
    if (text.trim().isEmpty) return;
    _text.clear();
    if (!await widget.thread.send(text) && mounted && _text.text.isEmpty) _text.text = text;
  }

  @override
  Widget build(BuildContext context) {
    final t = widget.thread;
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          Expanded(
            child: TextField(
              key: const Key('thread-input'),
              controller: _text,
              minLines: 1,
              maxLines: 5,
              decoration: const InputDecoration(hintText: 'Ask about this output', border: OutlineInputBorder()),
            ),
          ),
          const SizedBox(width: 8),
          if (t.streaming)
            OutlinedButton.icon(
              key: const Key('thread-stop'),
              onPressed: () => unawaited(t.stop()),
              icon: const Icon(Icons.stop),
              label: const Text('Stop'),
            )
          else
            FilledButton(
              key: const Key('thread-send'),
              onPressed: t.busy ? null : () => unawaited(_send()),
              style: FilledButton.styleFrom(backgroundColor: Tokens.accent),
              child: const Text('Send'),
            ),
        ],
      ),
    );
  }
}
