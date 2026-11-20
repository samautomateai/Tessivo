# ADR 0004: Local JSON API over a Unix socket instead of gRPC

- Status: accepted (2026-11-20). Amends ADR 0001 (desktop ↔ core transport).

## Context

The desktop app and the Go core always run on the same machine. gRPC would add grpc-go,
protobuf, grpc-dart and a protoc/buf toolchain to the build. ADR 0003 says to depend on a
library only when building the capability ourselves makes no sense; a local request/response
API with streamed run events is small and fully covered by both standard libraries.

## Decision

- The core serves a versioned JSON API (`/v1/...`) with `net/http`; runs stream as
  server-sent events (`status`, `delta`, then `finished` or `error`).
- Transport: a Unix domain socket (0600, in a 0700 directory) on macOS and Linux; a loopback
  TCP port on Windows. Non-loopback addresses are refused.
- Every request needs a per-session 256-bit bearer token, compared in constant time. The token
  and address are written to an `endpoint.json` file (0600) that the desktop app reads; it is
  removed on shutdown and never passed in argv.
- Requests from browsers (`Origin` / `Sec-Fetch-*`) and non-loopback `Host` headers are refused,
  blocking CSRF and DNS-rebinding attacks on the loopback port.
- Bodies are capped at 1 MiB and decoded strictly; at most 4 runs execute at once.
- The Flutter client uses `dart:io` `HttpClient` (Unix sockets via `connectionFactory`).
- The contract is documented in `docs/api.md`; the service layer (`internal/service`) is
  transport-neutral, so gRPC can be added later without touching business logic.

## Consequences

No generated code and no new dependencies on either side. We lose protobuf's typed codegen and
must keep `docs/api.md`, the Go handlers and the Dart client in step; handler tests and Dart
client tests guard that.
