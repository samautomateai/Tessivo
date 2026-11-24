import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/design/colors.dart';
import 'package:tessivo_desktop/src/design/components/approval.dart';
import 'package:tessivo_desktop/src/design/components/buttons.dart';
import 'package:tessivo_desktop/src/design/components/fields.dart';
import 'package:tessivo_desktop/src/design/components/modal.dart';
import 'package:tessivo_desktop/src/design/components/table.dart';
import 'package:tessivo_desktop/src/design/components/tags.dart';
import 'package:tessivo_desktop/src/design/components/tool_call.dart';
import 'package:tessivo_desktop/src/design/icons.dart';
import 'package:tessivo_desktop/src/design/theme.dart';

Future<void> pumpIn(WidgetTester tester, Widget child) => tester.pumpWidget(
  MaterialApp(
    theme: tvTheme(TvColors.dark),
    home: Scaffold(
      body: Center(child: SizedBox(width: 760, child: child)),
    ),
  ),
);

void main() {
  testWidgets('a field shows its label above and its error below', (tester) async {
    await pumpIn(tester, const TvTextField(label: 'Program', error: 'Enter a program', key: Key('f')));
    expect(find.text('Program'), findsOneWidget);
    final error = tester.widget<Text>(find.text('Enter a program'));
    expect(error.style!.color, TvColors.dark.error);
    expect(tester.getTopLeft(find.text('Program')).dy, lessThan(tester.getTopLeft(find.byType(TextField)).dy));
  });

  testWidgets('dropdown, toggle and checkbox report changes', (tester) async {
    String? picked;
    bool? toggled;
    bool? checked;
    await pumpIn(
      tester,
      Column(
        children: [
          TvDropdown<String>(
            key: const Key('dd'),
            label: 'Theme',
            items: const [('light', 'Light'), ('dark', 'Dark')],
            value: 'light',
            onChanged: (v) => picked = v,
          ),
          TvToggle(
            key: const Key('tg'),
            label: 'Keep running in the tray',
            value: false,
            onChanged: (v) => toggled = v,
          ),
          TvCheckbox(key: const Key('cb'), label: 'Include', value: false, onChanged: (v) => checked = v),
        ],
      ),
    );
    await tester.tap(find.byKey(const Key('dd')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Dark').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Keep running in the tray'));
    await tester.tap(find.text('Include'));
    expect((picked, toggled, checked), ('dark', true, true));
  });

  testWidgets('a modal closes on Esc and returns the chosen action', (tester) async {
    Future<String?>? result;
    await pumpIn(
      tester,
      Builder(
        builder: (context) => TvButton(
          'Open',
          onPressed: () => result = showTvModal<String>(
            context,
            title: 'Delete notebook?',
            body: const Text('This cannot be undone.'),
            actions: [
              const TvModalAction('Cancel', value: 'cancel'),
              const TvModalAction('Delete', value: 'delete', kind: TvButtonKind.danger),
            ],
          ),
        ),
      ),
    );
    await tester.tap(find.text('Open'));
    await tester.pumpAndSettle();
    expect(find.text('Delete notebook?'), findsOneWidget);
    await tester.tap(find.text('Delete'));
    await tester.pumpAndSettle();
    expect(await result, 'delete');
    await tester.tap(find.text('Open'));
    await tester.pumpAndSettle();
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    expect(find.text('Delete notebook?'), findsNothing);
    expect(await result, isNull);
  });

  testWidgets('a data table has a 32 px head and 36 px rows', (tester) async {
    await pumpIn(
      tester,
      const TvDataTable(
        columns: ['Severity', 'Finding'],
        rows: [
          [Text('High'), Text('Token expiry check removed')],
          [Text('Low'), Text('Log line includes order id')],
        ],
      ),
    );
    expect(tester.getSize(find.byKey(const Key('table-head'))).height, 32);
    expect(tester.getSize(find.byKey(const Key('table-row-0'))).height, 36);
    expect(find.text('Token expiry check removed'), findsOneWidget);
  });

  testWidgets('a tool call is one row that expands to its output', (tester) async {
    await pumpIn(
      tester,
      const ToolCallBlock(
        icon: TvIcon.terminal,
        tool: 'go test ./pay/...',
        tags: [
          TvTag('allow-listed', tone: TvTagTone.green),
          TvTag('exit 1', tone: TvTagTone.red),
        ],
        duration: '4.2 s',
        output: '--- FAIL: TestChargeRetries (0.31s)\nok  \tpayments/auth\t0.208s',
      ),
    );
    expect(tester.getSize(find.byKey(const Key('tool-head'))).height, 32);
    expect(find.textContaining('TestChargeRetries'), findsNothing);
    await tester.tap(find.byKey(const Key('tool-head')));
    await tester.pumpAndSettle();
    final fail = tester.widget<Text>(find.textContaining('--- FAIL'));
    expect(fail.style!.color, TvColors.dark.error);
    final ok = tester.widget<Text>(find.textContaining('payments/auth'));
    expect(ok.style!.color, TvColors.dark.success);
  });

  testWidgets('an approval card offers allow once, always and deny', (tester) async {
    final calls = <String>[];
    await pumpIn(
      tester,
      ApprovalCard(
        tool: 'github · create_comment',
        args: '{ "pr": 482 }',
        onAllow: () => calls.add('allow'),
        onAlways: () => calls.add('always'),
        onDeny: () => calls.add('deny'),
      ),
    );
    expect(find.text('Allow github · create_comment?'), findsOneWidget);
    for (final label in ['Allow once', 'Always in this chat', 'Deny']) {
      await tester.tap(find.text(label));
    }
    expect(calls, ['allow', 'always', 'deny']);
  });
}
