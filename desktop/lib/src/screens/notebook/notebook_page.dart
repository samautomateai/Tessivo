import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/models.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/empty_state.dart';
import '../../design/components/headers.dart';
import '../../design/components/notices.dart';
import '../../design/components/tabs.dart';
import '../../design/components/tags.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../widgets/notebook_menu.dart';
import '../new_notebook_dialog.dart';
import 'agent_tab.dart';
import 'memory_tab.dart';
import 'output_tab.dart';
import 'prompt_tab.dart';
import 'relink_notice.dart';
import 'sources_tab.dart';

/// The notebook page: header (title, work type, size and model, Run), tabs
/// Output, Sources, Prompt, Memory and Agent. The thread is in the panel.
class NotebookPage extends StatefulWidget {
  const NotebookPage({super.key, required this.state});

  final AppState state;

  @override
  State<NotebookPage> createState() => _NotebookPageState();
}

class _NotebookPageState extends State<NotebookPage> {
  final _prompt = TextEditingController(text: 'Assess the risk of merging this change.');
  int _tab = 0;

  /// The saved prompt in the box, until its text is edited.
  String? _promptName;

  /// Read each source on its own first (a run override, and saved with a prompt).
  bool _perSource = false;

  @override
  void initState() {
    super.initState();
    _prompt.addListener(_detach);
  }

  void _detach() {
    final p = widget.state.prompts.where((p) => p.name == _promptName).firstOrNull;
    if (_promptName != null && p?.text != _prompt.text) setState(() => _promptName = null);
  }

  void _use(SavedPrompt p) {
    _prompt.text = p.text;
    setState(() {
      _promptName = p.name;
      _perSource = p.chain == chainPerSource;
    });
  }

  void _run() {
    setState(() => _tab = 0);
    final chain = _perSource ? chainPerSource : null;
    unawaited(widget.state.run(_prompt.text, const {}, promptName: _promptName, chain: chain));
  }

  @override
  void dispose() {
    _prompt.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final state = widget.state;
    final nb = state.selected;
    final c = TvColors.of(context);
    return ColoredBox(
      color: c.background,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (state.setupMessage case final m?) _ConnectModel(state: state, message: m),
          Expanded(child: nb == null ? _NoNotebooks(state: state) : _notebook(nb)),
        ],
      ),
    );
  }

  Widget _notebook(Notebook nb) {
    final state = widget.state;
    final model = state.defaultModel;
    final tokens = '≈ ${nb.enabledTokens ~/ 1000}k tokens';
    int? count(int n) => n == 0 ? null : n;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        TvPageHeader(
          title: nb.name,
          // At most three tags, in at most 320 px that scroll sideways; Edit tags… shows them all.
          tag: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 320),
            child: SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: Row(
                children: [
                  WorkTypeTag(nb.workType),
                  for (final t in nb.tags.take(3)) ...[
                    const SizedBox(width: TvSpace.s4),
                    TvTag(t, tone: TvTagTone.blue),
                  ],
                  if (nb.tags.length > 3) ...[const SizedBox(width: TvSpace.s4), TvTag('+${nb.tags.length - 3}')],
                ],
              ),
            ),
          ),
          meta: model == null || model.isEmpty ? tokens : '$tokens · $model',
          actions: [
            TvButton(
              state.running ? 'Running…' : 'Run',
              key: const Key('run'),
              icon: TvIcon.run,
              kind: TvButtonKind.primary,
              onPressed: state.running ? null : _run,
            ),
            NotebookMenu(state: state, notebook: nb),
          ],
        ),
        if (nb.context.any((c) => c.needsRelink)) RelinkNotice(state: state, notebook: nb),
        const SizedBox(height: TvSpace.s8),
        TvTabs(
          tabs: [
            const TvTabItem('Output', key: Key('tab-output')),
            TvTabItem('Sources', count: count(nb.context.length), key: const Key('tab-sources')),
            const TvTabItem('Prompt', key: Key('tab-prompt')),
            TvTabItem('Memory', count: count(nb.memory.length), key: const Key('tab-memory')),
            const TvTabItem('Agent', key: Key('tab-agent')),
          ],
          selected: _tab,
          onSelect: (i) => setState(() => _tab = i),
        ),
        Expanded(
          child: switch (_tab) {
            1 => SourcesTab(state: state, notebook: nb),
            2 => PromptTab(
              state: state,
              controller: _prompt,
              promptName: _promptName,
              onUse: _use,
              perSource: _perSource,
              onPerSource: (v) => setState(() => _perSource = v),
            ),
            3 => MemoryTab(state: state, notebook: nb),
            4 => AgentTab(state: state, notebook: nb),
            _ => OutputTab(state: state, onRun: state.running ? null : _run),
          },
        ),
      ],
    );
  }
}

/// First run: no usable model yet. Replaces the setup banner on this page.
class _ConnectModel extends StatelessWidget {
  const _ConnectModel({required this.state, required this.message});

  final AppState state;
  final String message;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s16, TvSpace.s24, 0),
    child: TvInlineNotice(
      key: const Key('setup-banner'),
      kind: TvNoticeKind.info,
      title: 'Connect a model',
      message: message,
      action: TvButton(
        'Connect a model',
        key: const Key('setup-open'),
        dense: true,
        kind: TvButtonKind.primary,
        onPressed: () => state.openSettings(SettingsSection.models),
      ),
    ),
  );
}

class _NoNotebooks extends StatelessWidget {
  const _NoNotebooks({required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) => TvEmptyState(
    key: const Key('no-notebooks'),
    icon: TvIcon.notebooks,
    title: 'No notebooks yet',
    message: 'A notebook keeps sources, a prompt and memory together, and every output it produces.',
    actionLabel: 'New notebook',
    onAction: () => unawaited(showNewNotebookDialog(context, state)),
    secondaryLabel: 'Start from a template',
    onSecondary: () => state.go(Section.templates),
  );
}
