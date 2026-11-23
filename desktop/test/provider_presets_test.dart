import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/core_client.dart';

void main() {
  testWidgets('a preset fills the provider and its kind', (tester) async {
    tester.view.physicalSize = const Size(1600, 1200);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final core = DemoCoreClient(tick: Duration.zero);
    final state = AppState(core);
    await tester.pumpWidget(TessivoApp(state: state));
    await state.load();
    await tester.pumpAndSettle();
    await tester.tap(find.text('Settings'));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('add-provider')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('provider-preset')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Anthropic (native API)').last);
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(find.byKey(const Key('provider-name'))).controller!.text, 'anthropic');
    await tester.tap(find.byKey(const Key('save-provider')));
    await tester.pumpAndSettle();
    final p = (await core.getSettings()).providers.firstWhere((p) => p.name == 'anthropic');
    expect((p.kind, p.keyRef), ('anthropic', 'ANTHROPIC_KEY'));
    expect(find.text('Anthropic API'), findsOneWidget);

    await tester.tap(find.byKey(const Key('add-provider')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('provider-preset')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Ollama (local)').last);
    await tester.pumpAndSettle();
    expect(
      tester.widget<TextField>(find.byKey(const Key('provider-url'))).controller!.text,
      'http://localhost:11434/v1',
    );
    await tester.tap(find.byKey(const Key('save-provider')));
    await tester.pumpAndSettle();
    final o = (await core.getSettings()).providers.firstWhere((p) => p.name == 'ollama');
    expect((o.kind, o.keyRef), ('openai', null));
  });
}
