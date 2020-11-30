# Caddilytics

[![GoDoc](https://godoc.org/github.com/jraedisch/caddilytics?status.svg)](https://godoc.org/github.com/jraedisch/caddilytics)

This repository contains a minimal [Caddy][ca] module for tracking HTTP requests via Google Analytics [Measurement Protocol][mp].

Any advice/criticism/PRs are welcome!

## Version 2 Compatibility

Caddilytics is compatible with Caddy V2 only. Earlier commits supported Caddy pre V1, so feel free to browse history for info.

## Tracking Info

All requests are being tracked as `pageview`s with the following data:

- `dl` (location URL)
- `dr` (referer)
- `ua` (user agent)
- `ul` (language)

Tracking is done asynchronously with a timeout of `1` second.

## Usage (configurable per site):

`caddilytics UA-1234-5 session-cookie`

You will also have to define a global order (or a route) in your Caddyfile:

```
{
  order caddilytics before header
}
```

## Cookie

A http only, secure session cookie will be set with an unencrypted random UUID if none is set already.

## TODO (unordered ideas)

- Clean up specs.
- Allow non secure cookies.
- Better documentation, especially about building caddy.
- Log exceptions with `exd` (exception description) as hit type `exception`.
- Track `qt` (queue time).
- Track timing.

## License

Copyright (c) 2017 Jasper Rädisch. See the LICENSE file for license rights and limitations (MIT).

[ca]: https://caddyserver.com
[mp]: https://developers.google.com/analytics/devguides/collection/protocol/v1/
