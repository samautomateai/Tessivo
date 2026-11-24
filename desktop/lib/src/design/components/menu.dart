import 'package:material_ui/material_ui.dart';

import '../colors.dart';
import '../icons.dart';
import '../sizes.dart';
import '../type.dart';
import 'buttons.dart';

/// One entry of a [TvMenuButton]; [divider] draws a line instead.
class TvMenuItem {
  const TvMenuItem(this.label, {required this.onSelected, this.key, this.enabled = true}) : divider = false;

  const TvMenuItem.divider() : label = '', onSelected = null, key = null, enabled = false, divider = true;

  final String label;
  final VoidCallback? onSelected;
  final Key? key;
  final bool enabled;
  final bool divider;
}

/// A toolbar button that opens a menu (4 px corners, layer-02).
class TvMenuButton extends StatelessWidget {
  const TvMenuButton({super.key, required this.label, required this.icon, required this.items, this.compact = false});

  final String label;
  final TvIcon icon;
  final List<TvMenuItem> items;
  final bool compact;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return MenuAnchor(
      style: MenuStyle(
        backgroundColor: WidgetStatePropertyAll(c.layer02),
        surfaceTintColor: WidgetStatePropertyAll(c.layer02),
        side: WidgetStatePropertyAll(BorderSide(color: c.borderSubtle)),
        shape: WidgetStatePropertyAll(RoundedRectangleBorder(borderRadius: BorderRadius.circular(TvRadius.medium))),
        padding: const WidgetStatePropertyAll(EdgeInsets.symmetric(vertical: TvSpace.s4)),
      ),
      menuChildren: [
        for (final item in items)
          if (item.divider)
            Divider(height: TvSpace.s8, color: c.borderSubtle)
          else
            MenuItemButton(
              key: item.key,
              onPressed: item.enabled ? item.onSelected : null,
              style: ButtonStyle(
                minimumSize: const WidgetStatePropertyAll(Size(180, TvSize.button)),
                padding: const WidgetStatePropertyAll(EdgeInsets.symmetric(horizontal: TvSpace.s12)),
                textStyle: const WidgetStatePropertyAll(TvType.body),
                foregroundColor: WidgetStateProperty.resolveWith(
                  (s) => s.contains(WidgetState.disabled) ? c.textHelper : c.textPrimary,
                ),
              ),
              child: Text(item.label),
            ),
      ],
      builder: (context, controller, _) => TvToolButton(
        label,
        icon: icon,
        compact: compact,
        onPressed: () => controller.isOpen ? controller.close() : controller.open(),
      ),
    );
  }
}
