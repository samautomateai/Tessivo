import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/models.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/notices.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';
import '../../widgets/text_prompt_dialog.dart';

/// After an import: the sources that pointed at the sender's disk, each with
/// Relink… to choose the folder or file on this computer.
class RelinkNotice extends StatelessWidget {
  const RelinkNotice({super.key, required this.state, required this.notebook});

  final AppState state;
  final Notebook notebook;

  Future<void> _relink(BuildContext context, ContextItem c) async {
    final path = await askText(
      context,
      title: 'Relink "${c.label}"',
      label: 'Absolute path to the folder or file on this computer',
      maxLength: 4096,
    );
    if (path != null && path.trim().isNotEmpty) await state.relinkContext(notebook.id, c.id, path.trim());
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final items = notebook.context.where((x) => x.needsRelink).toList();
    final n = items.length;
    return Padding(
      key: const Key('relink-notice'),
      padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s12, TvSpace.s24, 0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TvInlineNotice(
            kind: TvNoticeKind.warning,
            title: n == 1 ? '1 source needs relinking' : '$n sources need relinking',
            message: 'They pointed at folders on the computer this notebook came from; runs leave them out.',
          ),
          for (final item in items)
            Container(
              padding: const EdgeInsets.only(left: TvSpace.s12, right: TvSpace.s4),
              decoration: BoxDecoration(
                color: c.layer01,
                border: Border(bottom: BorderSide(color: c.borderSubtle)),
              ),
              child: Row(
                children: [
                  TvIconView(TvIcon.folder, size: 14, color: c.textSecondary),
                  const SizedBox(width: TvSpace.s8),
                  Expanded(
                    child: Text(
                      '${item.label} · ${item.kind}',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TvType.body.copyWith(color: c.textPrimary),
                    ),
                  ),
                  TvButton(
                    'Relink…',
                    key: Key('relink-${item.id}'),
                    dense: true,
                    kind: TvButtonKind.ghost,
                    onPressed: () => unawaited(_relink(context, item)),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}
