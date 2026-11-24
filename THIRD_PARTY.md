# Third-party notices

Tessivo is MIT-licensed (see [LICENSE](LICENSE)). It includes or builds on the third-party work below.

## Go core and CLI (`tess`)

`tess` is built from the Go standard library (BSD-style, <https://go.dev/LICENSE>) and, for reading PDFs, these modules (their license texts ship in the Go module cache and at the links below):

| Module | License |
| --- | --- |
| github.com/klippa-app/go-pdfium | MIT, <https://github.com/klippa-app/go-pdfium/blob/main/LICENSE> |
| pdfium (embedded as WebAssembly) | BSD-3-Clause and Apache-2.0, <https://pdfium.googlesource.com/pdfium/+/main/LICENSE> |
| github.com/tetratelabs/wazero | Apache-2.0, <https://github.com/tetratelabs/wazero/blob/main/LICENSE> |
| github.com/jolestar/go-commons-pool/v2 | Apache-2.0, <https://github.com/jolestar/go-commons-pool/blob/master/LICENSE> |
| github.com/google/uuid | BSD-3-Clause, <https://github.com/google/uuid/blob/master/LICENSE> |
| golang.org/x/sys, golang.org/x/text | BSD-3-Clause, <https://go.dev/LICENSE> |

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

Direct third-party packages beyond the Flutter SDK and `material_ui`:

| Package | License |
| --- | --- |
| tray_manager, window_manager, menu_base, screen_retriever (leanflutter) | MIT, <https://github.com/leanflutter/tray_manager/blob/main/LICENSE> |
| shortid | BSD-3-Clause, <https://github.com/0xff00ff/shortid> |
| Carbon icons 11.53.0 (`@carbon/icons`, SVGs in `desktop/assets/icons/carbon/`) | Apache-2.0, <https://github.com/carbon-design-system/carbon/blob/main/LICENSE> (copy in that folder) |
| flutter_svg (and vector_graphics, path_parsing, xml, petitparser) | MIT and BSD-3-Clause, <https://pub.dev/packages/flutter_svg/license> |
| IBM Plex Sans 1.1.0 and Plex Mono 2.5.0 (fonts, bundled in `desktop/assets/fonts/`) | SIL Open Font License 1.1, <https://github.com/IBM/plex/blob/master/LICENSE.txt> (copy in `OFL.txt`) |

On Linux the tray uses the system library libayatana-appindicator3 (LGPL-2.1/3), linked dynamically and not shipped.

## Keeping this file current

Every new direct dependency or ported code is recorded in
[docs/adr/0003-dependency-policy.md](docs/adr/0003-dependency-policy.md) and, when its license asks for a notice,
here in the same change.
