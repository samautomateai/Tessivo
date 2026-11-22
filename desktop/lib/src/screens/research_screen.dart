import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../research_state.dart';
import '../theme.dart';
import '../widgets/research_chat.dart';
import '../widgets/research_notes.dart';
import '../widgets/research_sources.dart';

/// A research notebook: sources, conversation and notes side by side, or as
/// tabs when the window is narrower than [wide].
class ResearchScreen extends StatelessWidget {
  const ResearchScreen({super.key, required this.state});

  final AppState state;

  /// Narrowest width that shows the three columns.
  static const wide = 1100.0;

  @override
  Widget build(BuildContext context) {
    final r = state.researchState;
    if (r == null) return const Center(child: Text('Pick a research notebook.'));
    return ListenableBuilder(
      listenable: r,
      builder: (context, _) => Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _title(r),
          const Divider(height: 1),
          Expanded(child: MediaQuery.sizeOf(context).width >= wide ? _columns(r) : _tabs(r)),
        ],
      ),
    );
  }

  Widget _title(ResearchState r) {
    final nb = r.notebook;
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 16, 20, 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(nb?.name ?? '', style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w600)),
          if (nb != null && nb.description.isNotEmpty)
            Text(nb.description, style: const TextStyle(color: Tokens.textMuted)),
        ],
      ),
    );
  }

  Widget _columns(ResearchState r) => Row(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      SizedBox(width: 300, child: ResearchSources(state: r)),
      const VerticalDivider(width: 1),
      Expanded(child: ResearchChatPane(state: r)),
      const VerticalDivider(width: 1),
      SizedBox(width: 300, child: ResearchNotes(state: r)),
    ],
  );

  Widget _tabs(ResearchState r) => DefaultTabController(
    length: 3,
    initialIndex: 1,
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const TabBar(
          tabs: [
            Tab(key: Key('tab-sources'), text: 'Sources'),
            Tab(key: Key('tab-chat'), text: 'Conversation'),
            Tab(key: Key('tab-notes'), text: 'Notes'),
          ],
        ),
        Expanded(
          child: TabBarView(
            children: [
              ResearchSources(state: r),
              ResearchChatPane(state: r),
              ResearchNotes(state: r),
            ],
          ),
        ),
      ],
    ),
  );
}
