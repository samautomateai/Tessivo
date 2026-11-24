import 'dart:convert';

import '../design/theme.dart';
import '../design/zoom.dart';

/// An editable copy of the core's settings JSON. It edits the raw maps the
/// core sent, so fields the desktop does not show (headers, timeouts, TLS,
/// retries) go back unchanged on save.
class SettingsDraft {
  SettingsDraft.fromJson(Map<String, Object?> raw)
    : providers = _maps(raw['providers']),
      aliases = _maps(raw['aliases']),
      defaults = _map(raw['defaults']),
      limits = _map(raw['limits']),
      desktop = _map(raw['desktop']),
      agent = _map(raw['agent']);

  final List<Map<String, Object?>> providers;
  final List<Map<String, Object?>> aliases;
  final Map<String, Object?> defaults;
  final Map<String, Object?> limits;

  /// Agent-mode tool rules and limits (`agent` in the config).
  final Map<String, Object?> agent;

  /// The desktop app's own preferences (`desktop` in the config).
  final Map<String, Object?> desktop;

  /// Whether closing the window keeps the app running in the tray.
  bool get closeToTray => desktop['close_to_tray'] == true;

  set closeToTray(bool on) => on ? desktop['close_to_tray'] = true : desktop.remove('close_to_tray');

  static Map<String, Object?> _map(Object? v) =>
      v is Map ? (jsonDecode(jsonEncode(v)) as Map).cast<String, Object?>() : <String, Object?>{};

  static List<Map<String, Object?>> _maps(Object? v) => [
    if (v is List)
      for (final e in v)
        if (e is Map) _map(e),
  ];

  Map<String, Object?>? _provider(String name) => providers.where((p) => p['name'] == name).firstOrNull;

  void setProviderUrl(String name, String url) => _provider(name)?['base_url'] = url;

  void setProviderKeyRef(String name, String ref) {
    final p = _provider(name);
    if (p == null) return;
    if (ref.isEmpty) {
      p.remove('api_key_ref');
    } else {
      p['api_key_ref'] = ref;
    }
  }

  void addProvider(String name, String url, String keyRef, {String kind = 'openai'}) => providers.add({
    'name': name,
    if (kind != 'openai') 'kind': kind,
    'base_url': url,
    if (keyRef.isNotEmpty) 'api_key_ref': keyRef,
  });

  /// Sets an alias's fallback chain from "provider/model" entries.
  void setAliasChain(String name, List<String> chain) {
    final a = aliases.where((a) => a['name'] == name).firstOrNull;
    if (a == null) return;
    a['chain'] = [
      for (final entry in chain)
        if (entry.contains('/'))
          {'provider': entry.substring(0, entry.indexOf('/')), 'model': entry.substring(entry.indexOf('/') + 1)},
    ];
  }

  /// Sets a run default; null or '' removes it (the built-in default applies).
  void setDefault(String key, Object? value) {
    if (value == null || value == '') {
      defaults.remove(key);
    } else {
      defaults[key] = value;
    }
  }

  /// Sets a token limit; null or 0 removes it (no limit).
  void setLimit(String key, int? value) {
    if (value == null || value == 0) {
      limits.remove(key);
    } else {
      limits[key] = value;
    }
  }

  /// The PUT /v1/settings body.
  Map<String, Object?> toBody() => {
    'providers': providers,
    'aliases': aliases,
    'defaults': defaults,
    'limits': limits,
    'desktop': desktop,
    'agent': agent,
  };
}

/// Agent-mode settings in a [SettingsDraft]: limits and default tool rules.
extension AgentDraft on SettingsDraft {
  /// A limit such as max_steps; null when unset (the built-in default applies).
  int? agentLimit(String key) => (agent[key] as num?)?.toInt();

  /// Sets a limit; null or 0 removes it (the built-in default applies).
  void setAgentLimit(String key, int? value) => value == null || value == 0 ? agent.remove(key) : agent[key] = value;

  /// Tool → allow, ask or block.
  Map<String, String> get toolRules => {
    for (final e in (agent['tools'] as Map? ?? const {}).entries)
      if (e.key is String && e.value is String) e.key as String: e.value as String,
  };

  set toolRules(Map<String, String> rules) => rules.isEmpty ? agent.remove('tools') : agent['tools'] = rules;
}

/// The desktop app's appearance in a [SettingsDraft] (`desktop.theme`,
/// `desktop.zoom`, `desktop.layout`). Unknown or out-of-range values read as
/// the defaults, so a hand-edited config never breaks the app.
extension AppearanceDraft on SettingsDraft {
  static const _themes = {
    'system': TvThemeMode.system,
    'light': TvThemeMode.light,
    'dark': TvThemeMode.dark,
    'high-contrast': TvThemeMode.highContrast,
  };

  TvThemeMode get theme => _themes[desktop['theme']] ?? TvThemeMode.system;

  set theme(TvThemeMode mode) {
    if (mode == TvThemeMode.system) {
      desktop.remove('theme');
    } else {
      desktop['theme'] = _themes.entries.firstWhere((e) => e.value == mode).key;
    }
  }

  double get zoom {
    final z = (desktop['zoom'] as num?)?.toDouble() ?? 1;
    return z >= minZoom && z <= maxZoom ? z : 1;
  }

  set zoom(double z) => z == 1 ? desktop.remove('zoom') : desktop['zoom'] = z;

  /// The workbench's panel state (raw; LayoutPrefs reads it).
  Map<String, Object?> get layout => (desktop['layout'] as Map?)?.cast<String, Object?>() ?? const {};

  set layout(Map<String, Object?> l) => l.isEmpty ? desktop.remove('layout') : desktop['layout'] = l;
}
