/// A notebook's saved prompt with its own model settings (docs/api.md, prompts).
class SavedPrompt {
  const SavedPrompt({required this.name, required this.text, this.model = '', this.temperature, this.maxOutputTokens});

  factory SavedPrompt.fromJson(Map<String, Object?> j) {
    final s = j['settings'] is Map<String, Object?>
        ? j['settings']! as Map<String, Object?>
        : const <String, Object?>{};
    return SavedPrompt(
      name: j['name'] as String? ?? '',
      text: j['text'] as String? ?? '',
      model: s['model'] as String? ?? '',
      temperature: (s['temperature'] as num?)?.toDouble(),
      maxOutputTokens: (s['max_output_tokens'] as num?)?.toInt(),
    );
  }

  final String name;
  final String text;
  final String model;
  final double? temperature;
  final int? maxOutputTokens;

  /// The PUT body: the text and only the settings that are set.
  Map<String, Object?> toBody() {
    final settings = <String, Object?>{
      if (model.isNotEmpty) 'model': model,
      'temperature': ?temperature,
      'max_output_tokens': ?maxOutputTokens,
    };
    return {'text': text, if (settings.isNotEmpty) 'settings': settings};
  }
}
