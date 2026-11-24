import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../chat_state.dart';
import '../../core/core_client.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/fields.dart';
import '../../design/components/headers.dart';
import '../../design/components/menu.dart';
import '../../design/components/rows.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';
import '../../widgets/text_prompt_dialog.dart';
import '../chat_dialogs.dart';

/// "Today" for the same day as [now], "Last week" within seven days, else "Older".
String chatGroup(DateTime created, DateTime now) {
  final day = DateTime(now.year, now.month, now.day);
  if (!created.isBefore(day)) return 'Today';
  return now.difference(created).inDays <= 7 ? 'Last week' : 'Older';
}

/// The Chat side bar: search, New chat, then Pinned / Today / Last week /
/// Older, archived chats and the chat memory.
class ChatSideBar extends StatelessWidget {
  const ChatSideBar({super.key, required this.state});

  final AppState state;

  ChatState get chat => state.chat;

  Future<void> _act(BuildContext context, ChatSummary s, String action) async {
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

  Widget _row(BuildContext context, ChatSummary s) {
    void act(String a) => unawaited(_act(context, s, a));
    return TvTreeRow(
      key: Key('chat-${s.id}'),
      leading: TvIconView(TvIcon.chat, size: 14, color: TvColors.of(context).textSecondary),
      label: s.title.isEmpty ? 'New chat' : s.title,
      selected: chat.open?.id == s.id,
      onTap: () => unawaited(chat.select(s.id)),
      action: TvMenuButton(
        key: Key('chat-menu-${s.id}'),
        label: 'Chat actions',
        icon: TvIcon.more,
        compact: true,
        items: [
          TvMenuItem('Rename', onSelected: () => act('rename')),
          TvMenuItem(s.pinned ? 'Unpin' : 'Pin', onSelected: () => act('pin')),
          TvMenuItem(s.archived ? 'Unarchive' : 'Archive', onSelected: () => act('archive')),
          TvMenuItem('Delete', onSelected: () => act('delete')),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: chat,
    builder: (context, _) {
      final c = TvColors.of(context);
      final now = DateTime.now();
      final groups = <String, List<ChatSummary>>{'Pinned': [], 'Today': [], 'Last week': [], 'Older': []};
      for (final s in chat.sessions) {
        final at = DateTime.tryParse(s.updatedAt)?.toLocal();
        groups[s.pinned ? 'Pinned' : (at == null ? 'Older' : chatGroup(at, now))]!.add(s);
      }
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TvPanelHeader(
            chat.showArchived ? 'Archived chats' : 'Chats',
            actions: [
              TvIconButton(TvIcon.add, key: const Key('chat-new'), tooltip: 'New chat', onPressed: chat.newChat),
            ],
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(TvSpace.s12, 0, TvSpace.s12, TvSpace.s8),
            child: TextField(
              key: const Key('chat-search'),
              style: TvType.body.copyWith(color: c.textPrimary),
              cursorColor: c.interactive,
              decoration: tvFieldDecoration(c, hint: 'Search chats'),
              onChanged: chat.search,
            ),
          ),
          Expanded(
            child: ListView(
              children: [
                if (chat.sessions.isEmpty)
                  Padding(
                    padding: const EdgeInsets.all(TvSpace.s16),
                    child: Text(_emptyText(chat), style: TvType.body.copyWith(color: c.textSecondary)),
                  ),
                for (final MapEntry(key: name, value: list) in groups.entries)
                  if (list.isNotEmpty) ...[_GroupLabel(name), for (final s in list) _row(context, s)],
              ],
            ),
          ),
          Container(
            decoration: BoxDecoration(
              border: Border(top: BorderSide(color: c.borderSubtle)),
            ),
            child: Column(
              children: [
                TvTreeRow(
                  key: const Key('chat-archived'),
                  leading: TvIconView(chat.showArchived ? TvIcon.chat : TvIcon.folder, size: 14),
                  label: chat.showArchived ? 'Back to chats' : 'Archived',
                  muted: true,
                  onTap: () => unawaited(chat.toggleArchived()),
                ),
                TvTreeRow(
                  key: const Key('chat-memory'),
                  leading: const TvIconView(TvIcon.document, size: 14),
                  label: 'Memory · ${chat.memory.length}',
                  muted: true,
                  onTap: () => unawaited(showChatMemoryDialog(context, chat)),
                ),
              ],
            ),
          ),
        ],
      );
    },
  );

  static String _emptyText(ChatState chat) {
    if (chat.query.isNotEmpty) return 'No chats match.';
    return chat.showArchived ? 'Nothing archived.' : 'No chats yet.';
  }
}

class _GroupLabel extends StatelessWidget {
  const _GroupLabel(this.text);

  final String text;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(TvSpace.s16, TvSpace.s8, TvSpace.s16, TvSpace.s4),
    child: Text(text, style: TvType.caption.copyWith(color: TvColors.of(context).textHelper)),
  );
}
