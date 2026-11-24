part of 'app_state.dart';

/// Theme and zoom: read from the config, changed from Settings → Appearance,
/// the palette and the zoom shortcuts, and saved back at once.
extension AppearanceActions on AppState {
  /// Reads `desktop.theme` and `desktop.zoom`; bad or missing values give the defaults.
  Future<void> loadAppearance() async {
    try {
      final d = SettingsDraft.fromJson((await core.getSettings()).raw);
      themeMode = d.theme;
      layout = LayoutPrefs.fromJson(d.layout);
      defaultModel = d.defaults['model'] as String?;
      zoom = d.zoom;
    } on Object {
      return; // an older core or no core: keep the defaults
    }
  }

  Future<void> setThemeMode(TvThemeMode mode) async {
    themeMode = mode;
    _changed();
    await _saveAppearance((d) => d.theme = mode);
  }

  Future<void> setZoom(double factor) async {
    zoom = factor.clamp(minZoom, maxZoom);
    _changed();
    await _saveAppearance((d) => d.zoom = zoom);
  }

  /// Zooms in (+1), out (−1) or back to 100 % (0).
  Future<void> stepZoom(int direction) => setZoom(nextZoom(zoom, direction));

  Future<void> _saveAppearance(void Function(SettingsDraft) edit) async {
    try {
      final d = SettingsDraft.fromJson((await core.getSettings()).raw);
      edit(d);
      await core.updateSettings(d.toBody());
    } on Object {
      _notice('Could not save the appearance setting');
    }
  }
}
