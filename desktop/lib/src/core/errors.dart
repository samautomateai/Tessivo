/// An error returned by the core.
class CoreException implements Exception {
  const CoreException(this.status, this.message);
  final int status;
  final String message;

  @override
  String toString() => 'CoreException($status): $message';
}
