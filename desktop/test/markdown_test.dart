import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/widgets/markdown_view.dart';

void main() {
  test('parses the blocks models produce', () {
    final blocks = parseMarkdown(
      '# Title\n\nSome **bold** text\nwrapped.\n\n- one\n- two\n\n1. first\n2) second\n\n'
      '```go\nfmt.Println("x")\n```\n\n| A | B |\n|---|:-:|\n| 1 | 2 |\n| 3 |\n\n> quoted\n\n---\n## Sub',
    );
    expect(blocks.map((b) => b.runtimeType.toString()), [
      'MdHeading',
      'MdParagraph',
      'MdList',
      'MdList',
      'MdCode',
      'MdTable',
      'MdQuote',
      'MdRule',
      'MdHeading',
    ]);
    expect((blocks[1] as MdParagraph).text, 'Some **bold** text wrapped.');
    expect((blocks[2] as MdList).items, ['one', 'two']);
    expect((blocks[3] as MdList).ordered, isTrue);
    expect((blocks[4] as MdCode).code, 'fmt.Println("x")');
    expect((blocks[5] as MdTable).rows, [
      ['A', 'B'],
      ['1', '2'],
      ['3'],
    ]);
    expect((blocks[8] as MdHeading).level, 2);
  });

  test('unclosed code fences and lone pipes are safe', () {
    expect(parseMarkdown('```\ncode'), hasLength(1));
    expect(parseMarkdown('| not a table'), everyElement(isA<MdParagraph>()));
    expect(parseMarkdown(''), isEmpty);
  });

  test('inline spans', () {
    final span = inlineSpans('a **b** `c` *d* _e_ f');
    final parts = span.children!.cast<TextSpan>();
    expect(parts.map((p) => p.text), ['a ', 'b', ' ', 'c', ' ', 'd', ' ', 'e', ' f']);
    expect(parts[1].style?.fontWeight, FontWeight.w600);
    expect(parts[3].style?.fontFamily, 'monospace');
    expect(parts[5].style?.fontStyle, FontStyle.italic);
  });

  testWidgets('renders every block and cuts huge input', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: SingleChildScrollView(
            child: MarkdownView('# T\n\n- a\n\n1. b\n\n```\nc\n```\n\n| x |\n|---|\n| y |\n\n> q\n\n***\n\np'),
          ),
        ),
      ),
    );
    expect(find.text('c'), findsOneWidget);
    expect(find.byType(Table), findsOneWidget);
    expect(find.byType(Divider), findsOneWidget);
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: SingleChildScrollView(child: MarkdownView('x' * (MarkdownView.maxChars + 10)))),
      ),
    );
    expect(find.textContaining('…'), findsOneWidget);
  });
}
