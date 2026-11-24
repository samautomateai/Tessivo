import 'dart:async';

import 'package:material_ui/material_ui.dart';

import 'app_state.dart';
import 'core/models.dart';
import 'screens/chat_screen.dart';
import 'screens/library_screen.dart';
import 'screens/new_notebook_dialog.dart';
import 'screens/research_screen.dart';
import 'screens/schedules_screen.dart';
import 'screens/settings_screen.dart';
import 'screens/templates_screen.dart';
import 'screens/workspace_screen.dart';
import 'theme.dart';
import 'widgets/approvals_sheet.dart';
import 'widgets/command_palette.dart';
import 'widgets/search_box.dart';

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
          body: AppShortcuts(
            state: state,
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                _Sidebar(state: state),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      if (state.connectionLost)
                        const MaterialBanner(
                          key: Key('reconnecting'),
                          content: Text('Reconnecting to the Tessivo core…'),
                          actions: [SizedBox.shrink()],
                        ),
                      if (state.setupMessage != null) _SetupBanner(state: state),
                      if (state.notices.isNotEmpty) _NoticeBanner(state: state),
                      Expanded(child: _content()),
                    ],
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _content() => switch (state.section) {
    Section.chat => ChatScreen(state: state),
    Section.notebooks => WorkspaceScreen(state: state),
    Section.research => ResearchScreen(state: state),
    Section.templates => TemplatesScreen(state: state),
    Section.schedules => SchedulesScreen(state: state),
    Section.library => LibraryScreen(core: state.core),
    Section.settings => SettingsScreen(core: state.core, onChanged: () => unawaited(state.checkSetup())),
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
          SearchBox(state: state),
          const SizedBox(height: 12),
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
                  if (!nb.archived) _notebookItem(nb),
                if (state.notebooks.any((n) => n.archived))
                  KeyedSubtree(
                    key: const Key('archived-toggle'),
                    child: _NavItem(
                      label: '${state.showArchived ? '▾' : '▸'} Archived (${_archivedCount()})',
                      selected: false,
                      onTap: state.toggleArchived,
                    ),
                  ),
                if (state.showArchived)
                  for (final nb in state.notebooks)
                    if (nb.archived) _notebookItem(nb),
                const SizedBox(height: 8),
                const _Caption('RESEARCH'),
                for (final r in state.research)
                  KeyedSubtree(
                    key: Key('research-${r.id}'),
                    child: _NavItem(
                      label: r.name,
                      trailing: r.sources.isEmpty ? null : '${r.sources.length}',
                      selected: state.section == Section.research && r.id == state.researchState?.notebookId,
                      onTap: () => state.selectResearch(r.id),
                    ),
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

extension on _Sidebar {
  int _archivedCount() => state.notebooks.where((n) => n.archived).length;

  Widget _notebookItem(Notebook nb) => KeyedSubtree(
    key: Key('nav-${nb.id}'),
    child: _NavItem(
      label: nb.name,
      semanticLabel:
          '${nb.name}, ${workTypeLabel(nb.workType)} notebook${nb.schedule == null ? '' : ', runs ${nb.schedule}'}',
      dot: colorsFor(nb.workType).dot,
      trailing: nb.schedule,
      selected: state.section == Section.notebooks && nb.id == state.selectedId,
      onTap: () => state.select(nb.id),
    ),
  );
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
  const _NavItem({
    required this.label,
    required this.selected,
    required this.onTap,
    this.dot,
    this.trailing,
    this.semanticLabel,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;
  final Color? dot;
  final String? trailing;

  /// What a screen reader says (default: [label]); the dot's meaning goes here.
  final String? semanticLabel;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: selected,
      label: semanticLabel ?? label,
      excludeSemantics: true,
      onTap: onTap,
      child: _box(),
    );
  }

  Widget _box() {
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

/// First-run banner: what to set up before runs can work.
class _SetupBanner extends StatelessWidget {
  const _SetupBanner({required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    return MaterialBanner(
      key: const Key('setup-banner'),
      content: Text(state.setupMessage ?? ''),
      leading: const Icon(Icons.settings_suggest_outlined),
      actions: [
        TextButton(
          key: const Key('setup-open'),
          onPressed: () => state.go(Section.settings),
          child: const Text('Open settings'),
        ),
      ],
    );
  }
}
