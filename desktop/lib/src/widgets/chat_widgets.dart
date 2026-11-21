import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/core_client.dart';
import '../theme.dart';
import 'markdown_view.dart';
import 'text_prompt_dialog.dart';

/// The conversation: stored turns plus the reply streaming in.
class MessageList extends StatefulWidget {
  const MessageList({super.key, required this.messages, this.streaming, this.onSave});

  final List<ChatMessage> messages;

  /// The reply arriving now, or null when idle.
  final String? streaming;

  /// Saves a reply to the library; null hides the action.
  final void Function(ChatMessage)? onSave;

  @override
  State<MessageList> createState() => _MessageListState();
}

class _MessageListState extends State<MessageList> {
  final _scroll = ScrollController();

  @override
  void didUpdateWidget(MessageList old) {
    super.didUpdateWidget(old);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scroll.hasClients) _scroll.jumpTo(_scroll.position.maxScrollExtent);
    });
  }

  @override
  void dispose() {
    _scroll.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final streaming = widget.streaming;
    if (widget.messages.isEmpty && streaming == null) {
      return const Center(
        child: Text('Say hello, or attach files and ask about them.', style: TextStyle(color: Tokens.textMuted)),
      );
    }
    return ListView(
      controller: _scroll,
      padding: const EdgeInsets.all(20),
      children: [
        for (final m in widget.messages) MessageBubble(message: m, onSave: widget.onSave),
        if (streaming != null)
          MessageBubble(
            key: const Key('chat-streaming'),
            message: ChatMessage(id: '', role: 'assistant', content: streaming.isEmpty ? '…' : streaming),
          ),
      ],
    );
  }
}

/// One turn: user text on the right, the assistant's Markdown on the left.
class MessageBubble extends StatelessWidget {
  const MessageBubble({super.key, required this.message, this.onSave});

  final ChatMessage message;
  final void Function(ChatMessage)? onSave;

  @override
  Widget build(BuildContext context) {
    final m = message;
    final scheme = Theme.of(context).colorScheme;
    final footer = [
      if (m.model != null && m.model!.isNotEmpty) m.model!,
      if (m.stopped) m.error == null ? 'stopped' : 'failed: ${m.error}',
    ].join(' · ');
    return Align(
      alignment: m.isUser ? Alignment.centerRight : Alignment.centerLeft,
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 760),
        child: Container(
          key: m.id.isEmpty ? null : Key('msg-${m.id}'),
          margin: const EdgeInsets.only(bottom: 12),
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: m.isUser ? scheme.primaryContainer : scheme.surfaceContainerLow,
            borderRadius: BorderRadius.circular(10),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (m.attachments.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(bottom: 6),
                  child: Wrap(
                    spacing: 6,
                    runSpacing: 4,
                    children: [
                      for (final a in m.attachments)
                        Chip(label: Text(_chipLabel(a)), visualDensity: VisualDensity.compact),
                    ],
                  ),
                ),
              if (m.isUser) SelectableText(m.content) else MarkdownView(m.content),
              if (footer.isNotEmpty || (!m.isUser && onSave != null && m.id.isNotEmpty && m.content.isNotEmpty))
                Row(
                  children: [
                    Expanded(
                      child: Text(footer, style: const TextStyle(color: Tokens.textMuted, fontSize: 12)),
                    ),
                    if (!m.isUser && onSave != null && m.id.isNotEmpty && m.content.isNotEmpty)
                      TextButton.icon(
                        key: Key('save-${m.id}'),
                        onPressed: () => onSave!(m),
                        icon: const Icon(Icons.bookmark_add_outlined, size: 16),
                        label: const Text('Save as output'),
                      ),
                  ],
                ),
            ],
          ),
        ),
      ),
    );
  }

  static String _chipLabel(ChatAttachment a) => a.tokens > 0 ? '${a.display} · ≈${a.tokens ~/ 1000}k tok' : a.display;
}

/// Message box with attachments, send and stop.
class Composer extends StatefulWidget {
  const Composer({super.key, required this.state});

  final AppState state;

  @override
  State<Composer> createState() => _ComposerState();
}

class _ComposerState extends State<Composer> {
  final _text = TextEditingController();

  @override
  void dispose() {
    _text.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final text = _text.text;
    final chat = widget.state.chat;
    if (text.trim().isEmpty || chat.sending) return;
    _text.clear();
    final sent = await chat.send(text);
    if (!sent && mounted && _text.text.isEmpty) _text.text = text; // give it back to fix and resend
  }

  Future<void> _attachPath() async {
    final path = await askText(context, title: 'Attach a file or folder', label: 'Absolute path', maxLength: 4096);
    if (path == null) return;
    try {
      checkContextPath(path);
      widget.state.chat.attach(ChatAttachment(kind: 'path', ref: path));
    } on FormatException catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
    }
  }

  @override
  Widget build(BuildContext context) {
    final chat = widget.state.chat;
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (chat.pending.isNotEmpty)
            Wrap(
              spacing: 6,
              children: [
                for (final a in chat.pending)
                  InputChip(key: Key('pending-${a.ref}'), label: Text(a.display), onDeleted: () => chat.detach(a)),
              ],
            ),
          Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              PopupMenuButton<String>(
                key: const Key('chat-attach'),
                tooltip: 'Attach',
                icon: const Icon(Icons.attach_file),
                onSelected: (v) {
                  if (v == 'path') {
                    _attachPath();
                  } else {
                    final nb = widget.state.notebooks.firstWhere((n) => n.id == v);
                    chat.attach(ChatAttachment(kind: 'notebook', ref: nb.id, label: nb.name));
                  }
                },
                itemBuilder: (_) => [
                  const PopupMenuItem(value: 'path', child: Text('File or folder…')),
                  for (final nb in widget.state.notebooks)
                    PopupMenuItem(value: nb.id, child: Text('Notebook: ${nb.name}')),
                ],
              ),
              Expanded(
                child: TextField(
                  key: const Key('chat-input'),
                  controller: _text,
                  minLines: 1,
                  maxLines: 6,
                  decoration: const InputDecoration(hintText: 'Message', border: OutlineInputBorder()),
                ),
              ),
              const SizedBox(width: 8),
              if (chat.sending)
                OutlinedButton.icon(
                  key: const Key('chat-stop'),
                  onPressed: chat.stop,
                  icon: const Icon(Icons.stop),
                  label: const Text('Stop'),
                )
              else
                FilledButton(
                  key: const Key('chat-send'),
                  onPressed: _send,
                  style: FilledButton.styleFrom(backgroundColor: Tokens.accent),
                  child: const Text('Send'),
                ),
            ],
          ),
        ],
      ),
    );
  }
}
