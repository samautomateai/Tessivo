import 'package:material_ui/material_ui.dart';

import '../colors.dart';
import '../sizes.dart';
import '../type.dart';

/// One choice of a [TvSegmented]; [enabled] false greys it out.
class TvSegment<T> {
  const TvSegment(this.value, this.label, {this.key, this.enabled = true});

  final T value;
  final String label;
  final Key? key;
  final bool enabled;
}

/// A Carbon content switcher (small, 24 px): one of a few choices.
class TvSegmented<T> extends StatelessWidget {
  const TvSegmented({super.key, required this.segments, required this.value, required this.onChanged});

  final List<TvSegment<T>> segments;
  final T value;
  final ValueChanged<T> onChanged;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return Container(
      height: TvSize.buttonDense,
      decoration: BoxDecoration(
        border: Border.all(color: c.borderStrong),
        borderRadius: BorderRadius.circular(TvRadius.small),
      ),
      clipBehavior: Clip.antiAlias,
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          for (final s in segments)
            Semantics(
              key: s.key,
              button: true,
              selected: s.value == value,
              enabled: s.enabled,
              child: Material(
                color: s.value == value ? c.textPrimary : c.layer01,
                child: InkWell(
                  onTap: s.enabled && s.value != value ? () => onChanged(s.value) : null,
                  child: Padding(
                    padding: const EdgeInsets.symmetric(horizontal: TvSpace.s12),
                    child: Center(
                      child: Text(
                        s.label,
                        style: TvType.label.copyWith(
                          color: s.value == value ? c.background : (s.enabled ? c.textSecondary : c.textHelper),
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}
