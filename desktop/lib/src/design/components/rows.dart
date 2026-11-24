import 'package:material_ui/material_ui.dart';

import '../colors.dart';
import '../sizes.dart';
import '../type.dart';

/// A 26 px side-bar row: leading icon or dot, a label that truncates with a
/// tooltip, optional trailing meta. Selected rows get a layer and a 2 px
/// accent at the left, as in VS Code.
class TvTreeRow extends StatelessWidget {
  const TvTreeRow({
    super.key,
    required this.leading,
    required this.label,
    this.trailing,
    this.selected = false,
    this.onTap,
    this.muted = false,
  });

  final Widget leading;
  final String label;
  final String? trailing;
  final bool selected;
  final VoidCallback? onTap;

  /// Older or secondary items (earlier output versions) in secondary text.
  final bool muted;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return Semantics(
      selected: selected,
      button: onTap != null,
      child: Material(
        color: selected ? c.layerSelected : const Color(0x00000000),
        child: InkWell(
          onTap: onTap,
          hoverColor: c.layer02,
          child: Container(
            height: TvSize.treeRow,
            padding: const EdgeInsets.only(left: TvSpace.s16, right: TvSpace.s12),
            decoration: BoxDecoration(
              border: Border(left: BorderSide(color: selected ? c.interactive : const Color(0x00000000), width: 2)),
            ),
            child: Row(
              children: [
                leading,
                const SizedBox(width: TvSpace.s8),
                Expanded(
                  child: Tooltip(
                    message: label,
                    waitDuration: const Duration(milliseconds: 600),
                    child: Text(
                      label,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TvType.body.copyWith(color: muted ? c.textSecondary : c.textPrimary),
                    ),
                  ),
                ),
                if (trailing != null) ...[
                  const SizedBox(width: TvSpace.s8),
                  Text(trailing!, style: TvType.helper.copyWith(color: c.textHelper)),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}
