import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../design/colors.dart';
import '../widgets/command_palette.dart';
import 'activity_bar.dart';
import 'banners.dart';
import 'header.dart';
import 'sections.dart';
import 'side_bar.dart';
import 'status_bar.dart';

export 'layout_prefs.dart' show panelMinWindow, sideBarMinWindow;

/// The window: Carbon header, then the VS Code workbench (activity bar, side
/// bar, page, secondary panel) and the status bar.
class Workbench extends StatelessWidget {
  const Workbench({super.key, required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: state,
    builder: (context, _) {
      final c = TvColors.of(context);
      if (state.loading) {
        return Scaffold(
          backgroundColor: c.background,
          body: const Center(child: CircularProgressIndicator()),
        );
      }
      return Scaffold(
        backgroundColor: c.background,
        body: AppShortcuts(
          state: state,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TvHeader(state: state),
              Expanded(
                child: LayoutBuilder(
                  builder: (context, box) {
                    final side = state.sideBarShown(box.maxWidth) ? sideBarFor(state) : null;
                    final panel = state.panelShown(box.maxWidth) ? panelFor(state) : null;
                    return Row(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        ActivityBar(state: state),
                        if (side != null)
                          SideBarFrame(
                            width: state.layout.sideBarWidth,
                            onResize: (w) => unawaited(state.setSideBarWidth(w)),
                            onResized: (w) => unawaited(state.setSideBarWidth(w, save: true)),
                            child: side,
                          ),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.stretch,
                            children: [
                              WorkbenchBanners(state: state),
                              Expanded(child: pageFor(state)),
                            ],
                          ),
                        ),
                        if (panel != null)
                          Container(
                            key: const Key('secondary-panel'),
                            width: state.layout.panelWidth,
                            decoration: BoxDecoration(
                              color: c.layer01,
                              border: Border(left: BorderSide(color: c.borderSubtle)),
                            ),
                            child: panel,
                          ),
                      ],
                    );
                  },
                ),
              ),
              StatusBar(state: state),
            ],
          ),
        ),
      );
    },
  );
}
