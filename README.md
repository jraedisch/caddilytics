# Caddilytics

This is a minimal logging middleware for tracking [Caddy][ca] requests via Google Analytics Measurement Protocol.

All requests are being tracked as `pageview`s with referer, language and user agent.

Exception messages for response codes >= 500 are also tracked.

Tracking is done asynchronously with a timeout of 1 second.

Usage (configurable per site):

`caddilytics UA-1234-5 session-cookie`

A session cookie will be set with an unencrypted random UUID if none is set already.

More documentation will follow and I appreciate any advice/criticism/PRs!

Cleanup is definitely needed.

[ca]:https://caddyserver.com
