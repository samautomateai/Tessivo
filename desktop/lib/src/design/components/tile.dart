import 'package:material_ui/material_ui.dart';

import '../../core/models.dart';
import '../colors.dart';
import '../sizes.dart';
import '../work_types.dart';
import 'segmented.dart';

/// A Carbon selectable tile (2 px corners): a layer box that highlights when
/// [selected], for galleries such as templates and outputs.
class TvTile extends StatelessWidget {
  const TvTile({super.key, required this.child, this.onTap, this.selected = false, this.width = 280});

  final Widget child;
  final VoidCallback? onTap;
  final bool selected;
  final double width;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return SizedBox(
      width: width,
      child: Material(
        color: selected ? c.layerSelected : c.layer01,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(TvRadius.small),
          side: BorderSide(color: selected ? c.interactive : c.borderSubtle, width: selected ? 2 : 1),
        ),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: onTap,
          hoverColor: c.layer02,
          child: Padding(padding: const EdgeInsets.all(TvSpace.s16), child: child),
        ),
      ),
    );
  }
}

/// All or one work type, as a content switcher; segment keys are
/// `<keyPrefix>-all` and `<keyPrefix>-<work type>`.
class WorkTypeFilter extends StatelessWidget {
  const WorkTypeFilter({super.key, required this.value, required this.onChanged, required this.keyPrefix});

  final WorkTypeId? value;
  final ValueChanged<WorkTypeId?> onChanged;
  final String keyPrefix;

  @override
  Widget build(BuildContext context) => SingleChildScrollView(
    scrollDirection: Axis.horizontal,
    child: TvSegmented<WorkTypeId?>(
      value: value,
      onChanged: onChanged,
      segments: [
        TvSegment(null, 'All', key: Key('$keyPrefix-all')),
        for (final w in WorkTypeId.values) TvSegment(w, workTypeLabel(w), key: Key('$keyPrefix-${w.name}')),
      ],
    ),
  );
}
