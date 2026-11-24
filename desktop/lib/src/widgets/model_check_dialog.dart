import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../design/colors.dart';
import '../design/components/modal.dart';
import '../design/icons.dart';
import '../design/sizes.dart';
import '../design/type.dart';

/// Shows whether each model behind an alias answers one tiny request.
Future<void> showModelCheck(BuildContext context, String model, Future<List<ModelCheck>> Function(String) test) =>
    showTvModal<void>(
      context,
      title: 'Test "$model"',
      width: 520,
      body: ModelCheckResults(checks: test(model)),
      actions: const [TvModalAction('Close', value: null)],
    );

/// One row per model in the chain: answers (and how fast) or the error.
class ModelCheckResults extends StatelessWidget {
  const ModelCheckResults({super.key, required this.checks});

  final Future<List<ModelCheck>> checks;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return FutureBuilder<List<ModelCheck>>(
      future: checks,
      builder: (context, snap) {
        if (snap.hasError) {
          final e = snap.error;
          return Text(
            e is CoreException ? e.message : '$e',
            key: const Key('model-check-error'),
            style: TvType.body.copyWith(color: c.error),
          );
        }
        if (!snap.hasData) {
          return const Row(
            children: [
              SizedBox.square(dimension: 16, child: CircularProgressIndicator(strokeWidth: 2)),
              SizedBox(width: TvSpace.s12),
              Flexible(child: Text('Sending one short request to each model…')),
            ],
          );
        }
        return Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            for (final m in snap.data!)
              Padding(
                key: Key('model-check-${m.model}'),
                padding: const EdgeInsets.symmetric(vertical: TvSpace.s4),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    TvIconView(
                      m.ok ? TvIcon.success : TvIcon.error,
                      color: m.ok ? c.success : c.error,
                      semanticLabel: m.ok ? 'answers' : 'failed',
                    ),
                    const SizedBox(width: TvSpace.s8),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(m.model, style: TvType.body.copyWith(color: c.textPrimary)),
                          Text(
                            m.ok ? 'Answers in ${m.millis} ms' : m.error,
                            style: TvType.helper.copyWith(color: c.textHelper),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
          ],
        );
      },
    );
  }
}
