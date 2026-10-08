# Security policy

## Supported versions

Security fixes are released for the latest minor version of v2. Version 1 is
no longer maintained; please upgrade (see
[Upgrading to v2](docs/guides/upgrading-to-v2.md)).

| Version | Supported |
| --- | --- |
| 2.x (latest minor) | ✅ |
| 1.x | ❌ |

## Reporting a vulnerability

Please report vulnerabilities privately through
[GitHub's private vulnerability reporting](https://github.com/piglig/go-qr/security/advisories/new)
rather than in a public issue.

Include what an attacker can do, the affected versions, and a minimal input
or program that reproduces it. You can expect an acknowledgement within a
week. Once a fix is ready, it is released and a security advisory is
published, crediting you unless you prefer otherwise.

## Scope

The decoder and `payload.Parse` are designed to handle untrusted input: they
must not panic, hang or use memory out of proportion to the input on any
image or string. Reports of such behavior are in scope. Decoding time grows
with the number of pixels, so applications should limit the size of images
they accept; see [Decoding](docs/guides/decoding.md#decoding-untrusted-images).
