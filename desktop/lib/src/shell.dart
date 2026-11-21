import 'dart:async';

import 'package:material_ui/material_ui.dart';

import 'app_state.dart';
import 'screens/chat_screen.dart';
import 'screens/library_screen.dart';
import 'screens/new_notebook_dialog.dart';
import 'screens/schedules_screen.dart';
import 'screens/settings_screen.dart';
import 'screens/templates_screen.dart';
import 'screens/workspace_screen.dart';
import 'theme.dart';
import 'widgets/approvals_sheet.dart';

/// Window layout: graphite sidebar plus the active section.
class AppShell extends StatelessWidget {
  const AppShell({super.key, required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: state,
      builder: (context, _) {
        if (state.loading) {
          return const Scaffold(body: Center(child: CircularProgressIndicator()));
        }
        return Scaffold(
          body: Row(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              _Sidebar(state: state),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    if (state.notices.isNotEmpty) _NoticeBanner(state: state),
                    Expanded(child: _content()),
                  ],
                ),
              ),
            ],
          ),
        );
      },
    );
  }

  Widget _content() => switch (state.section) {
    Section.chat => ChatScreen(state: state),
    Section.notebooks => WorkspaceScreen(state: state),
    Section.templates => TemplatesScreen(state: state),
    Section.schedules => SchedulesScreen(state: state),
    Section.library => LibraryScreen(core: state.core),
    Section.settings => SettingsScreen(core: state.core),
  };
}

class _Sidebar extends StatelessWidget {
  const _Sidebar({required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 248,
      color: Tokens.sidebar,
      padding: const EdgeInsets.fromLTRB(12, 16, 12, 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const _Brand(),
          const SizedBox(height: 16),
          FilledButton.icon(
            key: const Key('new-notebook'),
            onPressed: () => showNewNotebookDialog(context, state),
            icon: const Icon(Icons.add, size: 18),
            label: const Text('New notebook'),
            style: FilledButton.styleFrom(backgroundColor: Tokens.accent),
          ),
          const SizedBox(height: 8),
          _NavItem(label: 'Chat', selected: state.section == Section.chat, onTap: () => state.go(Section.chat)),
          const SizedBox(height: 8),
          const _Caption('NOTEBOOKS'),
          Expanded(
            child: ListView(
              children: [
                for (final nb in state.notebooks)
                  _NavItem(
                    label: nb.name,
                    dot: colorsFor(nb.workType).dot,
                    trailing: nb.schedule,
                    selected: state.section == Section.notebooks && nb.id == state.selectedId,
                    onTap: () => state.select(nb.id),
                  ),
              ],
            ),
          ),
          const Divider(color: Tokens.sidebarSelected),
          if (state.approvals.isNotEmpty)
            KeyedSubtree(
              key: const Key('approvals-badge'),
              child: _NavItem(
                label: 'Approvals (${state.approvals.length})',
                dot: Colors.orange,
                selected: false,
                onTap: () => unawaited(showApprovalsSheet(context, state)),
              ),
            ),
          _NavItem(
            label: 'Templates',
            selected: state.section == Section.templates,
            onTap: () => state.go(Section.templates),
          ),
          _NavItem(
            label: 'Schedules',
            selected: state.section == Section.schedules,
            onTap: () => state.go(Section.schedules),
          ),
          _NavItem(
            label: 'Outputs library',
            selected: state.section == Section.library,
            onTap: () => state.go(Section.library),
          ),
          _NavItem(
            label: 'Settings',
            selected: state.section == Section.settings,
            onTap: () => state.go(Section.settings),
          ),
        ],
      ),
    );
  }
}

/// In-app notification for the newest core event (schedule finished or failed).
class _NoticeBanner extends StatelessWidget {
  const _NoticeBanner({required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    final extra = state.notices.length - 1;
    return MaterialBanner(
      key: const Key('notice-banner'),
      content: Text(extra > 0 ? '${state.notices.last}  (+$extra more)' : state.notices.last),
      leading: const Icon(Icons.notifications_outlined),
      actions: [
        TextButton(key: const Key('notice-dismiss'), onPressed: state.dismissNotice, child: const Text('Dismiss')),
      ],
    );
  }
}

class _Brand extends StatelessWidget {
  const _Brand();

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Container(
          width: 22,
          height: 22,
          decoration: BoxDecoration(color: Tokens.accent, borderRadius: BorderRadius.circular(6)),
        ),
        const SizedBox(width: 8),
        const Text(
          'Tessivo',
          style: TextStyle(color: Colors.white, fontWeight: FontWeight.w600, fontSize: 15),
        ),
      ],
    );
  }
}

class _Caption extends StatelessWidget {
  const _Caption(this.text);

  final String text;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(8, 4, 8, 4),
      child: Text(text, style: const TextStyle(color: Color(0xFF9298A5), fontSize: 11, letterSpacing: 0.5)),
    );
  }
}

class _NavItem extends StatelessWidget {
  const _NavItem({required this.label, required this.selected, required this.onTap, this.dot, this.trailing});

  final String label;
  final bool selected;
  final VoidCallback onTap;
  final Color? dot;
  final String? trailing;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: selected ? Tokens.sidebarSelected : Colors.transparent,
      borderRadius: BorderRadius.circular(6),
      child: InkWell(
        borderRadius: BorderRadius.circular(6),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 9),
          child: Row(
            children: [
              if (dot != null) ...[
                Container(
                  width: 8,
                  height: 8,
                  decoration: BoxDecoration(color: dot, shape: BoxShape.circle),
                ),
                const SizedBox(width: 10),
              ],
              Expanded(
                child: Text(
                  label,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: selected ? Colors.white : Tokens.sidebarText, fontSize: 13),
                ),
              ),
              if (trailing != null) Text(trailing!, style: const TextStyle(color: Color(0xFF9298A5), fontSize: 11)),
            ],
          ),
        ),
      ),
    );
  }
}
