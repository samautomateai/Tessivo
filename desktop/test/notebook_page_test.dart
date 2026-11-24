import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/design/components/buttons.dart';

import 'support/fake_core/fake_core.dart';

Future<AppState> pumpPage(WidgetTester tester, {double width = 1600}) async {
  tester.view.physicalSize = Size(width, 1100);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final state = AppState(FakeCoreClient(tick: Duration.zero));
  await tester.pumpWidget(TessivoApp(state: state));
  await state.load();
  await tester.pumpAndSettle();
  return state;
}

void main() {
  testWidgets('the latest output and its thread open with the notebook', (tester) async {
    final state = await pumpPage(tester);
    expect(find.byKey(const Key('outputs-list')), findsOneWidget);
    expect(find.byKey(const Key('secondary-panel')), findsOneWidget);
    expect(state.selectedOutputId, 'o1');
    expect(find.byKey(const Key('output-thread')), findsOneWidget);
    expect(find.text('Why is the auth risk rated high?'), findsOneWidget);
    expect(find.text('Updates prompt review, used by 1 schedule'), findsOneWidget);
  });

  testWidgets('asking about the output streams a reply into the thread', (tester) async {
    await pumpPage(tester);
    await tester.enterText(find.byKey(const Key('thread-input')), 'Which file?');
    await tester.tap(find.byKey(const Key('thread-send')));
    await tester.pumpAndSettle();
    expect(find.text('Which file?'), findsOneWidget);
    expect(find.textContaining('answer points back to the sources'), findsOneWidget);
  });

  testWidgets('apply and dismiss resolve suggestions', (tester) async {
    await pumpPage(tester);
    await tester.tap(find.byKey(const Key('suggestion-apply-sg1')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('suggestion-apply-sg1')), findsNothing);
    await tester.tap(find.byKey(const Key('suggest-improvements')));
    await tester.pumpAndSettle();
    final dismiss = find.byWidgetPredicate(
      (w) => w.key is ValueKey<String> && (w.key! as ValueKey<String>).value.startsWith('suggestion-dismiss-'),
    );
    expect(dismiss, findsOneWidget);
    await tester.tap(dismiss);
    await tester.pumpAndSettle();
    expect(dismiss, findsNothing);
  });

  testWidgets('new version appears in the outputs pane and opens', (tester) async {
    final state = await pumpPage(tester);
    await tester.tap(find.byKey(const Key('new-version')));
    await tester.pumpAndSettle();
    expect(state.selectedOutputId, 'o1-v2');
    expect(find.textContaining('v2 ← v1'), findsOneWidget);
    await tester.tap(find.byKey(const Key('output-o1')));
    await tester.pumpAndSettle();
    expect(state.selectedOutputId, 'o1');
    expect(find.textContaining('Saved as v2'), findsOneWidget);
  });

  testWidgets('a narrow window hides the thread; its toggle opens it for the session', (tester) async {
    final state = await pumpPage(tester, width: 1000);
    expect(find.byKey(const Key('secondary-panel')), findsNothing);
    final toggle = find.byKey(const Key('toggle-thread'));
    expect(tester.widget<TvIconButton>(toggle).tooltip, 'Show thread (Ctrl J)');
    await tester.tap(toggle);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('secondary-panel')), findsOneWidget);
    expect(find.byKey(const Key('output-thread')), findsOneWidget);
    expect(state.layout.panelOpen, isTrue, reason: 'the remembered layout is unchanged');
    await tester.tap(toggle);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('secondary-panel')), findsNothing);
  });

  testWidgets('a notebook without outputs explains what to do', (tester) async {
    final state = await pumpPage(tester);
    state.select('nb-3');
    await tester.pumpAndSettle();
    expect(find.textContaining('Run the prompt to get the first output'), findsOneWidget);
    expect(find.byKey(const Key('output-thread')), findsNothing);
  });
}
