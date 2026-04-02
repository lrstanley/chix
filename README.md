<!-- template:define:options
{
  "nodescription": true
}
-->
![logo](https://liam.sh/-/gh/svg/lrstanley/chix?icon=logos%3Ago&icon.height=65&layout=left&font=1.1&icon.color=rgba%280%2C+0%2C+0%2C+1%29)

<!-- template:begin:header -->
<!-- do not edit anything in this "template" block, its auto-generated -->

<p align="center">
  <a href="https://github.com/lrstanley/chix/tags">
    <img title="Latest Semver Tag" src="https://img.shields.io/github/v/tag/lrstanley/chix?style=flat-square">
  </a>
  <a href="https://github.com/lrstanley/chix/commits/master">
    <img title="Last commit" src="https://img.shields.io/github/last-commit/lrstanley/chix?style=flat-square">
  </a>




  <a href="https://github.com/lrstanley/chix/actions?query=workflow%3Atest+event%3Apush">
    <img title="GitHub Workflow Status (test @ master)" src="https://img.shields.io/github/actions/workflow/status/lrstanley/chix/test.yml?branch=master&label=test&style=flat-square">
  </a>




  <a href="https://codecov.io/gh/lrstanley/chix">
    <img title="Code Coverage" src="https://img.shields.io/codecov/c/github/lrstanley/chix/master?style=flat-square">
  </a>

  <a href="https://pkg.go.dev/github.com/lrstanley/chix/v2">
    <img title="Go Documentation" src="https://pkg.go.dev/badge/github.com/lrstanley/chix/v2?style=flat-square">
  </a>
  <a href="https://goreportcard.com/report/github.com/lrstanley/chix/v2">
    <img title="Go Report Card" src="https://goreportcard.com/badge/github.com/lrstanley/chix/v2?style=flat-square">
  </a>
</p>
<p align="center">
  <a href="https://github.com/lrstanley/chix/issues?q=is:open+is:issue+label:bug">
    <img title="Bug reports" src="https://img.shields.io/github/issues/lrstanley/chix/bug?label=issues&style=flat-square">
  </a>
  <a href="https://github.com/lrstanley/chix/issues?q=is:open+is:issue+label:enhancement">
    <img title="Feature requests" src="https://img.shields.io/github/issues/lrstanley/chix/enhancement?label=feature%20requests&style=flat-square">
  </a>
  <a href="https://github.com/lrstanley/chix/pulls">
    <img title="Open Pull Requests" src="https://img.shields.io/github/issues-pr/lrstanley/chix?label=prs&style=flat-square">
  </a>
  <a href="https://github.com/lrstanley/chix/discussions/new?category=q-a">
    <img title="Ask a Question" src="https://img.shields.io/badge/support-ask_a_question!-blue?style=flat-square">
  </a>
  <a href="https://liam.sh/chat"><img src="https://img.shields.io/badge/discord-bytecord-blue.svg?style=flat-square" title="Discord Chat"></a>
</p>
<!-- template:end:header -->

<!-- template:begin:toc -->
<!-- do not edit anything in this "template" block, its auto-generated -->
## :link: Table of Contents

  - [Usage](#gear-usage)
  - [Features](#sparkles-features)
  - [Related Libraries](#zap-related-libraries)
  - [Example Projects](#bulb-example-projects)
  - [Support &amp; Assistance](#raising_hand_man-support--assistance)
  - [Contributing](#handshake-contributing)
  - [License](#balance_scale-license)
<!-- template:end:toc -->

## :gear: Usage

<!-- template:begin:goget -->
<!-- do not edit anything in this "template" block, its auto-generated -->
```console
go get -u github.com/lrstanley/chix/v2@latest
```
<!-- template:end:goget -->

## :sparkles: Features

- `http.Server` helpers (`Run`, `RunTLS`) for starting and gracefully shutting down the server, with optional background jobs alongside HTTP via `lrstanley/x/sync/scheduler`.
- Per-request `Config` middleware: API base path, JSON encode/decode hooks, request decode/validate, `slog.Logger`, error resolvers, and masking of non-public 5xx errors.
- RealIP middleware (trusted proxy chain parsing; not "trust any `X-Forwarded-For`").
- Private IP middleware for internal-only routes.
- Request ID middleware (client header or generated ID; header name configurable on `Config`).
- Rendering helpers: JSON, XML, CSV, and streaming CSV via iterators -- all support `?pretty=true` where applicable. JSON uses the standard library by default; `encoding/json/v2` automatically used when compiled with support for it.
- Optional subpackage `xmetrics`: Prometheus HTTP request metrics (duration, count, bytes) keyed by chi route pattern.
- Auth (`xauth` subpackage):
  - [markbates/goth](https://github.com/markbates/goth) OAuth with many providers, plus a separate basic-auth flow.
  - Cookie-backed sessions ([gorilla/sessions](https://github.com/gorilla/sessions)); encrypted store helpers so you can avoid server-side session storage.
  - Generics for user identity type and ID -- no hand-rolled type assertions for your models.
  - Optional auth context, required-auth middleware, and `OverrideContextAuth` for tests or impersonation.
- API key and API version validation middleware (configurable headers).
- Struct binding from query, form, JSON, and multipart data with [go-playground/validator](https://github.com/go-playground/validator).
- Structured request logging with `log/slog`: `UseStructuredLogger` with configurable schemas, levels, optional request/response body capture, panic recovery, and `AppendLogAttrs` / `Log` (and level helpers) for handler-local fields.
- Debug middleware so handlers can tell if debug mode is on; integrates with error responses when you want details only in debug.
- Error handling that distinguishes API vs static/HTML responses, with `ResolvedError`, optional `ExposableError`, and per-error-type resolver functions.
- `go:embed` static file serving (SPA fallback, optional local directory override for development, catch-all safe behavior near API routes).
- Redirect helpers for auth flows: store a `next` URL in a cookie and redirect safely afterward.
- Small utilities: multi-header middleware, strip-slashes that skips `/debug/` for pprof, conditional middleware (`UseIf` / `UseIfFunc`).
- Middleware for `robots.txt` and `security.txt`.

## :zap: Related Libraries

- [lrstanley/clix](https://github.com/lrstanley/clix) -- go-flags wrapper, that
  handles parsing and decoding, with additional helpers.
- [lrstanley/go-query-parser](https://github.com/lrstanley/go-queryparser) -- similar
  to that of Google/Github/etc search, a query string parser that allows filters
  and tags to be dynamically configured by the end user.

## :bulb: Example Projects

Use these as a reference point for how you might use some of the functionality within
this library, or how you might want to structure your applications.

- [lrstanley/geoip](https://github.com/lrstanley/geoip)
- [lrstanley/liam.sh](https://github.com/lrstanley/liam.sh)
- [lrstanley/spectrograph](https://github.com/lrstanley/spectrograph)

<!-- template:begin:support -->
<!-- do not edit anything in this "template" block, its auto-generated -->
## :raising_hand_man: Support & Assistance

* :heart: Please review the [Code of Conduct](.github/CODE_OF_CONDUCT.md) for
     guidelines on ensuring everyone has the best experience interacting with
     the community.
* :raising_hand_man: Take a look at the [support](.github/SUPPORT.md) document on
     guidelines for tips on how to ask the right questions.
* :lady_beetle: For all features/bugs/issues/questions/etc, [head over here](https://github.com/lrstanley/chix/issues/new/choose).
<!-- template:end:support -->

<!-- template:begin:contributing -->
<!-- do not edit anything in this "template" block, its auto-generated -->
## :handshake: Contributing

* :heart: Please review the [Code of Conduct](.github/CODE_OF_CONDUCT.md) for guidelines
     on ensuring everyone has the best experience interacting with the
    community.
* :clipboard: Please review the [contributing](.github/CONTRIBUTING.md) doc for submitting
     issues/a guide on submitting pull requests and helping out.
* :old_key: For anything security related, please review this repositories [security policy](https://github.com/lrstanley/chix/security/policy).
<!-- template:end:contributing -->

<!-- template:begin:license -->
<!-- do not edit anything in this "template" block, its auto-generated -->
## :balance_scale: License

```
MIT License

Copyright (c) 2022 Liam Stanley <liam@liam.sh>

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

_Also located [here](LICENSE)_
<!-- template:end:license -->
