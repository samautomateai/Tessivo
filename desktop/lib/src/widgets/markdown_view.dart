import 'package:material_ui/material_ui.dart';

import '../design/colors.dart';
import '../design/sizes.dart';
import '../design/type.dart';

/// Renders the Markdown that models usually produce: headings, paragraphs,
/// bullet and numbered lists, fenced code blocks, tables, quotes and rules,
/// with **bold**, *italic* and `code` inline. Everything else shows as text.
/// No HTML, links or images are interpreted, so model output cannot load
/// anything. Text stays selectable. Type comes from TvType: body-long for
/// text, heading-02/01 for headings, code for code.
class MarkdownView extends StatelessWidget {
  const MarkdownView(this.source, {super.key});

  final String source;

  /// Longest source rendered; the rest is cut so huge replies stay responsive.
  static const maxChars = 200000;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final text = source.length > maxChars ? '${source.substring(0, maxChars)}\n\n…' : source;
    final blocks = parseMarkdown(text);
    return SelectionArea(
      child: DefaultTextStyle.merge(
        style: TvType.bodyLong.copyWith(color: c.textPrimary),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [for (final b in blocks) _block(c, b)]),
      ),
    );
  }

  Widget _block(TvColors c, MdBlock b) => Padding(
    padding: EdgeInsets.only(bottom: b is MdHeading ? TvSpace.s8 : TvSpace.s12),
    child: switch (b) {
      MdHeading(:final level, :final text) => Padding(
        padding: const EdgeInsets.only(top: TvSpace.s8),
        child: Text.rich(
          inlineSpans(text),
          style: (level <= 1 ? TvType.heading02 : TvType.heading01).copyWith(color: c.textPrimary),
        ),
      ),
      MdParagraph(:final text) => Text.rich(inlineSpans(text)),
      MdQuote(:final text) => Container(
        padding: const EdgeInsets.only(left: TvSpace.s12),
        decoration: BoxDecoration(
          border: Border(left: BorderSide(color: c.borderStrong, width: 3)),
        ),
        child: Text.rich(inlineSpans(text), style: TextStyle(color: c.textSecondary)),
      ),
      MdList(:final ordered, :final items) => Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          for (final (i, item) in items.indexed)
            Padding(
              padding: const EdgeInsets.only(left: TvSpace.s4, bottom: TvSpace.s2),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                    width: 22,
                    child: Text(ordered ? '${i + 1}.' : '•', style: TextStyle(color: c.textSecondary)),
                  ),
                  Expanded(child: Text.rich(inlineSpans(item))),
                ],
              ),
            ),
        ],
      ),
      MdCode(:final code) => Container(
        width: double.infinity,
        padding: const EdgeInsets.all(TvSpace.s12),
        decoration: BoxDecoration(
          color: c.layer01,
          border: Border.all(color: c.borderSubtle),
          borderRadius: BorderRadius.circular(TvRadius.small),
        ),
        child: SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: Text(code, style: TvType.code.copyWith(color: c.textPrimary)),
        ),
      ),
      MdTable(:final rows) => SingleChildScrollView(scrollDirection: Axis.horizontal, child: _table(c, rows)),
      MdRule() => Divider(color: c.borderSubtle, height: TvSpace.s16),
    },
  );

  /// Looks like the data table (layer head, subtle row lines), but rows grow
  /// with their text: model tables must not lose words to an ellipsis.
  Widget _table(TvColors c, List<List<String>> rows) {
    final cols = rows.fold(0, (n, r) => r.length > n ? r.length : n);
    final line = BorderSide(color: c.borderSubtle);
    return Table(
      defaultColumnWidth: const IntrinsicColumnWidth(),
      border: TableBorder(horizontalInside: line, bottom: line),
      children: [
        for (final (i, r) in rows.indexed)
          TableRow(
            decoration: BoxDecoration(color: i == 0 ? c.layer02 : c.layer01),
            children: [
              for (var col = 0; col < cols; col++)
                ConstrainedBox(
                  constraints: BoxConstraints(minHeight: i == 0 ? TvSize.tableHead : TvSize.tableRow, maxWidth: 360),
                  child: Padding(
                    padding: const EdgeInsets.symmetric(horizontal: TvSpace.s12, vertical: TvSpace.s8),
                    child: Text.rich(
                      inlineSpans(col < r.length ? r[col] : ''),
                      style: i == 0
                          ? TvType.label.copyWith(color: c.textPrimary, fontWeight: FontWeight.w600)
                          : TvType.body.copyWith(color: c.textSecondary),
                    ),
                  ),
                ),
            ],
          ),
      ],
    );
  }
}

/// A parsed Markdown block.
sealed class MdBlock {
  const MdBlock();
}

