import 'package:flutter_test/flutter_test.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/models.dart';

import 'support/fake_core/fake_core.dart';

class _FailingCore extends FakeCoreClient {
  @override
  Stream<RunEvent> startRun({
    required String notebookId,
    required String prompt,
    required Map<String, String> vars,
    String? promptName,
    String? model,
  }) async* {
    yield const RunStatus('starting');
    throw StateError('model unavailable');
  }
}

class _EmptyCore extends FakeCoreClient {
  @override
  Future<List<Notebook>> listNotebooks() async => const [];
}

void main() {
  test('demo core lists work types and notebooks', () async {
    final core = FakeCoreClient(tick: Duration.zero);
    expect(await core.listWorkTypes(), hasLength(4));
    expect(await core.listNotebooks(), hasLength(4));
    expect(await core.listOutputs(workType: WorkTypeId.summarize), hasLength(1));
    expect(() => core.setContextEnabled(notebookId: 'missing', contextId: 'c1', enabled: false), throwsStateError);
  });

  test('run failure is reported and running resets', () async {
    final state = AppState(_FailingCore());
    await state.load();
    await state.run('p', const {});
    expect(state.running, isFalse);
    expect(state.status, contains('model unavailable'));
    expect(state.outcome, isNull);
  });

  test('actions without a selection are no-ops', () async {
    final state = AppState(_EmptyCore());
    await state.load();
    expect(state.selected, isNull);
    await state.toggleContext('c1', false);
    await state.run('p', const {});
    expect(state.running, isFalse);
  });

  test('notebook helpers', () {
    const nb = Notebook(
      id: 'n',
      name: 'n',
      workType: WorkTypeId.report,
      context: [
        ContextItem(id: 'a', label: 'a', kind: 'k', enabled: true, estimatedTokens: 10),
        ContextItem(id: 'b', label: 'b', kind: 'k', enabled: false, estimatedTokens: 5),
      ],
    );
    expect(nb.enabledTokens, 10);
    expect(nb.copyWith().context, hasLength(2));
    expect(nb.context.first.copyWith().enabled, isTrue);
  });
}
