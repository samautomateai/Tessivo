import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../theme.dart';

/// Right pane of a notebook: its outputs, newest first, with version lineage.
class NotebookOutputsPane extends StatelessWidget {
  const NotebookOutputsPane({super.key, required this.state});

  final AppState state;

  /// "v2 ← v1" for a revision whose parent is listed, else "vN".
  static String lineage(OutputSummary o, List<OutputSummary> all) {
    final parent = all.where((p) => p.id == o.parentId).firstOrNull;
    return parent == null ? 'v${o.version}' : 'v${o.version} ← v${parent.version}';
  }

  @override
  Widget build(BuildContext context) {
    final outputs = state.outputs;
    return Container(
      key: const Key('outputs-pane'),
      width: 300,
      decoration: BoxDecoration(
        border: Border(left: BorderSide(color: Theme.of(context).dividerColor)),
      ),
      child: ListView(
        padding: const EdgeInsets.symmetric(vertical: 16, horizontal: 8),
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(8, 0, 8, 8),
            child: Text('Outputs', style: Theme.of(context).textTheme.titleSmall),
          ),
          if (outputs.isEmpty)
            const Padding(
              padding: EdgeInsets.all(8),
              child: Text('No outputs yet.', style: TextStyle(color: Tokens.textMuted)),
            ),
          for (final o in outputs)
            ListTile(
              key: Key('output-${o.id}'),
              dense: true,
              selected: o.id == state.selectedOutputId,
              onTap: () => state.selectOutput(o.id),
              leading: Icon(o.trigger == 'Scheduled' ? Icons.schedule : Icons.description_outlined, size: 18),
              title: Text(o.title, maxLines: 2, overflow: TextOverflow.ellipsis),
              subtitle: Text('${lineage(o, outputs)} · ${o.createdAt}'),
            ),
        ],
      ),
    );
  }
}