class MdHeading extends MdBlock {
  const MdHeading(this.level, this.text);
  final int level;
  final String text;
}

class MdParagraph extends MdBlock {
  const MdParagraph(this.text);
  final String text;
}

class MdQuote extends MdBlock {
  const MdQuote(this.text);
  final String text;
}

class MdList extends MdBlock {
  const MdList({required this.ordered, required this.items});
  final bool ordered;
  final List<String> items;
}

class MdCode extends MdBlock {
  const MdCode(this.code);
  final String code;
}

class MdTable extends MdBlock {
  const MdTable(this.rows);
  final List<List<String>> rows; // first row is the header
}

class MdRule extends MdBlock {
  const MdRule();
}

final _heading = RegExp(r'^(#{1,6})\s+(.*)$');
final _bullet = RegExp(r'^\s*[-*+]\s+(.*)$');
final _numbered = RegExp(r'^\s*\d+[.)]\s+(.*)$');
final _rule = RegExp(r'^\s*([-*_])(\s*\1){2,}\s*$');
final _tableSep = RegExp(r'^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$');

/// Splits Markdown into blocks, line by line (linear time).
List<MdBlock> parseMarkdown(String source) {
  final lines = source.replaceAll('\r\n', '\n').split('\n');
  final out = <MdBlock>[];
  final para = <String>[];
  void flush() {
    if (para.isNotEmpty) out.add(MdParagraph(para.join(' ')));
    para.clear();
  }

  for (var i = 0; i < lines.length; i++) {
    final line = lines[i];
    final trimmed = line.trim();
    if (trimmed.startsWith('```')) {
      flush();
      final code = <String>[];
      for (i++; i < lines.length && !lines[i].trim().startsWith('```'); i++) {
        code.add(lines[i]);
      }
      out.add(MdCode(code.join('\n')));
    } else if (trimmed.isEmpty) {
      flush();
    } else if (_heading.firstMatch(trimmed) case final m?) {
      flush();
      out.add(MdHeading(m.group(1)!.length, m.group(2)!.trim()));
    } else if (_rule.hasMatch(trimmed)) {
      flush();
      out.add(const MdRule());
    } else if (trimmed.startsWith('|') && i + 1 < lines.length && _tableSep.hasMatch(lines[i + 1])) {
      flush();
      final rows = [_cells(trimmed)];
      for (i += 2; i < lines.length && lines[i].trim().startsWith('|'); i++) {
        rows.add(_cells(lines[i].trim()));
      }
      i--;
      out.add(MdTable(rows));
    } else if (_bullet.hasMatch(line) || _numbered.hasMatch(line)) {
      flush();
      final ordered = !_bullet.hasMatch(line);
      final items = <String>[];
      for (; i < lines.length; i++) {
        final m = (ordered ? _numbered : _bullet).firstMatch(lines[i]);
        if (m == null) break;
        items.add(m.group(1)!.trim());
      }
      i--;
      out.add(MdList(ordered: ordered, items: items));
    } else if (trimmed.startsWith('>')) {
      flush();
      out.add(MdQuote(trimmed.replaceFirst(RegExp(r'^>\s?'), '')));
    } else {
      para.add(trimmed);
    }
  }
  flush();
  return out;
}

List<String> _cells(String row) {
  var r = row;
  if (r.startsWith('|')) r = r.substring(1);
  if (r.endsWith('|')) r = r.substring(0, r.length - 1);
  return r.split('|').map((c) => c.trim()).toList();
}

final _inline = RegExp(r'(\*\*[^*]+\*\*|`[^`]+`|\*[^*\s][^*]*\*|_[^_\s][^_]*_)');

/// Bold, italic and inline code; other characters are literal.
TextSpan inlineSpans(String text) {
  final spans = <InlineSpan>[];
  var last = 0;
  for (final m in _inline.allMatches(text)) {
    if (m.start > last) spans.add(TextSpan(text: text.substring(last, m.start)));
    final t = m.group(0)!;
    if (t.startsWith('**')) {
      spans.add(
        TextSpan(
          text: t.substring(2, t.length - 2),
          style: const TextStyle(fontWeight: FontWeight.w600),
        ),
      );
    } else if (t.startsWith('`')) {
      spans.add(
        TextSpan(
          text: t.substring(1, t.length - 1),
          style: const TextStyle(fontFamily: TvType.mono),
        ),
      );
    } else {
      spans.add(
        TextSpan(
          text: t.substring(1, t.length - 1),
          style: const TextStyle(fontStyle: FontStyle.italic),
        ),
      );
    }
    last = m.end;
  }
  if (last < text.length) spans.add(TextSpan(text: text.substring(last)));
  return TextSpan(children: spans);
}
