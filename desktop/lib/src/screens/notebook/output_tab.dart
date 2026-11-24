import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/empty_state.dart';
import '../../design/components/headers.dart';
import '../../design/components/tags.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';
import '../../thread_state.dart';
import '../../widgets/compare_button.dart';
import '../../widgets/export_menu.dart';
import '../../widgets/markdown_view.dart';
import '../../widgets/output_edit_form.dart';
import '../../widgets/rerun_button.dart';
import '../approvals_page.dart';

/// Below this width the toolbar's actions show only their icons.
const _labelsMinWidth = 620.0;

/// The Output tab: the open output's toolbar, the run in progress, and the
/// output itself at reading width.
class OutputTab extends StatelessWidget {
  const OutputTab({super.key, required this.state, required this.onRun});

  final AppState state;
  final VoidCallback? onRun;

  @override
  Widget build(BuildContext context) {
    final thread = state.thread;
    final running = runActive(state);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (thread != null)
          ListenableBuilder(
            listenable: thread,
            builder: (context, _) => _OutputToolbar(state: state, thread: thread),
          ),
        RunPanel(state: state),
        Expanded(
          child: thread == null
              ? (running
                    ? const SizedBox.shrink()
                    : TvEmptyState(
                        icon: TvIcon.document,
                        title: 'No output yet',
                        message: 'Run the prompt to get the first output.',
                        actionLabel: 'Run',
                        onAction: onRun,
                      ))
              : ListenableBuilder(
                  listenable: thread,
                  builder: (context, _) => _OutputBody(thread: thread),
                ),
        ),
      ],
    );
  }
}

class _OutputBody extends StatelessWidget {
  const _OutputBody({required this.thread});

  final ThreadState thread;

  @override
  Widget build(BuildContext context) {
    final detail = thread.detail;
    if (detail == null) {
      return thread.loaded
          ? const TvEmptyState(
              icon: TvIcon.warning,
              title: 'This output could not be opened',
              message: 'Pick it again in the side bar, or run the prompt again.',
            )
          : const Center(child: SizedBox.square(dimension: 20, child: CircularProgressIndicator()));
    }
    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s16, TvSpace.s24, TvSpace.s32),
      child: Align(
        alignment: Alignment.topLeft,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: TvSize.readingWidth),
          child: SelectionArea(child: MarkdownView(detail.markdown)),
        ),
      ),
    );
  }
}

class _OutputToolbar extends StatelessWidget {
  const _OutputToolbar({required this.state, required this.thread});

  final AppState state;
  final ThreadState thread;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final width = MediaQuery.sizeOf(context).width;
    final shown = state.panelShown(width);
    final idle = !thread.busy && !thread.streaming && !state.running;
    final open = state.outputs.where((o) => o.id == thread.outputId).firstOrNull;
    return LayoutBuilder(
      builder: (context, box) {
        final compact = box.maxWidth < _labelsMinWidth;
        return TvToolbar(
          leading: Row(
            children: [
              TvTag('v${open?.version ?? 1}'),
              const SizedBox(width: TvSpace.s8),
              Flexible(child: Text(thread.detail?.title ?? open?.title ?? '')),
              if (thread.busy) ...[
                const SizedBox(width: TvSpace.s8),
                const SizedBox.square(dimension: 12, child: CircularProgressIndicator(strokeWidth: 2)),
              ],
            ],
          ),
          actions: [
            RerunButton(state: state, outputId: thread.outputId, enabled: idle, compact: compact),
            EditOutputButton(state: state, enabled: idle, compact: compact),
            if (open?.parentId != null)
              CompareButton(
                core: state.core,
                notebookId: thread.notebookId,
                outputId: thread.outputId,
                compact: compact,
              ),
            ExportMenu(
              core: state.core,
              notebookId: thread.notebookId,
              outputId: thread.outputId,
              title: thread.detail?.title ?? 'output',
              compact: compact,
            ),
            Container(width: 1, height: 16, margin: const EdgeInsets.symmetric(horizontal: 6), color: c.borderSubtle),
            TvIconButton(
              shown ? TvIcon.panelClose : TvIcon.panelOpen,
              key: const Key('toggle-thread'),
              tooltip: shown ? 'Hide thread (Ctrl J)' : 'Show thread (Ctrl J)',
              onPressed: () => unawaited(state.togglePanel(width: width)),
            ),
          ],
        );
      },
    );
  }
}

/// Whether a run is going on or just finished, so the run panel shows.
bool runActive(AppState state) =>
    state.running || state.outcome != null || state.streamedText.isNotEmpty || state.paused.isNotEmpty;

/// The run in progress (or just finished): status, tool calls, approvals and
/// the streamed text. Hidden when there is nothing to show.
class RunPanel extends StatelessWidget {
  const RunPanel({super.key, required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    if (!runActive(state)) return const SizedBox.shrink();
    final c = TvColors.of(context);
    final outcome = state.outcome;
    final muted = TvType.helper.copyWith(color: c.textHelper);
    return Container(
      constraints: const BoxConstraints(maxHeight: 280),
      decoration: BoxDecoration(
        color: c.layer01,
        border: Border(bottom: BorderSide(color: c.borderSubtle)),
      ),
      child: ListView(
        shrinkWrap: true,
        padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s12, TvSpace.s24, TvSpace.s12),
        children: [
          Row(
            children: [
              Text('Latest run', style: TvType.heading01.copyWith(color: c.textPrimary)),
              const SizedBox(width: TvSpace.s8),
              if (outcome != null)
                outcome.valid
                    ? const TvTag('Validated', tone: TvTagTone.green)
                    : const TvTag('Needs review', tone: TvTagTone.yellow),
            ],
          ),
          if (state.running) Text(state.status, style: muted),
          for (final line in state.toolLog) Text(line, style: TvType.code.copyWith(color: c.textSecondary)),
          if (state.paused.isNotEmpty) ...[
            const SizedBox(height: TvSpace.s8),
            Text('Waiting for approval', style: TvType.heading01.copyWith(color: c.textPrimary)),
            for (final a in state.paused) ApprovalTile(state: state, approval: a),
          ],
          const SizedBox(height: TvSpace.s4),
          Text(
            state.streamedText,
            key: const Key('output'),
            style: TvType.bodyLong.copyWith(color: c.textPrimary),
          ),
          if (outcome != null)
            Text(
              '${outcome.modelUsed} · ${outcome.totalTokens ~/ 1000}k tokens'
              '${outcome.fallbackNote == null ? '' : '\n${outcome.fallbackNote}'}',
              style: muted,
            ),
        ],
      ),
    );
  }
}
