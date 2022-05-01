<!-- template:begin:header -->
<!-- do not edit anything in this "template" block, its auto-generated -->
<p align="center">chix -- helper middleware and functionality for go-chi/chi</p>
<p align="center">
  <a href="https://github.com/lrstanley/chix/tags">
    <img title="Latest Semver Tag" src="https://img.shields.io/github/v/tag/lrstanley/chix?style=flat-square">
  </a>
  <a href="https://github.com/lrstanley/chix/commits/master">
    <img title="Last commit" src="https://img.shields.io/github/last-commit/lrstanley/chix?style=flat-square">
  </a>


  <a href="https://github.com/lrstanley/chix/actions?query=workflow%3Atest+event%3Apush">
    <img title="GitHub Workflow Status (test @ master)" src="https://img.shields.io/github/workflow/status/lrstanley/chix/test/master?label=test&style=flat-square&event=push">
  </a>

  <a href="https://codecov.io/gh/lrstanley/chix">
    <img title="Code Coverage" src="https://img.shields.io/codecov/c/github/lrstanley/chix/master?style=flat-square">
  </a>

  <a href="https://pkg.go.dev/github.com/lrstanley/chix">
    <img title="Go Documentation" src="https://pkg.go.dev/badge/github.com/lrstanley/chix?style=flat-square">
  </a>
  <a href="https://goreportcard.com/report/github.com/lrstanley/chix">
    <img title="Go Report Card" src="https://goreportcard.com/badge/github.com/lrstanley/chix?style=flat-square">
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
  - [Support &amp; Assistance](#raising_hand_man-support--assistance)
  - [Contributing](#handshake-contributing)
  - [License](#balance_scale-license)
<!-- template:end:toc -->

## :gear: Usage

<!-- template:begin:goget -->
<!-- do not edit anything in this "template" block, its auto-generated -->
```console
$ go get -u github.com/lrstanley/chix@latest
```
<!-- template:end:goget -->

## :sparkles: Features

- `http.Server` wrapper that easily allows starting, and gracefully shutting
  down your http server, and other background services, using `errgroup`.
- RealIP middleware (supports whitelisting specific proxies, rather than allowing
  any source).
- private IP middleware, restricting endpoints to be internal only.
- Rendering helpers:
  - `JSON` (with `?pretty=true` support).
- Auth middleware:
  - Uses [markbates/goth](https://github.com/markbates/goth) to support many
    different providers.
  - Encrypts session cookies, which removes the need for local session storage.
  - Uses Go 1.18's generics functionality to provide a custom ID and auth object
    resolver.
    - No longer have to type assert to your local models!
  - Optionally requiring authentication.
  - Optionally requiring specific roles.
  - Optionally adding authentication info to context for use by children handlers.
  - API key validation.
  - API version validation.
- Struct/type binding, from get/post data, with support for [go-playground/validator](https://github.com/go-playground/validator).
- Structured logging using [apex/log](https://github.com/apex/log) (same API
  as logrus).
  - Allows injecting additional metadata into logs.
  - Injects logger into context for use by children handlers.
- Debug middleware:
  - Easily let children handlers know if global debug flags are enabled.
  - Allows masking errors, unless debugging is enabled.
- Error handler, that automatically handles api-vs-static content responses.
  - Supports `ErrorResolver`'s, providing the ability to override status codes
    for specific types of errors.
- `go:embed` helpers for mounting an embedded filesystem seamlessly as an http
  endpoint.
  - Useful for projects that bundle their frontend assets in their binary.
  - Supports local filesystem reading, when debugging is enabled (TODO).
- Middleware for robots.txt and security.txt responding.

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

Copyright (c) 2022 Liam Stanley <me@liamstanley.io>

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
