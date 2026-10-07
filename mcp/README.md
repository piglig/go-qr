# go-qr-mcp

A [Model Context Protocol](https://modelcontextprotocol.io) server that gives
AI assistants exact QR Code tools, built on
[go-qr](https://github.com/piglig/go-qr):

- `decode_qr` decodes the code in a local image and explains what scanning it
  would do, with risk signals such as lookalike domains, open Wi-Fi or
  embedded 2FA secrets;
- `generate_qr` creates plain, payload (Wi-Fi, contact, event, 2FA, SEPA
  payment, ...) and styled codes, and verifies that each one scans;
- `inspect_qr` explains content the assistant already has.

Vision models cannot read QR Codes reliably and tend to invent plausible
content; this server decodes them instead.

```shell
go install github.com/piglig/go-qr/mcp/cmd/go-qr-mcp@latest   # Go 1.25+
claude mcp add go-qr -- go-qr-mcp                             # Claude Code
```

For other clients:

```json
{
  "mcpServers": {
    "go-qr": { "command": "go-qr-mcp", "args": ["-root", "/Users/me/Pictures"] }
  }
}
```

`-root` confines file access to one directory; `-http localhost:8090` serves
streamable HTTP instead of stdio.

See the [guide](../docs/guides/mcp.md) for client setup, the tool schemas,
the inspection report and security notes.
