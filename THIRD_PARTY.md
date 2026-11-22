# Third-party notices

Tessivo is MIT-licensed (see [LICENSE](LICENSE)). It includes or builds on the third-party work below.

## Go core and CLI (`tess`)

The Go module has no third-party module dependencies: it uses only the Go standard library
(BSD-style license, <https://go.dev/LICENSE>), which is compiled into `tess`.

### Zero (ported code)

Parts of `internal/mcp` (the MCP client: JSON-RPC framing, stdio and Streamable HTTP transports) are ported
from Zero, <https://github.com/Twigpine/zero>, commit `8288ae81bd6a`, and reworked for Tessivo.

```
MIT License

Copyright (c) 2026 Gitlawb

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## Desktop app

The desktop app is built with Flutter and Dart packages (listed in `desktop/pubspec.lock`). Flutter collects the
license of the engine, framework and every package into the `NOTICES` file inside each desktop build.

## Keeping this file current

Every new direct dependency or ported code is recorded in
[docs/adr/0003-dependency-policy.md](docs/adr/0003-dependency-policy.md) and, when its license asks for a notice,
here in the same change.
