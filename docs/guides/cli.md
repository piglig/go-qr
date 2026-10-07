# Command-line tool

`generator` encodes and decodes QR Codes from the shell.

```shell
go install github.com/piglig/go-qr/tools/generator@latest
```

The tool lives in its own module, `github.com/piglig/go-qr/tools`, which is
versioned separately (`tools/v1.x`) and pins a released version of the
library. Options added to the library after that release become CLI flags
in a later tools release.

```
generator <command> [flags] [args]

Commands:
  encode     Encode text or a structured payload into QR image(s)
  decode     Decode a QR code image into text
  version    Print version and exit
  help       Show help
```

## encode

```shell
generator encode [flags] <content>
```

Flags must come before the content: everything after the first positional
argument is encoded. Alternatively, pass the content with `-content`.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-content string` | | Content to encode; a positional argument takes precedence. |
| `-payload string` | | Interpret the content as `key=value` pairs for a payload: `wifi`, `vcard`, `email`, `sms`, `tel`, `geo` or `url` (see below). |
| `-ecc string` | `high` | Error correction: `low`, `medium`, `quartile` or `high`. |
| `-simple` | off | Encode the whole text in one mode instead of switching modes. |
| `-scale int` | 10 | Pixels (PNG) or units (SVG) per module. |
| `-quiet-zone int` | 4 | Margin around the symbol, in modules. |
| `-png string` | | Write a PNG file. |
| `-svg string` | | Write an SVG file. |
| `-stdout string` | | Write `png`, `svg` or `text` to standard output instead of files. |
| `-logo string` | | Logo image (PNG, JPEG or GIF) to draw in the center. |
| `-logo-ratio float` | 0.2 | Logo side as a fraction of the symbol side. |
| `-verify` | off | Decode the generated PNG and fail if it does not match the input. |
| `-preview` | off | Print an ANSI preview to standard error. |
| `-quiet` | off | Suppress non-error output. |

`-stdout` cannot be combined with `-png` or `-svg`. Without any output flag
the command prints a preview.

### Payload keys

Separate pairs with commas; escape a literal comma or equals sign with a
backslash.

| Payload | Keys |
| --- | --- |
| `wifi` | `ssid`, `password`, `auth` (`WPA`, `WEP`, `nopass`), `hidden` (`true`) |
| `vcard` | `name`, `phone`, `email`, `url`, `address`, `org`, `note` |
| `email` | `to`, `subject`, `body` |
| `sms` | `number`, `body` |
| `tel` | `number` |
| `geo` | `lat`, `lon`, `query` |
| `url` | `href` |

## decode

```shell
generator decode <image-file>
```

Reads a PNG, JPEG or GIF and prints the text to standard output.

## Examples

```shell
generator encode hello                                        # preview in the terminal
generator encode -png hello.png -svg hello.svg hello
generator encode -stdout text hello                           # Unicode block text
generator encode -stdout png hello > hello.png
generator encode -payload wifi -png wifi.png "ssid=home,password=s3cret,auth=WPA"
generator encode -logo logo.png -png branded.png -verify "https://example.com"
generator decode hello.png
```
