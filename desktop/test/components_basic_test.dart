import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/core/models.dart';
import 'package:tessivo_desktop/src/design/colors.dart';
import 'package:tessivo_desktop/src/design/components/buttons.dart';
import 'package:tessivo_desktop/src/design/components/empty_state.dart';
import 'package:tessivo_desktop/src/design/components/headers.dart';
import 'package:tessivo_desktop/src/design/components/notices.dart';
import 'package:tessivo_desktop/src/design/components/rows.dart';
import 'package:tessivo_desktop/src/design/components/tabs.dart';
import 'package:tessivo_desktop/src/design/components/tags.dart';
import 'package:tessivo_desktop/src/design/icons.dart';
import 'package:tessivo_desktop/src/design/theme.dart';

Future<void> pumpIn(WidgetTester tester, Widget child, {TvColors colors = TvColors.dark}) => tester.pumpWidget(
  MaterialApp(
    theme: tvTheme(colors),
    home: Scaffold(body: Center(child: child)),
  ),
);

void main() {
  testWidgets('buttons have the spec heights and the primary colour', (tester) async {
    await pumpIn(
      tester,
      Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          TvButton('Run', key: const Key('p'), icon: TvIcon.run, kind: TvButtonKind.primary, onPressed: () {}),
          TvButton('Dense', key: const Key('d'), dense: true, onPressed: () {}),
        ],
      ),
    );
    expect(tester.getSize(find.byKey(const Key('p'))).height, 28);
    expect(tester.getSize(find.byKey(const Key('d'))).height, 24);
    final style = tester
        .widget<TextButton>(find.descendant(of: find.byKey(const Key('p')), matching: find.byType(TextButton)))
        .style!;
    expect(style.backgroundColor!.resolve({}), TvColors.dark.buttonPrimary);
    expect(find.byType(TvIconView), findsOneWidget);
  });

  testWidgets('icon buttons expose their tooltip as a label and show a badge', (tester) async {
    var taps = 0;
    await pumpIn(
      tester,
      TvIconButton(TvIcon.approvals, tooltip: 'Approvals waiting', badge: 3, onPressed: () => taps++),
    );
    expect(find.bySemanticsLabel(RegExp('Approvals waiting')), findsOneWidget);
    expect(find.text('3'), findsOneWidget);
    await tester.tap(find.byType(TvIconButton));
    expect(taps, 1);
    expect(tester.getSize(find.byType(TvIconButton)), const Size(28, 28));
  });

  testWidgets('badge hides at 0 and caps at 99+', (tester) async {
    await pumpIn(tester, const Row(mainAxisSize: MainAxisSize.min, children: [TvBadge(0), TvBadge(120)]));
    expect(find.text('0'), findsNothing);
    expect(find.text('99+'), findsOneWidget);
  });

  testWidgets('a long tree row truncates with a tooltip instead of overflowing', (tester) async {
    final long = 'Quarterly vendor contract renewal review ' * 5;
    await pumpIn(
      tester,
      SizedBox(
        width: 236,
        child: TvTreeRow(leading: const TvStatusDot(Color(0xFFFF0000)), label: long, trailing: 'Mon', selected: true),
      ),
    );
    expect(tester.takeException(), isNull);
    expect(tester.getSize(find.byType(TvTreeRow)).height, 26);
    expect(find.byWidgetPredicate((w) => w is Tooltip && w.message == long), findsOneWidget);
  });

  testWidgets('tabs show counts, mark the selected tab and report taps', (tester) async {
    int? picked;
    await pumpIn(
      tester,
      TvTabs(
        tabs: const [TvTabItem('Output'), TvTabItem('Sources', count: 3), TvTabItem('Memory', count: 0)],
        selected: 0,
        onSelect: (i) => picked = i,
      ),
    );
    expect(find.text('Sources · 3'), findsOneWidget);
    expect(find.text('Memory'), findsOneWidget);
    expect(tester.getSize(find.byType(TvTabs)).height, 36);
    await tester.tap(find.text('Sources · 3'));
    expect(picked, 1);
    expect(tester.getSemantics(find.text('Output')), isSemantics(isSelected: true, isButton: true));
  });

  testWidgets('headers and toolbar use the spec heights', (tester) async {
    await pumpIn(
      tester,
      const Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          TvPanelHeader('Thread'),
          TvToolbar(leading: Text('v2')),
        ],
      ),
    );
    expect(tester.getSize(find.byType(TvPanelHeader)).height, 36);
    expect(tester.getSize(find.byType(TvToolbar)).height, 38);
    expect(find.text('THREAD'), findsOneWidget);
  });

  testWidgets('every component renders in all three themes', (tester) async {
    for (final c in [TvColors.dark, TvColors.light, TvColors.highContrast]) {
      await pumpIn(
        tester,
        SizedBox(
          width: 900,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TvPageHeader(
                title: 'Change risk review',
                tag: const WorkTypeTag(WorkTypeId.analyze),
                meta: '≈ 51k tokens · smart',
                actions: [TvButton('Run', kind: TvButtonKind.primary, onPressed: () {})],
              ),
              const TvTag('MCP'),
              const TvTag('exit 1', tone: TvTagTone.red),
              const TvInlineNotice(kind: TvNoticeKind.warning, title: 'Reconnecting', message: 'The core went away.'),
              TvEmptyState(
                icon: TvIcon.notebooks,
                title: 'No notebooks yet',
                message: 'Make one.',
                actionLabel: 'New notebook',
                onAction: () {},
              ),
            ],
          ),
        ),
        colors: c,
      );
      expect(tester.takeException(), isNull);
    }
    expect(find.text('Analyze'), findsOneWidget);
    expect(find.text('New notebook'), findsOneWidget);
  });

  testWidgets('toast shows the message', (tester) async {
    await pumpIn(
      tester,
      Builder(builder: (context) => TvButton('Go', onPressed: () => showTvToast(context, 'Saved as a new version'))),
    );
    await tester.tap(find.text('Go'));
    await tester.pump();
    expect(find.text('Saved as a new version'), findsOneWidget);
  });

  testWidgets('a row of icon buttons meets the labelled tap target guideline', (tester) async {
    final handle = tester.ensureSemantics();
    await pumpIn(
      tester,
      Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          TvIconButton(TvIcon.runAgain, tooltip: 'Run again', onPressed: () {}),
          TvIconButton(TvIcon.export, tooltip: 'Export', onPressed: () {}),
        ],
      ),
    );
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    handle.dispose();
  });
}
