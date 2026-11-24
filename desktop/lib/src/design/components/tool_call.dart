import 'package:material_ui/material_ui.dart';

import '../colors.dart';
import '../icons.dart';
import '../sizes.dart';
import '../type.dart';

/// Lines a tool block renders; the rest is summarised, since tool output is
/// untrusted and may be huge.
const maxToolLines = 400;

/// The colour of one output line: failures in the error colour, passes in
/// the success colour, the rest secondary.
Color toolLineColor(TvColors c, String line) {
  final t = line.trimLeft();
  if (t.startsWith('--- FAIL') || t.startsWith('FAIL') || t.startsWith('panic:')) return c.error;
  if (t.startsWith('ok ') || t.startsWith('ok\t') || t.startsWith('--- PASS') || t.startsWith('PASS')) {
    return c.success;
  }
  return c.textSecondary;
}

/// A tool call in a chat or an agent run: one 32 px row (icon, tool, tags,
/// duration) that expands to its output in mono.
class ToolCallBlock extends StatefulWidget {
  const ToolCallBlock({
    super.key,
    required this.icon,
    required this.tool,
    this.tags = const [],
    this.duration,
    this.output = '',
    this.initiallyExpanded = false,
  });

  final TvIcon icon;
  final String tool;
  final List<Widget> tags;
  final String? duration;
  final String output;
  final bool initiallyExpanded;

  @override
  State<ToolCallBlock> createState() => _ToolCallBlockState();
}

class _ToolCallBlockState extends State<ToolCallBlock> {
  late bool _open = widget.initiallyExpanded;

  Widget _head(TvColors c, bool hasOutput) => Row(
    children: [
      AnimatedRotation(
        turns: _open ? 0.25 : 0,
        duration: TvMotion.fast,
        child: Opacity(
          opacity: hasOutput ? 1 : 0,
          child: TvIconView(TvIcon.expand, size: 12, color: c.textSecondary),
        ),
      ),
      const SizedBox(width: TvSpace.s4),
      TvIconView(widget.icon, size: 14, color: c.textSecondary),
      const SizedBox(width: TvSpace.s8),
      Flexible(
        child: Text(
          widget.tool,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: TvType.code.copyWith(color: c.textPrimary),
        ),
      ),
      for (final t in widget.tags) ...[const SizedBox(width: TvSpace.s8), t],
      const Spacer(),
      if (widget.duration case final d?) Text(d, style: TvType.helper.copyWith(color: c.textHelper)),
    ],
  );

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final hasOutput = widget.output.isNotEmpty;
    return Container(
      decoration: BoxDecoration(
        color: c.layer01,
        border: Border.all(color: c.borderSubtle),
        borderRadius: BorderRadius.circular(TvRadius.small),
      ),
      clipBehavior: Clip.antiAlias,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Semantics(
            button: hasOutput,
            expanded: hasOutput ? _open : null,
            child: Material(
              type: MaterialType.transparency,
              child: InkWell(
                key: const Key('tool-head'),
                onTap: hasOutput ? () => setState(() => _open = !_open) : null,
                hoverColor: c.layer02,
                child: SizedBox(
                  height: 32,
                  child: Padding(
                    padding: const EdgeInsets.symmetric(horizontal: TvSpace.s8),
                    child: _head(c, hasOutput),
                  ),
                ),
              ),
            ),
          ),
          if (_open && hasOutput) _Output(widget.output),
        ],
      ),
    );
  }
}

class _Output extends StatelessWidget {
  const _Output(this.output);

  final String output;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final lines = output.split('\n');
    final shown = lines.length > maxToolLines ? lines.sublist(0, maxToolLines) : lines;
    return Container(
      constraints: const BoxConstraints(maxHeight: 320),
      decoration: BoxDecoration(
        color: c.background,
        border: Border(top: BorderSide(color: c.borderSubtle)),
      ),
      child: SingleChildScrollView(
        padding: const EdgeInsets.symmetric(horizontal: TvSpace.s12, vertical: TvSpace.s8),
        child: SelectionArea(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              for (final line in shown) Text(line, style: TvType.code.copyWith(color: toolLineColor(c, line))),
              if (lines.length > maxToolLines)
                Text('… ${lines.length - maxToolLines} more lines', style: TvType.code.copyWith(color: c.textHelper)),
            ],
          ),
        ),
      ),
    );
  }
}
