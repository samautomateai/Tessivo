import 'package:flutter_svg/flutter_svg.dart';
import 'package:material_ui/material_ui.dart';

import 'sizes.dart';

/// What an icon means; screens name meanings, never files. Carbon icons
/// (`@carbon/icons` 11.53.0, Apache-2.0) vendored under assets/icons/carbon.
enum TvIcon {
  notebooks('notebook'),
  research('microscope'),
  chat('chat'),
  schedules('time'),
  templates('template'),
  search('search'),
  approvals('task--approved'),
  notifications('notification'),
  settings('settings'),
  account('user--avatar'),
  add('add'),
  subtract('subtract'),
  close('close'),
  run('play'),
  runAgain('renew'),
  edit('edit'),
  compare('compare'),
  export('download'),
  newVersion('version'),
  agent('bot'),
  tools('tool-kit'),
  terminal('terminal'),
  document('document'),
  folder('folder'),
  link('link'),
  attach('attachment'),
  send('send--alt'),
  panelOpen('side-panel--open'),
  panelClose('side-panel--close'),
  expand('chevron--right'),
  success('checkmark--filled'),
  warning('warning--alt'),
  error('error--filled');

  const TvIcon(this._file);
  final String _file;

  String get asset => 'assets/icons/carbon/$_file.svg';
}

/// A Carbon icon tinted with [color] (default: the surrounding text colour).
/// Give [semanticLabel] when the icon is the only thing saying what it means.
class TvIconView extends StatelessWidget {
  const TvIconView(this.icon, {super.key, this.size = TvSize.icon, this.color, this.semanticLabel});

  final TvIcon icon;
  final double size;
  final Color? color;
  final String? semanticLabel;

  @override
  Widget build(BuildContext context) {
    final tint = color ?? DefaultTextStyle.of(context).style.color ?? IconTheme.of(context).color!;
    return SvgPicture.asset(
      icon.asset,
      width: size,
      height: size,
      colorFilter: ColorFilter.mode(tint, BlendMode.srcIn),
      semanticsLabel: semanticLabel,
      excludeFromSemantics: semanticLabel == null,
    );
  }
}
