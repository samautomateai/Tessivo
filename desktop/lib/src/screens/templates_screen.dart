import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/components/empty_state.dart';
import '../design/components/headers.dart';
import '../design/components/notices.dart';
import '../design/components/tags.dart';
import '../design/components/tile.dart';
import '../design/icons.dart';
import '../design/sizes.dart';
import '../design/type.dart';
import '../widgets/text_prompt_dialog.dart';

/// Template gallery: browse built-in and user templates, start a notebook from
/// one, save the selected notebook as a template, and remove user templates.
class TemplatesScreen extends StatefulWidget {
  const TemplatesScreen({super.key, required this.state});

  final AppState state;

  @override
  State<TemplatesScreen> createState() => _TemplatesScreenState();
}

class _TemplatesScreenState extends State<TemplatesScreen> {
  WorkTypeId? _filter;
  List<TemplateInfo> _items = const [];
  String? _error;
  int _request = 0;

  /// The template shown in the detail panel.
  String? _open;

  @override
  void initState() {
    super.initState();
    unawaited(_reload());
  }

  Future<void> _reload() async {
    final req = ++_request;
    try {
      final items = await widget.state.core.listTemplates(workType: _filter);
      if (mounted && req == _request) setState(() => _items = items);
    } on Object catch (e) {
      if (mounted) setState(() => _error = '$e');
    }
  }

  Future<void> _run(Future<void> Function() action) async {
    setState(() => _error = null);
    try {
      await action();
    } on Object catch (e) {
      if (mounted) setState(() => _error = e is FormatException ? e.message : '$e');
    }
  }

  Future<void> _use(TemplateInfo t) async {
    final name = await askText(context, title: 'New notebook from "${t.name}"', initial: t.name);
    if (name == null) return;
    await _run(() => widget.state.createFromTemplate(t.id, name: name));
  }

  Future<void> _save() async {
    final nb = widget.state.selected;
    if (nb == null) return;
    final name = await askText(context, title: 'Save "${nb.name}" as a template', initial: nb.name);
    if (name == null) return;
    await _run(() async {
      await widget.state.core.saveAsTemplate(nb.id, name: name);
      await _reload();
    });
  }

  Future<void> _delete(TemplateInfo t) => _run(() async {
    await widget.state.core.deleteTemplate(t.id);
    await _reload();
  });

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final nb = widget.state.selected;
    final open = _items.where((t) => t.id == _open).firstOrNull;
    return ColoredBox(
      color: c.background,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          LayoutBuilder(
            builder: (context, box) => TvPageHeader(
              title: 'Templates',
              actions: [
                TvToolButton(
                  nb == null ? 'Save notebook as template' : 'Save "${nb.name}" as template',
                  key: const Key('save-as-template'),
                  icon: TvIcon.templates,
                  compact: box.maxWidth < 640,
                  onPressed: nb == null ? null : () => unawaited(_save()),
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s4, TvSpace.s24, TvSpace.s12),
            child: Text(
              'Templates carry prompts, context slots and memory. Never your files or secrets.',
              style: TvType.helper.copyWith(color: c.textHelper),
            ),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: TvSpace.s24),
            child: Align(
              alignment: Alignment.centerLeft,
              child: WorkTypeFilter(
                keyPrefix: 'template-filter',
                value: _filter,
                onChanged: (w) {
                  setState(() => _filter = w);
                  unawaited(_reload());
                },
              ),
            ),
          ),
          if (_error case final e?)
            Padding(
              padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s12, TvSpace.s24, 0),
              child: TvInlineNotice(key: const Key('template-error'), kind: TvNoticeKind.error, title: e),
            ),
          Expanded(
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Expanded(
                  child: _items.isEmpty
                      ? const TvEmptyState(
                          key: Key('no-templates'),
                          icon: TvIcon.templates,
                          title: 'No templates here yet',
                          message: 'Save a notebook as a template to start others from it.',
                        )
                      : ListView(
                          padding: const EdgeInsets.all(TvSpace.s24),
                          children: [
                            Wrap(
                              spacing: TvSpace.s16,
                              runSpacing: TvSpace.s16,
                              children: [
                                for (final t in _items)
                                  _TemplateTile(
                                    template: t,
                                    selected: t.id == _open,
                                    onOpen: () => setState(() => _open = t.id == _open ? null : t.id),
                                    onUse: () => unawaited(_use(t)),
                                    onDelete: () => unawaited(_delete(t)),
                                  ),
                              ],
                            ),
                          ],
                        ),
                ),
                if (open != null) _TemplateDetail(template: open, onUse: () => unawaited(_use(open))),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

String _meta(TemplateInfo t) => [
  '${t.prompts.length} prompt${t.prompts.length == 1 ? '' : 's'}',
  if (t.scheduleCron != null) 'schedule ${t.scheduleCron}',
  if (t.isUser) 'v${t.version}',
  if (t.parent != null) 'extends ${t.parent}',
].join(' · ');

class _TemplateTile extends StatelessWidget {
  const _TemplateTile({
    required this.template,
    required this.selected,
    required this.onOpen,
    required this.onUse,
    required this.onDelete,
  });

  final TemplateInfo template;
  final bool selected;
  final VoidCallback onOpen;
  final VoidCallback onUse;
  final VoidCallback onDelete;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final t = template;
    return TvTile(
      key: Key('template-card-${t.id}'),
      selected: selected,
      onTap: onOpen,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              WorkTypeTag(t.workType),
              const Spacer(),
              Text(t.isUser ? 'Yours' : 'Built-in', style: TvType.helper.copyWith(color: c.textHelper)),
            ],
          ),
          const SizedBox(height: TvSpace.s12),
          Text(
            t.name,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TvType.heading01.copyWith(color: c.textPrimary),
          ),
          const SizedBox(height: TvSpace.s4),
          SizedBox(
            height: 54,
            child: Text(
              t.description,
              maxLines: 3,
              overflow: TextOverflow.ellipsis,
              style: TvType.body.copyWith(color: c.textSecondary),
            ),
          ),
          Text(
            _meta(t),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TvType.helper.copyWith(color: c.textHelper),
          ),
          const SizedBox(height: TvSpace.s12),
          Row(
            children: [
              TvButton('Use template', key: Key('template-use-${t.id}'), dense: true, onPressed: onUse),
              const Spacer(),
              if (t.isUser)
                TvIconButton(
                  TvIcon.close,
                  key: Key('template-delete-${t.id}'),
                  tooltip: 'Remove template',
                  onPressed: onDelete,
                ),
            ],
          ),
        ],
      ),
    );
  }
}

