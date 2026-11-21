import 'dart:io';

/// Flutter's widget-test binding replaces [HttpClient] with a stub that
/// answers every request with 400. HTTP client tests talk to real loopback
/// fake servers, so they need a real client.
HttpClient realHttpClient() => HttpOverrides.runWithHttpOverrides(HttpClient.new, _Real());

class _Real extends HttpOverrides {}
