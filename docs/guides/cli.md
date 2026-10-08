# Command-line tool

`generator` encodes, styles, verifies and decodes QR Codes from the shell.

```shell
go install github.com/piglig/go-qr/tools/generator@latest
```

The tool lives in its own module, `github.com/piglig/go-qr/tools`, which is
versioned separately (`tools/v1.x`) and pins a released version of the
library. `go version -m $(go env GOPATH)/bin/generator` shows which library
version an installed binary was built with.

```
generator <command> [flags] [args]

Commands:
  encode     Encode text or a structured payload into QR image(s)
  decode     Decode QR code images into text
  version    Print version and exit
  help       Show help
```

## encode

```shell
generator encode [flags] <content>
```

Flags must come before the content: everything after the first positional
argument is encoded. Alternatively, pass the content with `-content`.

### Content

| Flag | Default | Meaning |
| --- | --- | --- |
| `-content string` | | Content to encode; a positional argument takes precedence. |
| `-payload string` | | Interpret the content as `key=value` pairs for a [payload](#payloads). |
| `-gs1` | off | Encode GS1 data, written either as printed under barcodes, `(01)09501101530003(10)ABC123`, or as a raw element string. Separators after variable-length elements are added for you. |
| `-structured` | off | Split content too long for one symbol over up to 16 linked symbols. File outputs get `-1`, `-2`, … before the extension. |

### Encoding

| Flag | Default | Meaning |
| --- | --- | --- |
| `-ecc string` | `high` | Minimum error correction: `low`, `medium`, `quartile` or `high`. |
| `-no-boost` | off | Keep exactly the `-ecc` level instead of raising it into spare capacity. |
| `-min-version int`, `-max-version int` | 1, 40 | Restrict the symbol version. |
| `-mask int` | -1 | Force mask pattern 0–7; -1 picks the lowest penalty. |
| `-simple` | off | Encode the whole text in one mode instead of switching modes. |
| `-utf8-eci` | off | Declare UTF-8 with an ECI designator when the text is not ASCII. |

### Rendering and styling

| Flag | Default | Meaning |
| --- | --- | --- |
| `-scale int` | 10 | Pixels (PNG) or units (SVG) per module. |
| `-quiet-zone int` | 4 | Margin around the symbol, in modules. |
| `-module string` | `square` | Module shape: `square`, `dot` or `rounded`. |
| `-finder string` | `square` | Finder pattern shape: `square`, `rounded` or `circle`. |
| `-fg string`, `-bg string` | black, white | Colors as `#rrggbb` or `#rrggbbaa`; `-bg transparent` is allowed. |
| `-gradient string` | | Gradient over the dark modules: `from,to[,angle]`, for example `'#1a237e,#00695c,45'`. |
| `-finder-color string` | | Finder colors: `ring[,center]`. |
| `-logo string` | | Logo image (PNG, JPEG or GIF) to draw in the center. |
| `-logo-ratio float` | 0.2 | Logo side as a fraction of the symbol side. |

See [Styling](styling.md) for what each style does and how to keep it
readable.

### Output

| Flag | Meaning |
| --- | --- |
| `-png string` | Write a PNG file. |
| `-svg string` | Write an SVG file. |
| `-stdout string` | Write `png`, `svg` or `text` to standard output instead of files. |
| `-verify` | Render, decode and check every symbol, including color contrast, like `Code.Verify`; exit with status 1 if it is not readable. |
| `-preview` | Print an ANSI preview to standard error. |
| `-quiet` | Suppress non-error output. |

`-stdout` cannot be combined with `-png` or `-svg`, and writes a single
image, so `-structured` content that needs several symbols must go to files
(`-stdout text` prints them all). Without any output flag the command prints
a preview.

### Payloads

Separate pairs with commas; escape a literal comma or equals sign with a
backslash. List values (phones, emails, CC and BCC) are separated with `;`.

| Payload | Keys |
| --- | --- |
| `wifi` | `ssid`, `password`, `auth` (`WPA`, `WEP`, `nopass`), `hidden` (`true`) |
| `vcard` | `name`, `phone`, `email`, `url`, `address`, `org`, `note` (MECARD) |
| `contact` | `name`, `given`, `family`, `org`, `title`, `phone`, `email`, `url`, `address`, `note` (vCard 3.0) |
| `event` | `summary`, `location`, `description`, `start`, `end`, `allday` (`true`). Times are RFC 3339, `2006-01-02T15:04` in UTC, or a date `2006-01-02` for all-day events. |
| `otp` | `issuer`, `account`, `secret` (required), `type` (`totp`, `hotp`), `algorithm`, `digits`, `period`, `counter` |
| `epc` | `name`, `iban`, `bic`, `amount` (euros, such as `12.50`), `purpose`, `reference`, `text`, `info`; validated against EPC069-12 |
| `email` | `to`, `subject`, `body`, `cc`, `bcc` |
| `sms` | `number`, `body` |
| `tel` | `number` |
| `geo` | `lat`, `lon`, `query` |
| `url` | `href` |

## decode

```shell
generator decode [-json] <image-file>...
```

Reads PNG, JPEG or GIF images and prints their text. Images that form a
structured append sequence are joined into one message, in any order;
otherwise each text is printed on its own line.

`-json` prints the text together with each symbol's version, error
correction level, mask, segments, structured append position and detected
payload type:

```json
{
  "text": "WIFI:T:WPA;S:home;P:pw;;",
  "symbols": [
    {
      "file": "wifi.png",
      "text": "WIFI:T:WPA;S:home;P:pw;;",
      "version": 3,
      "ecc": "H",
      "mask": 1,
      "payload": "WiFi",
      "segments": [
        { "mode": "alphanumeric", "chars": 10, "eci": -1 },
        { "mode": "byte", "chars": 14, "eci": -1 }
      ]
    }
  ]
}
```

## Examples

```shell
# Basics
generator encode hello                                        # preview in the terminal
generator encode -png hello.png -svg hello.svg hello
generator encode -stdout text hello                           # Unicode block text
generator encode -stdout png hello > hello.png

# Styles, checked before writing
generator encode -module rounded -finder rounded -gradient '#1a237e,#00695c,45' -verify -png styled.png hello
generator encode -module dot -finder circle -finder-color '#c62828,#000' -verify -svg dots.svg hello
generator encode -logo logo.png -verify -png branded.png "https://example.com"

# Payloads
generator encode -payload wifi -png wifi.png "ssid=home,password=s3cret,auth=WPA"
generator encode -payload otp -png 2fa.png "issuer=Example,account=alice@example.com,secret=JBSWY3DPEHPK3PXP"
generator encode -payload event -png event.png "summary=Launch,start=2026-10-07T18:00,end=2026-10-07T21:00"
generator encode -payload epc -ecc medium -png pay.png "name=Red Cross,iban=BE72 0000 0000 1616,amount=25,text=Donation"

# GS1 and long content
generator encode -gs1 -png label.png "(01)09501101530003(17)250101(10)ABC123"
generator encode -structured -max-version 10 -png part.png -content "$(cat long.txt)"
generator decode part-*.png

# Decoding
generator decode hello.png
generator decode -json wifi.png
```
