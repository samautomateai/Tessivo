import 'package:material_ui/material_ui.dart';

import '../colors.dart';
import '../sizes.dart';
import '../type.dart';

/// One tab: its label and an optional count shown as "Label · n".
class TvTabItem {
  const TvTabItem(this.label, {this.count, this.key});
  final String label;
  final int? count;
  final Key? key;

  String get text => (count ?? 0) > 0 ? '$label · $count' : label;
}

/// Carbon line tabs: 36 px, the selected tab marked by a 2 px line.
class TvTabs extends StatelessWidget {
  const TvTabs({super.key, required this.tabs, required this.selected, required this.onSelect});

  final List<TvTabItem> tabs;
  final int selected;
  final ValueChanged<int> onSelect;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return Container(
      height: TvSize.tabBar,
      decoration: BoxDecoration(
        border: Border(bottom: BorderSide(color: c.borderSubtle)),
      ),
      // Scrolls sideways when the tabs do not fit (narrow window, large zoom).
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            for (final (i, t) in tabs.indexed)
              Semantics(
                key: t.key,
                selected: i == selected,
                button: true,
                child: InkWell(
                  onTap: () => onSelect(i),
                  hoverColor: c.layer01,
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 14),
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      border: Border(
                        bottom: BorderSide(color: i == selected ? c.interactive : const Color(0x00000000), width: 2),
                      ),
                    ),
                    child: Text(
                      t.text,
                      style: TvType.body.copyWith(
                        color: i == selected ? c.textPrimary : c.textSecondary,
                        fontWeight: i == selected ? FontWeight.w500 : FontWeight.w400,
                      ),
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
