import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/components/menu.dart';
import '../design/components/notices.dart';
import '../design/components/segmented.dart';
import '../design/icons.dart';
import '../design/sizes.dart';
import '../design/type.dart';
import '../research_state.dart';
import 'text_prompt_dialog.dart';

/// The Sources tab: each source with its summary state and context level.
class ResearchSources extends StatelessWidget {
  const ResearchSources({super.key, required this.state});

  final ResearchState state;

  Future<void> _add(BuildContext context) async {
    final path = await askText(
      context,
      title: 'Add a source',
      label: 'Absolute path to a file or folder',
      maxLength: 4096,
    );
    if (path == null) return;
    try {
      checkContextPath(path);
    } on FormatException catch (e) {
      if (context.mounted) showTvToast(context, e.message, kind: TvNoticeKind.error);
      return;
    }
    await state.addSource(path);
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final sources = state.notebook?.sources ?? const <ResearchSource>[];
    return ListView(
      key: const Key('research-sources'),
      padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s16, TvSpace.s24, TvSpace.s24),
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                'Full sends the whole source, Summary its short summary, Off leaves it out.',
                style: TvType.helper.copyWith(color: c.textHelper),
              ),
            ),
            TvButton(
              'Add',
              key: const Key('research-add-source'),
              icon: TvIcon.add,
              dense: true,
              onPressed: () => unawaited(_add(context)),
            ),
          ],
        ),
        const SizedBox(height: TvSpace.s12),
        if (sources.isEmpty)
          Text(
            'Add a file or folder to ask about it. Each one gets a short summary.',
            style: TvType.body.copyWith(color: c.textSecondary),
          ),
        for (final s in sources) _SourceCard(state: state, source: s),
      ],
    );
  }
}

class _SourceCard extends StatelessWidget {
  const _SourceCard({required this.state, required this.source});

  final ResearchState state;
  final ResearchSource source;

  String get _status => switch (source.summary) {
    'ready' => source.tokens > 0 ? 'Summary ready · ≈ ${source.tokens ~/ 1000}k tokens in full' : 'Summary ready',
    'updating' => 'Updating summary…',
    'failed' => 'Summary failed${source.error == null ? '' : ': ${source.error}'}',
    'unreadable' => 'Cannot read this source${source.error == null ? '' : ': ${source.error}'}',
    _ => 'Summarizing…',
  };

  TvIcon get _icon => switch (source.kind) {
    'folder' => TvIcon.folder,
    'mcp-resource' => TvIcon.tools,
    _ => TvIcon.document,
  };

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final s = source;
    final bad = s.summary == 'failed' || s.summary == 'unreadable';
    return Container(
      key: Key('source-${s.id}'),
      padding: const EdgeInsets.fromLTRB(TvSpace.s12, TvSpace.s8, TvSpace.s8, TvSpace.s8),
      decoration: BoxDecoration(
        color: c.layer01,
        border: Border(bottom: BorderSide(color: c.borderSubtle)),
      ),
      child: Row(
        children: [
          TvIconView(_icon, color: c.textSecondary),
          const SizedBox(width: TvSpace.s12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                InkWell(
                  onTap: () => unawaited(showSourceText(context, state, s.id, s.label)),
                  child: Text(
                    s.label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TvType.body.copyWith(color: c.textPrimary),
                  ),
                ),
                Text(_status, style: TvType.helper.copyWith(color: bad ? c.error : c.textHelper)),
              ],
            ),
          ),
          const SizedBox(width: TvSpace.s8),
          TvSegmented<String>(
            value: s.level,
            onChanged: (level) => unawaited(state.setLevel(s.id, level)),
            segments: [
              TvSegment('full', 'Full', key: Key('level-${s.id}-full')),
              TvSegment('summary', 'Summary', key: Key('level-${s.id}-summary'), enabled: s.hasSummary),
              TvSegment('off', 'Off', key: Key('level-${s.id}-off')),
            ],
          ),
          TvMenuButton(
            key: Key('source-menu-${s.id}'),
            label: 'More',
            icon: TvIcon.more,
            compact: true,
            items: [
              TvMenuItem('Summarize again', onSelected: () => unawaited(state.refreshSummary(s.id))),
              TvMenuItem('Remove', onSelected: () => unawaited(state.removeSource(s.id))),
            ],
          ),
        ],
      ),
    );
  }
}

/// Shows a source's text, the cited file of a folder when [path] is set, or
/// its summary when [summary] is true.
Future<void> showSourceText(
  BuildContext context,
  ResearchState state,
  String sourceId,
  String title, {
  String? path,
  bool summary = false,
}) async {
  final read = await state.sourceText(sourceId);
  if (read == null || !context.mounted) return;
  final body = summary && read.summary.isNotEmpty ? read.summary : fileSection(read.text, path);
  final heading = [title, ?path, if (summary) 'summary'].join(' · ');
  await showDialog<void>(
    context: context,
    builder: (ctx) => AlertDialog(
      key: const Key('source-dialog'),
      title: Text(heading),
      content: SizedBox(width: 720, height: 480, child: SingleChildScrollView(child: SelectableText(body))),
      actions: [TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Close'))],
    ),
  );
}
