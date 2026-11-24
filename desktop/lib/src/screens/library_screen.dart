import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../core/models.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/components/empty_state.dart';
import '../design/components/headers.dart';
import '../design/components/tags.dart';
import '../design/components/tile.dart';
import '../design/icons.dart';
import '../design/sizes.dart';
import '../design/type.dart';
import '../widgets/markdown_view.dart';

/// Every saved output, filtered by work type, as tiles; the picked one opens
/// in the detail panel.
class LibraryScreen extends StatefulWidget {
  const LibraryScreen({super.key, required this.core});

  final CoreClient core;

  @override
  State<LibraryScreen> createState() => _LibraryScreenState();
}

class _LibraryScreenState extends State<LibraryScreen> {
  WorkTypeId? _filter;
  late Future<List<OutputSummary>> _outputs = widget.core.listOutputs();
  OutputSummary? _open;
  Future<OutputDetail>? _detail;

  void _setFilter(WorkTypeId? f) {
    setState(() {
      _filter = f;
      _outputs = widget.core.listOutputs(workType: f);
    });
  }

  void _show(OutputSummary o) {
    if (o.notebookId.isEmpty) return;
    final detail = widget.core.getOutput(o.notebookId, o.id);
    detail.ignore(); // the panel's FutureBuilder shows any error; don't report it as unhandled first
    setState(() {
      _open = o;
      _detail = detail;
    });
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final open = _open;
    return ColoredBox(
      color: c.background,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const TvPageHeader(title: 'Outputs'),
          Padding(
            padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s12, TvSpace.s24, 0),
            child: Align(
              alignment: Alignment.centerLeft,
              child: WorkTypeFilter(keyPrefix: 'filter', value: _filter, onChanged: _setFilter),
            ),
          ),
          Expanded(
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Expanded(
                  child: FutureBuilder<List<OutputSummary>>(
                    future: _outputs,
                    builder: (context, snap) {
                      if (!snap.hasData) return const Center(child: CircularProgressIndicator());
                      final items = snap.data!;
                      if (items.isEmpty) {
                        return const TvEmptyState(
                          icon: TvIcon.document,
                          title: 'No outputs yet.',
                          message: 'Every run of a notebook saves its output here.',
                        );
                      }
                      return ListView(
                        padding: const EdgeInsets.all(TvSpace.s24),
                        children: [
                          Wrap(
                            spacing: TvSpace.s16,
                            runSpacing: TvSpace.s16,
                            children: [for (final o in items) _tile(c, o)],
                          ),
                        ],
                      );
                    },
                  ),
                ),
                if (open != null) _panel(c, open),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _tile(TvColors c, OutputSummary o) => TvTile(
    key: Key('output-${o.id}'),
    selected: _open?.id == o.id,
    onTap: o.notebookId.isEmpty ? null : () => _show(o),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            WorkTypeTag(o.workType),
            const Spacer(),
            Text('v${o.version}', style: TvType.helper.copyWith(color: c.textHelper)),
          ],
        ),
        const SizedBox(height: TvSpace.s12),
        Text(
          o.title,
          maxLines: 2,
          overflow: TextOverflow.ellipsis,
          style: TvType.heading01.copyWith(color: c.textPrimary),
        ),
        const SizedBox(height: TvSpace.s4),
        Text(
          '${o.notebookName} · ${o.trigger} · ${o.createdAt}',
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: TvType.helper.copyWith(color: c.textHelper),
        ),
      ],
    ),
  );

  Widget _panel(TvColors c, OutputSummary o) => Container(
    key: const Key('output-detail'),
    width: 420,
    decoration: BoxDecoration(
      color: c.layer01,
      border: Border(left: BorderSide(color: c.borderSubtle)),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        TvPanelHeader(
          o.title,
          actions: [
            TvIconButton(
              TvIcon.close,
              key: const Key('output-detail-close'),
              tooltip: 'Close',
              onPressed: () => setState(() => _open = null),
            ),
          ],
        ),
        Expanded(
          child: FutureBuilder<OutputDetail>(
            future: _detail,
            builder: (context, snap) {
              if (snap.hasError) {
                return Padding(
                  padding: const EdgeInsets.all(TvSpace.s16),
                  child: Text('Could not open this output: ${snap.error}', style: TvType.body.copyWith(color: c.error)),
                );
              }
              if (!snap.hasData) return const Center(child: CircularProgressIndicator());
              return SingleChildScrollView(
                padding: const EdgeInsets.all(TvSpace.s16),
                child: KeyedSubtree(key: const Key('output-markdown'), child: MarkdownView(snap.data!.markdown)),
              );
            },
          ),
        ),
      ],
    ),
  );
}
