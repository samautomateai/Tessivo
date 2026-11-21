import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../chat_state.dart';
import '../core/core_client.dart';
import '../theme.dart';
import '../widgets/chat_widgets.dart';
import '../widgets/text_prompt_dialog.dart';
import 'chat_dialogs.dart';

/// Chat: sessions on the left, the open conversation on the right.
class ChatScreen extends StatefulWidget {
  const ChatScreen({super.key, required this.state});

  final AppState state;

  @override
  State<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  ChatState get chat => widget.state.chat;
  List<String> _models = const [''];

  @override
  void initState() {
    super.initState();
    chat.load();
    _loadModels();
  }

  Future<void> _loadModels() async {
    try {
      final s = await widget.state.core.getSettings();
      if (mounted) setState(() => _models = ['', for (final a in s.aliases) a.name]);
    } on Object {
      // The default model still works without the alias list.
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: chat,
      builder: (context, _) => Row(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          SizedBox(width: 290, child: _SessionList(chat: chat)),
          const VerticalDivider(width: 1),
          Expanded(
            child: chat.open == null
                ? _Empty(onNew: chat.newChat)
                : _Conversation(state: widget.state, models: _models),
          ),
        ],
      ),
    );
  }
}

class _Empty extends StatelessWidget {
  const _Empty({required this.onNew});

  final VoidCallback onNew;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Text('Ask anything, attach files or notebooks, and keep the good answers.'),
          const SizedBox(height: 12),
          FilledButton.icon(
            key: const Key('chat-start'),
            onPressed: onNew,
            icon: const Icon(Icons.add, size: 18),
            label: const Text('New chat'),
          ),
        ],
      ),
    );
  }
}

class _SessionList extends StatelessWidget {
  const _SessionList({required this.chat});

  final ChatState chat;

  Future<void> _menu(BuildContext context, ChatSummary s, String action) async {
    switch (action) {
      case 'rename':
        final title = await askText(context, title: 'Rename chat', initial: s.title, maxLength: maxChatTitle);
        if (title != null) await chat.rename(s.id, title);
      case 'pin':
        await chat.setPinned(s.id, !s.pinned);
      case 'archive':
        await chat.setArchived(s.id, !s.archived);
      case 'delete':
        await chat.delete(s.id);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(12, 12, 8, 4),
          child: Row(
            children: [
              Expanded(
                child: TextField(
                  key: const Key('chat-search'),
                  decoration: const InputDecoration(
                    isDense: true,
                    prefixIcon: Icon(Icons.search, size: 18),
                    hintText: 'Search chats',
                  ),
                  onChanged: chat.search,
                ),
              ),
              IconButton(
                key: const Key('chat-new'),
                tooltip: 'New chat',
                icon: const Icon(Icons.edit_square),
                onPressed: chat.newChat,
              ),
            ],
          ),
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12),
          child: Row(
            children: [
              FilterChip(
                key: const Key('chat-archived'),
                label: const Text('Archived'),
                selected: chat.showArchived,
                onSelected: (_) => chat.toggleArchived(),
              ),
              const Spacer(),
              TextButton.icon(
                key: const Key('chat-memory'),
                onPressed: () => showChatMemoryDialog(context, chat),
                icon: const Icon(Icons.psychology_outlined, size: 18),
                label: Text('Memory (${chat.memory.length})'),
              ),
            ],
          ),
        ),
        const SizedBox(height: 4),
        Expanded(
          child: chat.sessions.isEmpty
              ? Padding(
                  padding: const EdgeInsets.all(16),
                  child: Text(
                    _emptyText(chat),
                    style: const TextStyle(color: Tokens.textMuted),
                  ),
                )
              : ListView(children: [for (final s in chat.sessions) _tile(context, s)]),
        ),
      ],
    );
  }

  static String _emptyText(ChatState chat) {
    if (chat.query.isNotEmpty) return 'No chats match.';
    return chat.showArchived ? 'Nothing archived.' : 'No chats yet.';
  }

  Widget _tile(BuildContext context, ChatSummary s) => ListTile(
    key: Key('chat-${s.id}'),
    dense: true,
    selected: chat.open?.id == s.id,
    leading: s.pinned ? const Icon(Icons.push_pin, size: 16) : null,
    title: Text(s.title.isEmpty ? 'New chat' : s.title, maxLines: 1, overflow: TextOverflow.ellipsis),
    subtitle: Text(s.preview, maxLines: 1, overflow: TextOverflow.ellipsis),
    onTap: () => chat.select(s.id),
    trailing: PopupMenuButton<String>(
      key: Key('chat-menu-${s.id}'),
      tooltip: 'Chat actions',
      onSelected: (a) => _menu(context, s, a),
      itemBuilder: (_) => [
        const PopupMenuItem(value: 'rename', child: Text('Rename')),
        PopupMenuItem(value: 'pin', child: Text(s.pinned ? 'Unpin' : 'Pin')),
        PopupMenuItem(value: 'archive', child: Text(s.archived ? 'Unarchive' : 'Archive')),
        const PopupMenuItem(value: 'delete', child: Text('Delete')),
      ],
    ),
  );
}

/// Thinking effort choices; '' uses the app default.
const chatThinking = ['', 'off', 'low', 'medium', 'high'];

class _Conversation extends StatelessWidget {
  const _Conversation({required this.state, required this.models});

  final AppState state;
  final List<String> models;

  ChatState get chat => state.chat;

  @override
  Widget build(BuildContext context) {
    final s = chat.open!;
    final modelChoices = {...models, s.model}.toList();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 12, 12, 8),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  s.title.isEmpty ? 'New chat' : s.title,
                  style: Theme.of(context).textTheme.titleMedium,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              DropdownButton<String>(
                key: const Key('chat-model'),
                value: s.model,
                items: [
                  for (final m in modelChoices)
                    DropdownMenuItem(value: m, child: Text(m.isEmpty ? 'Default model' : m)),
                ],
                onChanged: chat.sending ? null : (m) => chat.setModel(m ?? ''),
              ),
              const SizedBox(width: 12),
              DropdownButton<String>(
                key: const Key('chat-thinking'),
                value: chatThinking.contains(s.thinkingEffort) ? s.thinkingEffort : '',
                items: [
                  for (final t in chatThinking)
                    DropdownMenuItem(value: t, child: Text(t.isEmpty ? 'Default thinking' : 'Thinking: $t')),
                ],
                onChanged: chat.sending ? null : (t) => chat.setThinking(t ?? ''),
              ),
              const SizedBox(width: 12),
              OutlinedButton.icon(
                key: const Key('chat-repeat'),
                onPressed: chat.sending || s.messages.isEmpty ? null : () => showRepeatDialog(context, state),
                icon: const Icon(Icons.repeat, size: 18),
                label: const Text('Make this repeatable'),
              ),
            ],
          ),
        ),
        if (chat.error != null)
          MaterialBanner(
            key: const Key('chat-error'),
            content: Text(chat.error!),
            actions: [TextButton(onPressed: chat.clearError, child: const Text('Dismiss'))],
          ),
        const Divider(height: 1),
        Expanded(
          child: MessageList(
            messages: s.messages,
            streaming: chat.sending ? chat.streaming : null,
            onSave: chat.sending ? null : (m) => showSaveReplyDialog(context, chat, m),
          ),
        ),
        const Divider(height: 1),
        Composer(state: state),
      ],
    );
  }
}
