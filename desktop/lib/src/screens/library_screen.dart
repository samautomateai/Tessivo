import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../core/models.dart';
import '../theme.dart';
import '../widgets/work_type_badge.dart';

/// Outputs library filtered by work type.
class LibraryScreen extends StatefulWidget {
  const LibraryScreen({super.key, required this.core});

  final CoreClient core;

  @override
  State<LibraryScreen> createState() => _LibraryScreenState();
}

class _LibraryScreenState extends State<LibraryScreen> {
  WorkTypeId? _filter;
  late Future<List<OutputSummary>> _outputs = widget.core.listOutputs();

  void _setFilter(WorkTypeId? f) {
    setState(() {
      _filter = f;
      _outputs = widget.core.listOutputs(workType: f);
    });
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(24),
      children: [
        Text(
          'Outputs library',
          style: Theme.of(context).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w600),
        ),
        const SizedBox(height: 16),
        Wrap(
          spacing: 8,
          children: [
            ChoiceChip(
              key: const Key('filter-all'),
              label: const Text('All'),
              selected: _filter == null,
              onSelected: (_) => _setFilter(null),
            ),
            for (final t in WorkTypeId.values)
              ChoiceChip(
                key: Key('filter-${t.name}'),
                label: Text(workTypeLabel(t)),
                selected: _filter == t,
                onSelected: (_) => _setFilter(t),
              ),
          ],
        ),
        const SizedBox(height: 16),
        FutureBuilder<List<OutputSummary>>(
          future: _outputs,
          builder: (context, snap) {
            if (!snap.hasData) {
              return const Center(child: CircularProgressIndicator());
            }
            final items = snap.data!;
            if (items.isEmpty) {
              return const Text('No outputs yet.');
            }
            return Card(
              child: Column(children: [for (final o in items) _row(o)]),
            );
          },
        ),
      ],
    );
  }

  Widget _row(OutputSummary o) => ListTile(
    key: Key('output-${o.id}'),
    title: Text(o.title),
    subtitle: Text('${o.notebookName} · ${o.trigger} · ${o.createdAt}'),
    leading: WorkTypeBadge(workType: o.workType),
    trailing: Text('v${o.versions}', style: const TextStyle(color: Tokens.textMuted)),
  );
}
