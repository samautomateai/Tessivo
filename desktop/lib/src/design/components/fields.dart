import 'package:material_ui/material_ui.dart';

import '../colors.dart';
import '../sizes.dart';
import '../type.dart';

/// The label above a Carbon field and the helper or error text below it.
class _FieldFrame extends StatelessWidget {
  const _FieldFrame({required this.label, required this.child, this.helper, this.error});

  final String label;
  final String? helper;
  final String? error;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final note = error ?? helper;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (label.isNotEmpty) ...[
          Text(label, style: TvType.label.copyWith(color: c.textSecondary)),
          const SizedBox(height: TvSpace.s4),
        ],
        child,
        if (note != null) ...[
          const SizedBox(height: TvSpace.s4),
          Text(note, style: TvType.helper.copyWith(color: error != null ? c.error : c.textHelper)),
        ],
      ],
    );
  }
}

/// The Carbon field look: a layer fill with an underline that becomes a 2 px
/// outline on focus (interactive) or on error (error).
InputDecoration tvFieldDecoration(TvColors c, {String? hint, bool error = false}) {
  OutlineInputBorder outline(Color color) => OutlineInputBorder(
    borderRadius: BorderRadius.circular(TvRadius.small),
    borderSide: BorderSide(color: color, width: 2),
  );
  final underline = UnderlineInputBorder(borderSide: BorderSide(color: c.fieldUnderline));
  return InputDecoration(
    isDense: true,
    hintText: hint,
    hintStyle: TvType.body.copyWith(color: c.textHelper),
    filled: true,
    fillColor: c.layer01,
    contentPadding: const EdgeInsets.symmetric(horizontal: TvSpace.s12, vertical: 7),
    border: underline,
    enabledBorder: error ? outline(c.error) : underline,
    focusedBorder: outline(error ? c.error : c.interactive),
  );
}

/// A labelled text input, 32 px tall for one line.
class TvTextField extends StatelessWidget {
  const TvTextField({
    super.key,
    required this.label,
    this.controller,
    this.hint,
    this.helper,
    this.error,
    this.onChanged,
    this.onSubmitted,
    this.obscure = false,
    this.maxLines = 1,
    this.minLines,
    this.mono = false,
    this.fieldKey,
  });

  final String label;
  final TextEditingController? controller;
  final String? hint;
  final String? helper;
  final String? error;
  final ValueChanged<String>? onChanged;
  final ValueChanged<String>? onSubmitted;
  final bool obscure;
  final int maxLines;
  final int? minLines;
  final bool mono;

  /// The key of the inner [TextField], for tests that enter text.
  final Key? fieldKey;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return _FieldFrame(
      label: label,
      helper: helper,
      error: error,
      child: TextField(
        key: fieldKey,
        controller: controller,
        onChanged: onChanged,
        onSubmitted: onSubmitted,
        obscureText: obscure,
        maxLines: obscure ? 1 : maxLines,
        minLines: obscure ? null : minLines,
        style: (mono ? TvType.code : TvType.body).copyWith(color: c.textPrimary),
        cursorColor: c.interactive,
        decoration: tvFieldDecoration(c, hint: hint, error: error != null),
      ),
    );
  }
}

/// A labelled select over (value, label) pairs.
class TvDropdown<T> extends StatelessWidget {
  const TvDropdown({
    super.key,
    required this.label,
    required this.items,
    required this.value,
    required this.onChanged,
    this.helper,
  });

  final String label;
  final List<(T, String)> items;
  final T? value;
  final ValueChanged<T>? onChanged;
  final String? helper;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final change = onChanged;
    return _FieldFrame(
      label: label,
      helper: helper,
      child: Container(
        height: TvSize.input,
        padding: const EdgeInsets.only(left: TvSpace.s12, right: TvSpace.s8),
        decoration: BoxDecoration(
          color: c.layer01,
          border: Border(bottom: BorderSide(color: c.fieldUnderline)),
        ),
        child: DropdownButtonHideUnderline(
          child: DropdownButton<T>(
            value: value,
            isExpanded: true,
            isDense: true,
            dropdownColor: c.layer02,
            borderRadius: BorderRadius.circular(TvRadius.medium),
            iconEnabledColor: c.textSecondary,
            style: TvType.body.copyWith(color: c.textPrimary),
            focusColor: c.layer01,
            items: [for (final (v, text) in items) DropdownMenuItem<T>(value: v, child: Text(text))],
            onChanged: change == null
                ? null
                : (v) {
                    if (v != null) change(v);
                  },
          ),
        ),
      ),
    );
  }
}

/// A row that toggles a setting; the whole row is the hit target.
class _ChoiceRow extends StatelessWidget {
  const _ChoiceRow({required this.label, required this.control, required this.onTap});

  final String label;
  final Widget control;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(TvRadius.small),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: TvSpace.s4),
        child: Row(
          children: [
            control,
            const SizedBox(width: TvSpace.s8),
            Flexible(
              child: Text(label, style: TvType.body.copyWith(color: c.textPrimary)),
            ),
          ],
        ),
      ),
    );
  }
}

/// A Carbon toggle (small): a switch with its label to the right.
class TvToggle extends StatelessWidget {
  const TvToggle({super.key, required this.label, required this.value, required this.onChanged});

  final String label;
  final bool value;
  final ValueChanged<bool>? onChanged;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final change = onChanged;
    return _ChoiceRow(
      label: label,
      onTap: change == null ? null : () => change(!value),
      control: SizedBox(
        height: 20,
        child: FittedBox(
          child: Switch(
            value: value,
            onChanged: change,
            activeTrackColor: c.success,
            inactiveTrackColor: c.layer02,
            trackOutlineColor: WidgetStatePropertyAll(c.borderStrong),
          ),
        ),
      ),
    );
  }
}

/// A checkbox with its label to the right; 2 px corners.
class TvCheckbox extends StatelessWidget {
  const TvCheckbox({super.key, required this.label, required this.value, required this.onChanged});

  final String label;
  final bool value;
  final ValueChanged<bool>? onChanged;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final change = onChanged;
    return _ChoiceRow(
      label: label,
      onTap: change == null ? null : () => change(!value),
      control: SizedBox.square(
        dimension: 16,
        child: Checkbox(
          value: value,
          onChanged: change == null ? null : (v) => change(v ?? false),
          materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
          visualDensity: VisualDensity.compact,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(TvRadius.small)),
          side: BorderSide(color: c.textPrimary),
          activeColor: c.textPrimary,
          checkColor: c.background,
        ),
      ),
    );
  }
}
