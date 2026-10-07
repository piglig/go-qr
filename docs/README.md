# go-qr documentation

These pages follow the [Diátaxis](https://diataxis.fr/) structure: a
tutorial to start with, guides for specific tasks, reference material to look
things up, and background on how and why the library works the way it does.
The API itself is documented, with runnable examples, on
[pkg.go.dev](https://pkg.go.dev/github.com/piglig/go-qr/v2).

The documentation tracks the `main` branch. Features added after v2.0 are
marked with the release that introduced them, such as *Since v2.1*; check
your version with `go list -m github.com/piglig/go-qr/v2`.

## Tutorial

- [Getting started](getting-started.md): encode, style, verify, save and decode
  your first code.

## Guides

- [Encoding](guides/encoding.md): error correction, versions, masks, segment
  modes, ECI, GS1 and splitting long data over several symbols.
- [Rendering](guides/rendering.md): PNG, SVG, images and text, colors, quiet
  zone, logos, and checking readability with `Verify`.
- [Styling](guides/styling.md): module and finder shapes, finder colors and
  gradients.
- [Decoding](guides/decoding.md): reading codes from images, the result
  fields, and what the decoder supports.
- [Payloads](guides/payloads.md): Wi-Fi, contacts, calendar events, 2FA, SEPA
  payments and more, and parsing them back.
- [Batch processing](guides/batch.md): generating many codes concurrently.
- [Command-line tool](guides/cli.md): the `generator` CLI.
- [Upgrading to v2](upgrading-to-v2.md): v1 to v2 API mapping.

## Reference

- [API reference](https://pkg.go.dev/github.com/piglig/go-qr/v2) on pkg.go.dev
- [Errors](reference/errors.md): the sentinel errors and when each occurs.
- [Standards support](reference/standards.md): what the encoder and decoder
  implement from ISO/IEC 18004 and related specifications.
- [Changelog](../CHANGELOG.md)

## Background

- [How it works](explanation/how-it-works.md): the encode, render and decode
  pipelines, and the reasoning behind key design decisions.
- [Performance](performance.md): benchmarks against other Go libraries, and
  the cost of each feature.
- [Troubleshooting](troubleshooting.md): codes that do not scan, and other
  common problems.
- [Decoder design](design/decoder.md): the original design proposal for the
  native decoder.