/// What a template brings: its prompts, the context it asks for, and more.
class _TemplateDetail extends StatelessWidget {
  const _TemplateDetail({required this.template, required this.onUse});

  final TemplateInfo template;
  final VoidCallback onUse;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final t = template;
    final label = TvType.caption.copyWith(color: c.textSecondary);
    final body = TvType.body.copyWith(color: c.textPrimary);
    return Container(
      key: const Key('template-detail'),
      width: TvSize.panel,
      decoration: BoxDecoration(
        color: c.layer01,
        border: Border(left: BorderSide(color: c.borderSubtle)),
      ),
      child: ListView(
        padding: const EdgeInsets.all(TvSpace.s16),
        children: [
          Align(alignment: Alignment.centerLeft, child: WorkTypeTag(t.workType)),
          const SizedBox(height: TvSpace.s8),
          Text(t.name, style: TvType.heading02.copyWith(color: c.textPrimary)),
          const SizedBox(height: TvSpace.s8),
          Text(t.description, style: TvType.body.copyWith(color: c.textSecondary)),
          const SizedBox(height: TvSpace.s16),
          Text('PROMPTS', style: label),
          if (t.prompts.isEmpty) Text('—', style: body),
          for (final p in t.prompts) Text(p, style: body),
          const SizedBox(height: TvSpace.s16),
          Text('CONTEXT IT ASKS FOR', style: label),
          if (t.slots.isEmpty) Text('—', style: body),
          for (final s in t.slots)
            Padding(
              padding: const EdgeInsets.only(top: TvSpace.s4),
              child: Text.rich(
                TextSpan(
                  children: [
                    TextSpan(text: s.required ? '${s.name} (required)' : s.name, style: body),
                    if (s.description.isNotEmpty)
                      TextSpan(
                        text: '\n${s.description}',
                        style: TvType.helper.copyWith(color: c.textHelper),
                      ),
                  ],
                ),
              ),
            ),
          const SizedBox(height: TvSpace.s16),
          Text(_meta(t), style: TvType.helper.copyWith(color: c.textHelper)),
          const SizedBox(height: TvSpace.s16),
          Align(
            alignment: Alignment.centerLeft,
            child: TvButton(
              'Use template',
              key: const Key('template-detail-use'),
              kind: TvButtonKind.primary,
              onPressed: onUse,
            ),
          ),
        ],
      ),
    );
  }
}
