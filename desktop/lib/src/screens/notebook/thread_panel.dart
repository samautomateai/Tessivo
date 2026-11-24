import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/core_client.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/fields.dart';
import '../../design/components/headers.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';
import '../../thread_state.dart';
import '../../widgets/chat_widgets.dart';

/// The secondary panel on a notebook page: the open output's conversation,
/// the suggestions it produced, and the box to ask about it.
class ThreadPanel extends StatelessWidget {
  const ThreadPanel({super.key, required this.state, required this.thread});

  final AppState state;
  final ThreadState thread;

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    key: const Key('output-thread'),
    listenable: thread,
    builder: (context, _) {
      final c = TvColors.of(context);
      final session = thread.session;
      final idle = !thread.busy && !thread.streaming;
      final hasReply = session?.messages.any((m) => m.role == 'assistant') ?? false;
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const TvPanelHeader('Thread'),
          Expanded(
            child: ListView(
              padding: const EdgeInsets.all(TvSpace.s12),
              children: [
                if (session != null && session.messages.isEmpty && !thread.streaming)
                  Text(
                    'Ask why it says something, where a claim comes from, or what to change.',
                    style: TvType.helper.copyWith(color: c.textHelper),
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
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: TvSpace.s12),
            child: Wrap(
              spacing: TvSpace.s8,
              runSpacing: TvSpace.s4,
              children: [
                TvButton(
                  'Suggest improvements',
                  key: const Key('suggest-improvements'),
                  dense: true,
                  kind: TvButtonKind.ghost,
                  onPressed: idle && hasReply ? () => unawaited(thread.suggest()) : null,
                ),
                TvButton(
                  'New version',
                  key: const Key('new-version'),
                  icon: TvIcon.newVersion,
                  dense: true,
                  onPressed: idle && hasReply && !state.running ? () => unawaited(state.newVersion()) : null,
                ),
              ],
            ),
          ),
          ThreadComposer(thread: thread),
        ],
      );
    },
  );
}

class _Note extends StatelessWidget {
  const _Note(this.text);

  final String text;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: TvSpace.s4),
    child: Text(
      text,
      textAlign: TextAlign.center,
      style: TvType.helper.copyWith(color: TvColors.of(context).textHelper),
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
    final c = TvColors.of(context);
    final s = suggestion;
    final muted = TvType.helper.copyWith(color: c.textHelper);
    final body = TvType.body.copyWith(color: c.textPrimary);
    return Container(
      key: Key('suggestion-${s.id}'),
      margin: const EdgeInsets.symmetric(vertical: TvSpace.s8),
      padding: const EdgeInsets.all(TvSpace.s12),
      decoration: BoxDecoration(
        color: c.layer02,
        border: Border(left: BorderSide(color: c.interactive, width: 3)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            s.isPrompt ? 'Suggested prompt change' : 'Suggested memory',
            style: TvType.heading01.copyWith(color: c.textPrimary),
          ),
          if (s.reason case final r?) Text(r, style: muted),
          if (s.target case final t?) Text(t.reach, style: muted),
          const SizedBox(height: TvSpace.s8),
          if (s.isPrompt && s.target != null) ...[
            Text('Before', style: muted),
            SelectableText(s.target!.current, style: body),
            const SizedBox(height: TvSpace.s4),
            Text('After', style: muted),
          ],
          SelectableText(s.text, style: body),
          const SizedBox(height: TvSpace.s8),
          Row(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              TvButton(
                'Dismiss',
                key: Key('suggestion-dismiss-${s.id}'),
                kind: TvButtonKind.ghost,
                dense: true,
                onPressed: thread.busy ? null : () => unawaited(thread.dismiss(s.id)),
              ),
              const SizedBox(width: TvSpace.s8),
              TvButton(
                'Apply',
                key: Key('suggestion-apply-${s.id}'),
                kind: TvButtonKind.primary,
                dense: true,
                onPressed: thread.busy ? null : () => unawaited(thread.apply(s.id)),
              ),
            ],
          ),
        ],
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
    final c = TvColors.of(context);
    final t = widget.thread;
    return Padding(
      padding: const EdgeInsets.all(TvSpace.s12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          Expanded(
            child: TextField(
              key: const Key('thread-input'),
              controller: _text,
              minLines: 1,
              maxLines: 5,
              style: TvType.body.copyWith(color: c.textPrimary),
              cursorColor: c.interactive,
              decoration: tvFieldDecoration(c, hint: 'Ask about this output'),
            ),
          ),
          const SizedBox(width: TvSpace.s8),
          if (t.streaming)
            TvIconButton(
              TvIcon.close,
              key: const Key('thread-stop'),
              tooltip: 'Stop',
              onPressed: () => unawaited(t.stop()),
            )
          else
            TvIconButton(
              TvIcon.send,
              key: const Key('thread-send'),
              tooltip: 'Send',
              color: c.interactive,
              onPressed: t.busy ? null : () => unawaited(_send()),
            ),
        ],
      ),
    );
  }
}
