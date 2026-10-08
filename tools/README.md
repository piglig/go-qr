# go-qr tools

A separate module, `github.com/piglig/go-qr/tools`, for programs that are not
part of the library or that need other dependencies:

| Package | Purpose |
| --- | --- |
| [`generator`](generator) | The `generator` command-line tool. See the [CLI guide](../docs/guides/cli.md). |
| [`verify`](verify) | Round-trip helpers that decode rendered PNGs, used by the CLI's `-verify`. |
| [`bench`](bench) | Benchmarks and accuracy tests against other Go QR libraries. See [Performance](../docs/explanation/performance.md). |

```shell
go install github.com/piglig/go-qr/tools/generator@latest
```

The module pins a released version of the library. To work against a local
checkout, see [CONTRIBUTING.md](../CONTRIBUTING.md#repository-layout).
