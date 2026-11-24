import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../design/colors.dart';
import '../design/components/empty_state.dart';
import '../design/components/headers.dart';
import '../design/components/tabs.dart';
import '../design/icons.dart';
import '../widgets/research_chat.dart';
import '../widgets/research_notes.dart';
import '../widgets/research_sources.dart';
import 'new_notebook_dialog.dart';

/// A research notebook: tabs Sources, Ask and Notes under its header.
class ResearchScreen extends StatefulWidget {
  const ResearchScreen({super.key, required this.state});

  final AppState state;

  @override
  State<ResearchScreen> createState() => _ResearchScreenState();
}

class _ResearchScreenState extends State<ResearchScreen> {
  int _tab = 1;

  /// The notebook [_tab] belongs to; another one starts on Ask, or on
  /// Sources while it has none.
  String? _tabFor;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final state = widget.state;
    final r = state.researchState;
    if (r == null) {
      final none = state.research.isEmpty;
      return ColoredBox(
        color: c.background,
        child: TvEmptyState(
          key: const Key('no-research'),
          icon: TvIcon.research,
          title: none ? 'No research notebooks yet' : 'Pick a research notebook',
          message: 'Add files and folders, ask about them with references, and keep what matters as notes.',
          actionLabel: 'New research notebook',
          onAction: () => unawaited(showNewNotebookDialog(context, state, research: true)),
        ),
      );
    }
    return ListenableBuilder(
      listenable: r,
      builder: (context, _) {
        final nb = r.notebook;
        if (_tabFor != r.notebookId && nb != null) {
          _tabFor = r.notebookId;
          _tab = nb.sources.isEmpty ? 0 : 1;
        }
        int? count(int n) => n == 0 ? null : n;
        return ColoredBox(
          color: c.background,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TvPageHeader(title: nb?.name ?? '', meta: nb == null || nb.description.isEmpty ? null : nb.description),
              const SizedBox(height: 8),
              TvTabs(
                tabs: [
                  TvTabItem('Sources', count: count(nb?.sources.length ?? 0), key: const Key('tab-sources')),
                  const TvTabItem('Ask', key: Key('tab-chat')),
                  TvTabItem('Notes', count: count(r.notes.length), key: const Key('tab-notes')),
                ],
                selected: _tab,
                onSelect: (i) => setState(() => _tab = i),
              ),
              Expanded(
                child: switch (_tab) {
                  0 => ResearchSources(state: r),
                  2 => ResearchNotes(state: r),
                  _ => ResearchChatPane(state: r),
                },
              ),
            ],
          ),
        );
      },
    );
  }
}
