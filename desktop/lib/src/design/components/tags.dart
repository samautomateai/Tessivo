import 'package:material_ui/material_ui.dart';

import '../../core/models.dart';
import '../colors.dart';
import '../sizes.dart';
import '../type.dart';
import '../work_types.dart';

/// Status colours for small tags (tool calls, rules, states).
enum TvTagTone { gray, blue, green, red, yellow }

const _tagText = TextStyle(fontFamily: TvType.sans, fontSize: 11, height: 16 / 11, fontWeight: FontWeight.w500);

/// A small 18 px label with 2 px corners.
class TvTag extends StatelessWidget {
  const TvTag(this.text, {super.key, this.tone = TvTagTone.gray});

  final String text;
  final TvTagTone tone;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final base = switch (tone) {
      TvTagTone.gray => c.layer02,
      TvTagTone.blue => c.info,
      TvTagTone.green => c.success,
      TvTagTone.red => c.error,
      TvTagTone.yellow => c.warning,
    };
    // A tint of the tone behind primary text keeps every tone readable in every theme.
    final bg = tone == TvTagTone.gray ? base : Color.alphaBlend(base.withValues(alpha: 0.24), c.background);
    return _Pill(text: text, background: bg, foreground: c.textPrimary, border: _hcBorder(c));
  }
}

/// The work type's tag in its Carbon colour.
class WorkTypeTag extends StatelessWidget {
  const WorkTypeTag(this.id, {super.key});

  final WorkTypeId id;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final t = c.tag(id);
    return _Pill(text: workTypeLabel(id), background: t.background, foreground: t.text, border: _hcBorder(c, t.text));
  }
}

Color? _hcBorder(TvColors c, [Color? color]) => identical(c, TvColors.highContrast) ? color ?? c.textPrimary : null;

class _Pill extends StatelessWidget {
  const _Pill({required this.text, required this.background, required this.foreground, this.border});

  final String text;
  final Color background;
  final Color foreground;
  final Color? border;

  @override
  Widget build(BuildContext context) => Container(
    height: 18,
    padding: const EdgeInsets.symmetric(horizontal: TvSpace.s8),
    alignment: Alignment.center,
    decoration: BoxDecoration(
      color: background,
      borderRadius: BorderRadius.circular(TvRadius.small),
      border: border == null ? null : Border.all(color: border!),
    ),
    child: Text(text, style: _tagText.copyWith(color: foreground), maxLines: 1),
  );
}

/// A count in a small red disc; nothing for 0, "99+" above 99.
class TvBadge extends StatelessWidget {
  const TvBadge(this.count, {super.key});

  final int count;

  @override
  Widget build(BuildContext context) {
    if (count <= 0) return const SizedBox.shrink();
    return Container(
      constraints: const BoxConstraints(minWidth: 14),
      height: 14,
      padding: const EdgeInsets.symmetric(horizontal: 3),
      alignment: Alignment.center,
      decoration: BoxDecoration(color: TvColors.of(context).error, borderRadius: BorderRadius.circular(7)),
      child: Text(
        count > 99 ? '99+' : '$count',
        style: TvType.caption.copyWith(color: const Color(0xFFFFFFFF), fontWeight: FontWeight.w600),
      ),
    );
  }
}

/// A round status dot (connection, tool results) or a square work-type dot.
class TvStatusDot extends StatelessWidget {
  const TvStatusDot(this.color, {super.key, this.size = 8, this.square = false});

  final Color color;
  final double size;
  final bool square;

  @override
  Widget build(BuildContext context) => Container(
    width: size,
    height: size,
    decoration: BoxDecoration(
      color: color,
      shape: square ? BoxShape.rectangle : BoxShape.circle,
      borderRadius: square ? BorderRadius.circular(1) : null,
    ),
  );
}
