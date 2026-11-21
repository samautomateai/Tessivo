import 'package:material_ui/material_ui.dart';

import '../theme.dart';

/// Renders the Markdown that models usually produce: headings, paragraphs,
/// bullet and numbered lists, fenced code blocks, tables, quotes and rules,
/// with **bold**, *italic* and `code` inline. Everything else shows as text.
/// No HTML, links or images are interpreted, so model output cannot load
/// anything. Text stays selectable.
class MarkdownView extends StatelessWidget {
  const MarkdownView(this.source, {super.key});

  final String source;

  /// Longest source rendered; the rest is cut so huge replies stay responsive.
  static const maxChars = 200000;

  @override
  Widget build(BuildContext context) {
    final text = source.length > maxChars ? '${source.substring(0, maxChars)}\n\n…' : source;
    final blocks = parseMarkdown(text);
    return SelectionArea(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [for (final b in blocks) _block(context, b)],
      ),
    );
  }

  Widget _block(BuildContext context, MdBlock b) {
    final theme = Theme.of(context).textTheme;
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: switch (b) {
        MdHeading(:final level, :final text) => Text.rich(
          inlineSpans(text),
          style: (level == 1 ? theme.titleLarge : (level == 2 ? theme.titleMedium : theme.titleSmall))?.copyWith(
            fontWeight: FontWeight.w600,
          ),
        ),
        MdParagraph(:final text) => Text.rich(inlineSpans(text)),
        MdQuote(:final text) => Container(
          padding: const EdgeInsets.only(left: 10),
          decoration: const BoxDecoration(
            border: Border(left: BorderSide(color: Tokens.textMuted, width: 3)),
          ),
          child: Text.rich(inlineSpans(text), style: const TextStyle(color: Tokens.textMuted)),
        ),
        MdList(:final ordered, :final items) => Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            for (final (i, item) in items.indexed)
              Padding(
                padding: const EdgeInsets.only(left: 4, bottom: 2),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    SizedBox(width: 22, child: Text(ordered ? '${i + 1}.' : '•')),
                    Expanded(child: Text.rich(inlineSpans(item))),
                  ],
                ),
              ),
          ],
        ),
        MdCode(:final code) => Container(
          width: double.infinity,
          padding: const EdgeInsets.all(10),
          decoration: BoxDecoration(
            color: Theme.of(context).colorScheme.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(6),
          ),
          child: SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            child: Text(code, style: const TextStyle(fontFamily: 'monospace', fontSize: 13)),
          ),
        ),
        MdTable(:final rows) => SingleChildScrollView(scrollDirection: Axis.horizontal, child: _table(context, rows)),
        MdRule() => const Divider(),
      },
    );
  }

  Widget _table(BuildContext context, List<List<String>> rows) {
    final cols = rows.fold(0, (n, r) => r.length > n ? r.length : n);
    final border = BorderSide(color: Theme.of(context).dividerColor);
    return Table(
      defaultColumnWidth: const IntrinsicColumnWidth(),
      border: TableBorder(horizontalInside: border, top: border, bottom: border),
      children: [
        for (final (i, r) in rows.indexed)
          TableRow(
            children: [
              for (var c = 0; c < cols; c++)
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                  child: Text.rich(
                    inlineSpans(c < r.length ? r[c] : ''),
                    style: i == 0 ? const TextStyle(fontWeight: FontWeight.w600) : null,
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
          style: const TextStyle(fontFamily: 'monospace', backgroundColor: Color(0x14000000)),
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
