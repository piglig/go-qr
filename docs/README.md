# go-qr documentation

Start with the tutorial, look up a task in the guides, and read the
background pages to understand how the library works. The API itself is
documented, with runnable examples, on
[pkg.go.dev](https://pkg.go.dev/github.com/piglig/go-qr/v2).

These pages track the `main` branch. Features added after v2.0 are marked
with the release that introduced them, such as *Since v2.1*; check your
version with `go list -m github.com/piglig/go-qr/v2`.

## Tutorial

- [Getting started](getting-started.md): encode, style, verify, save and
  decode your first code.

## Guides

Using the library:

- [Encoding](guides/encoding.md): error correction, versions, masks, segment
  modes, ECI, GS1 and splitting long data over several symbols.
- [Rendering](guides/rendering.md): PNG, SVG, images and text, colors, quiet
  zone, logos, and checking readability with `Verify`.
- [Styling](guides/styling.md): module and finder shapes, finder colors and
  gradients.
- [Decoding](guides/decoding.md): reading codes from images and photos, the
  result fields, and text encodings.
- [Payloads](guides/payloads.md): Wi-Fi, contacts, calendar events, 2FA, SEPA
  payments and more, and parsing them back.
- [Batch processing](guides/batch.md): generating many codes concurrently.
- [Troubleshooting](guides/troubleshooting.md): codes that do not scan,
  photos that do not decode, and wrong characters.
- [Upgrading to v2](guides/upgrading-to-v2.md): the v1 to v2 API mapping.

Tools built on it:

- [Command-line tool](guides/cli.md): the `generator` CLI.
- [AI assistants](guides/mcp.md): the `go-qr-mcp` server that gives Claude,
  Cursor and other MCP clients decode, generate and inspect tools.

## Reference

- [API](https://pkg.go.dev/github.com/piglig/go-qr/v2) on pkg.go.dev
- [Errors](reference/errors.md): the sentinel errors and when each occurs.
- [Standards support](reference/standards.md): what the encoder and decoder
  implement from ISO/IEC 18004, character sets and payload formats.
- [Changelog](../CHANGELOG.md)

## Background

- [How it works](explanation/how-it-works.md): the encode, render and decode
  pipelines, the source files behind them, and the reasoning behind key
  design decisions.
- [Performance](explanation/performance.md): speed against other libraries,
  decode rates on real photos, and the cost of each feature.
