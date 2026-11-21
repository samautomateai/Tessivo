import 'package:material_ui/material_ui.dart';

import '../core/models.dart';
import '../theme.dart';

/// Pill showing a work type in its colour.
class WorkTypeBadge extends StatelessWidget {
  const WorkTypeBadge({super.key, required this.workType});

  final WorkTypeId workType;

  @override
  Widget build(BuildContext context) {
    final c = colorsFor(workType);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 3),
      decoration: BoxDecoration(color: c.background, borderRadius: BorderRadius.circular(999)),
      child: Text(
        workTypeLabel(workType),
        style: TextStyle(color: c.foreground, fontSize: 12, fontWeight: FontWeight.w500),
      ),
    );
  }
}
