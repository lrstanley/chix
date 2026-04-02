// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lrstanley/chix/v2/internal/text"
)

var securityExpires = time.Now()

// UseRobotsText returns a handler that serves a robots.txt file.
func UseRobotsText(config *RobotsTextConfig) func(next http.Handler) http.Handler {
	if config == nil {
		config = &RobotsTextConfig{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/robots.txt") || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
				next.ServeHTTP(w, r)
				return
			}

			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(config.String()))
		})
	}
}

// RobotsTextConfig configures the robots.txt middleware.
type RobotsTextConfig struct {
	Rules    []RobotsTextRule
	Sitemaps []string
}

// String returns the robots.txt file as a string.
func (c *RobotsTextConfig) String() string {
	buf := strings.Builder{}
	if len(c.Rules) == 0 {
		r := RobotsTextRule{
			UserAgent: "*",
			Disallow:  []string{"/"},
		}
		buf.WriteString(r.String() + "\n")
	} else {
		for _, rule := range c.Rules {
			buf.WriteString(rule.String() + "\n")
		}
	}

	for i, sitemap := range c.Sitemaps {
		if i == 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString("Sitemap: " + sitemap + "\n")
	}
	return buf.String()
}

// RobotsTextRule configures a rule for the robots.txt file.
type RobotsTextRule struct {
	UserAgent  string
	Allow      []string
	Disallow   []string
	CrawlDelay time.Duration
}

// String returns the robots.txt rule as a string. Recommended to use
// [RobotsTextConfig.String] instead, as it is the full rendered version.
func (r *RobotsTextRule) String() string {
	buf := strings.Builder{}
	if r.UserAgent == "" {
		buf.WriteString("User-Agent: *\n")
	} else {
		buf.WriteString("User-Agent: " + r.UserAgent + "\n")
	}
	for _, allow := range r.Allow {
		buf.WriteString("Allow: " + allow + "\n")
	}
	for _, disallow := range r.Disallow {
		buf.WriteString("Disallow: " + disallow + "\n")
	}

	if len(r.Allow) == 0 && len(r.Disallow) == 0 {
		buf.WriteString("Disallow:\n")
	}

	if r.CrawlDelay > 0 {
		buf.WriteString("Crawl-Delay: " + strconv.Itoa(int(r.CrawlDelay.Milliseconds())) + "\n")
	}
	return buf.String()
}

// UseSecurityText returns a handler that serves a security.txt file at the
// standardized path(s). Only the provided fields will be included in the
// response.
func UseSecurityText(config SecurityTextConfig) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if (r.URL.Path != "/security.txt" && r.URL.Path != "/.well-known/security.txt") || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
				next.ServeHTTP(w, r)
				return
			}

			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(config.String()))
		})
	}
}

// SecurityTextConfig configures the security.txt middleware.
type SecurityTextConfig struct {
	// Expires is the time when the content of the security.txt file should
	// be considered stale (so security researchers should then not trust it).
	// Make sure you update this value periodically and keep your file under
	// review.
	Expires time.Time

	// ExpiresIn is similar to Expires, but uses a given timeframe from when
	// the http server was started.
	ExpiresIn time.Duration

	// Contacts contains links or e-mail addresses for people to contact you
	// about security issues. Remember to include "https://" for URLs, and
	// "mailto:" for e-mails (this will be auto-included if it contains an @
	// character).
	Contacts []string

	// KeyLinks contains links to keys which security researchers should use
	// to securely talk to you. Remember to include "https://".
	KeyLinks []string

	// Languages is a list of language codes that your security team speaks.
	Languages []string

	// Acknowledgements contains links to webpages where you say thank you
	// to security researchers who have helped you. Remember to include
	// "https://".
	Acknowledgements []string

	// Policies contains links to policies detailing what security researchers
	// should do when searching for or reporting security issues. Remember
	// to include "https://".
	Policies []string

	// Canonical contains the URLs for accessing your security.txt file. It
	// is important to include this if you are digitally signing the
	// security.txt file, so that the location of the security.txt file can
	// be digitally signed too.
	Canonical []string
}

// String returns the security.txt file as a string.
func (h *SecurityTextConfig) String() string {
	buf := strings.Builder{}

	for _, entry := range h.Contacts {
		if strings.Contains(entry, "@") && !strings.Contains(entry, "mailto:") {
			entry = "mailto:" + entry
		}

		buf.WriteString("Contact: " + entry + "\n")
	}

	for _, entry := range h.KeyLinks {
		buf.WriteString("Encryption: " + entry + "\n")
	}

	if len(h.Languages) > 0 {
		buf.WriteString("Preferred-Languages: " + strings.Join(h.Languages, ", ") + "\n")
	}

	for _, entry := range h.Acknowledgements {
		buf.WriteString("Acknowledgements: " + entry + "\n")
	}

	for _, entry := range h.Policies {
		buf.WriteString("Policy: " + entry + "\n")
	}

	for _, entry := range h.Canonical {
		buf.WriteString("Canonical: " + entry + "\n")
	}

	if !h.Expires.IsZero() {
		buf.WriteString("Expires: " + h.Expires.Format(time.RFC3339) + "\n")
	} else if h.ExpiresIn > 0 {
		buf.WriteString("Expires: " + securityExpires.Add(h.ExpiresIn).Format(time.RFC3339) + "\n")
	}

	return buf.String()
}

var (
	ErrCrossOriginRequest           = errors.New("cross-origin request detected from Sec-Fetch-Site header")
	ErrCrossOriginRequestOldBrowser = errors.New("cross-origin request detected, and/or browser is out of date: Sec-Fetch-Site is missing, and Origin does not match Host")
)

type CrossOriginValidator func(r *http.Request, origin *url.URL) bool

// UseCrossOriginProtection implements protections against [Cross-Site Request
// Forgery (CSRF)] by rejecting non-safe cross-origin browser requests. It will check
// if the request is from a trusted origin, and if not, it will return a 403
// Forbidden error. It uses the [Sec-Fetch-Site] and Origin headers to determine
// if the request is trusted. GET, HEAD, and OPTIONS methods are [safe methods] and
// are always allowed. Rules should be provided in one of the following formats:
//   - Trusted origin: "scheme://host[:port]" (e.g. "https://example.com", "http://localhost:8080")
//   - Glob pattern: "*.example.com" (e.g. "*.example.com" which is http/https, "https://*.example.com").
//
// [Cross-Site Request Forgery (CSRF)]: https://developer.mozilla.org/en-US/docs/Web/Security/Attacks/CSRF
// [safe methods]: https://developer.mozilla.org/en-US/docs/Glossary/Safe/HTTP
// [Sec-Fetch-Site]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Sec-Fetch-Site
//
// [Origin]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Origin
func UseCrossOriginProtection(rules ...string) func(next http.Handler) http.Handler {
	var allowed []*url.URL
	var allowedGlob []string

	for _, rule := range rules {
		if strings.Contains(rule, text.GlobChar) {
			if !strings.HasPrefix(rule, "http") {
				allowedGlob = append(allowedGlob, "http://"+rule, "https://"+rule)
				continue
			}
			allowedGlob = append(allowedGlob, rule)
			continue
		}

		uri, err := url.Parse(rule)
		if err != nil {
			panic(fmt.Errorf("failed to parse cross origin rule: %w", err))
		}
		if uri.Path != "" || uri.RawQuery != "" || uri.Fragment != "" {
			panic(errors.New("failed to parse cross origin rule: path, query, and fragment are not allowed"))
		}
		allowed = append(allowed, uri)
	}

	cond := func(_ *http.Request, origin *url.URL) bool {
		if origin == nil {
			return false
		}
		for _, uri := range allowed {
			if uri.Host == origin.Host && uri.Scheme == origin.Scheme {
				return true
			}
		}
		for _, glob := range allowedGlob {
			if text.Glob(glob, origin.Path) {
				return true
			}
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := crossOriginCheck(r, cond); err != nil {
				ErrorWithCode(w, r, http.StatusForbidden, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UseCrossOriginProtectionFunc is an alternative to [UseCrossOriginProtection]
// that allows you to provide a custom condition function to determine if the
// request is trusted. Note that the origin URL may not be provided in the request.
// In that scenario, unless you have a specific path that you want to bypass, you
// should return false.
func UseCrossOriginProtectionFunc(cond CrossOriginValidator) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := crossOriginCheck(r, cond); err != nil {
				ErrorWithCode(w, r, http.StatusForbidden, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func crossOriginCheck(r *http.Request, cond CrossOriginValidator) error {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return nil
	}

	origin, _ := url.Parse(r.Header.Get("Origin"))

	switch r.Header.Get("Sec-Fetch-Site") {
	case "":
		// No Sec-Fetch-Site header is present, so we'll check the Origin header below.
	case "same-origin", "none":
		return nil
	default:
		if cond(r, origin) {
			return nil
		}
		return ErrCrossOriginRequest
	}

	if origin == nil {
		// Neither Sec-Fetch-Site nor Origin headers are present. Either the request
		// is same-origin or not a browser request.
		return nil
	}

	if origin.Host == r.Host {
		// The Origin header matches the Host header. Note that the Host header
		// doesn't include the scheme, so we don't know if this might be an
		// HTTP→HTTPS cross-origin request. We fail open, since all modern
		// browsers support Sec-Fetch-Site since 2023, and running an older
		// browser makes a clear security trade-off already. Sites can mitigate
		// this with HTTP Strict Transport Security (HSTS).
		return nil
	}

	if cond(r, origin) {
		return nil
	}
	return ErrCrossOriginRequestOldBrowser
}

// CORSConfig configures the cross-origin resource sharing middleware.
type CORSConfig struct {
	// AllowedOrigins is a list of origins a cross-domain request can be executed
	// from. Wildcards are supported, though there will be a small perf hit. Default
	// value is "*", which allows all.
	AllowedOrigins []string
	// AllowOriginFunc is a custom function to validate the origin. If this returns
	// true, the origin is allowed. Additionally, if headers is non-nil, the headers
	// are added to the Vary header.
	AllowOriginFunc func(r *http.Request, origin string) (headers []string, allowed bool)
	// AllowedMethods is a list of methods the client is allowed to use with
	// cross-domain requests. Default value is HEAD, GET and POST.
	AllowedMethods []string
	// AllowedHeaders is list of headers the client is allowed to use with
	// cross-domain requests. Default is "Accept", "Content-Type", and "Origin".
	// Allows "*".
	AllowedHeaders []string
	// ExposedHeaders indicates which headers are safe to expose.
	ExposedHeaders []string
	// MaxAge indicates how long the results of a preflight request can be cached.
	// If not provided, no Access-Control-Max-Age header will be sent back, resulting
	// in browsers using their default value (typically 5s). Using a negative value
	// will tell browsers to explicitly disable caching.
	MaxAge time.Duration
	// AllowCredentials indicates whether the request can include user credentials
	// like cookies, HTTP authentication or client side SSL certificates.
	AllowCredentials bool
	// AllowPrivateNetwork indicates whether to accept cross-origin requests over a
	// private network.
	AllowPrivateNetwork bool
	// PassthroughPreflight allows other middleware/handlers to process the OPTIONS
	// method, though this is typically not required.
	PassthroughPreflight bool
	// PreflightStatus provides a status code to use for successful preflight OPTIONS
	// requests. Default is [net/http.StatusNoContent] (204).
	PreflightStatus int

	// Cached logic fields.

	allowsAllOrigins bool
	allowsAllHeaders bool
	exposedHeaders   []string
	preflightVary    []string
	maxAgeSeconds    []string
}

// Validate validates the CORS config. Use this to validate the config before using
// it, otherwise [UseCrossOriginResourceSharing] will panic if an invalid config is
// provided.
func (c *CORSConfig) Validate() error {
	c.AllowedOrigins = text.Map(c.AllowedOrigins, strings.ToLower, strings.TrimSpace)

	if c.AllowOriginFunc == nil { //nolint:nestif
		if len(c.AllowedOrigins) == 0 {
			c.AllowedOrigins = AllowAllCORSConfig().AllowedOrigins
		}
		if slices.Contains(c.AllowedOrigins, "*") {
			c.AllowOriginFunc = func(_ *http.Request, _ string) ([]string, bool) {
				return nil, true
			}
		} else {
			c.AllowOriginFunc = func(_ *http.Request, origin string) ([]string, bool) {
				for _, v := range c.AllowedOrigins {
					if strings.Contains(v, text.GlobChar) {
						if text.Glob(origin, v) {
							return nil, true
						}
					} else {
						if origin == v {
							return nil, true
						}
					}
				}
				return nil, false
			}
		}
	}

	if len(c.AllowedMethods) == 0 {
		c.AllowedMethods = DefaultCORSConfig().AllowedMethods
	}

	if len(c.AllowedHeaders) == 0 {
		c.AllowedHeaders = DefaultCORSConfig().AllowedHeaders
	}

	c.AllowedHeaders = text.Map(c.AllowedHeaders, http.CanonicalHeaderKey)
	c.ExposedHeaders = text.Map(c.ExposedHeaders, http.CanonicalHeaderKey)

	if c.PreflightStatus == 0 {
		c.PreflightStatus = http.StatusNoContent
	}

	// Cached logic fields.

	c.allowsAllOrigins = slices.Contains(c.AllowedOrigins, "*")
	c.allowsAllHeaders = slices.Contains(c.AllowedHeaders, "*")
	c.exposedHeaders = []string{strings.Join(c.ExposedHeaders, ", ")}
	c.preflightVary = []string{"Origin, Access-Control-Request-Method, Access-Control-Request-Headers"}
	if c.AllowPrivateNetwork {
		c.preflightVary[0] += ", Access-Control-Request-Private-Network"
	}

	if c.MaxAge > 0 {
		c.maxAgeSeconds = []string{strconv.Itoa(int(c.MaxAge.Seconds()))}
	} else if c.MaxAge < 0 {
		c.maxAgeSeconds = []string{"0"}
	}

	return nil
}

func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPost,
		},
		AllowedHeaders: []string{
			"Accept",
			"Content-Type",
			"Origin",
		},
	}
}

func AllowAllCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowedHeaders: []string{"*"},
	}
}

var (
	corsHeaderOriginAll  = []string{"*"}
	corsHeaderTrue       = []string{"true"}
	corsHeaderVaryOrigin = []string{"Origin"}
)

func (c *CORSConfig) handlePreflight(r *http.Request, headers http.Header, origin string) {
	if v, ok := headers["Vary"]; ok {
		headers["Vary"] = append(v, c.preflightVary[0])
	} else {
		headers["Vary"] = c.preflightVary
	}

	vary, allowed := c.AllowOriginFunc(r, origin)
	if len(vary) > 0 {
		headers.Add("Vary", strings.Join(text.Map(vary, http.CanonicalHeaderKey), ", "))
	}

	if origin == "" || !allowed {
		return
	}

	if m := r.Header.Get("Access-Control-Request-Method"); m == http.MethodOptions || !slices.Contains(c.AllowedMethods, m) {
		return
	}

	reqHeaders, hasRequestHeaders := r.Header["Access-Control-Request-Headers"]
	reqHeaders = text.Map(text.SplitM(reqHeaders, ","), strings.TrimSpace, http.CanonicalHeaderKey)
	if hasRequestHeaders && !c.allowsAllHeaders {
		for _, h := range reqHeaders {
			if !slices.Contains(c.AllowedHeaders, h) {
				return
			}
		}
	}

	if c.allowsAllOrigins {
		headers["Access-Control-Allow-Origin"] = corsHeaderOriginAll
	} else {
		headers["Access-Control-Allow-Origin"] = r.Header["Origin"]
	}

	headers["Access-Control-Allow-Methods"] = r.Header["Access-Control-Request-Method"]
	if hasRequestHeaders && reqHeaders[0] != "" {
		headers["Access-Control-Allow-Headers"] = reqHeaders
	}

	if c.AllowCredentials {
		headers["Access-Control-Allow-Credentials"] = corsHeaderTrue
	}

	if c.AllowPrivateNetwork && r.Header.Get("Access-Control-Request-Private-Network") == "true" {
		headers["Access-Control-Allow-Private-Network"] = corsHeaderTrue
	}

	if len(c.maxAgeSeconds) > 0 {
		headers["Access-Control-Max-Age"] = c.maxAgeSeconds
	}
}

func (c *CORSConfig) handleRequest(r *http.Request, headers http.Header, origin string) {
	if v := headers["Vary"]; v == nil {
		headers["Vary"] = corsHeaderVaryOrigin
	} else {
		headers["Vary"] = append(v, corsHeaderVaryOrigin[0])
	}

	vary, allowed := c.AllowOriginFunc(r, origin)

	if len(vary) > 0 {
		headers.Add("Vary", strings.Join(text.Map(vary, http.CanonicalHeaderKey), ", "))
	}

	if origin == "" || !allowed {
		return
	}

	if r.Method != http.MethodOptions && !slices.Contains(c.AllowedMethods, r.Method) {
		return
	}

	if c.allowsAllOrigins {
		headers["Access-Control-Allow-Origin"] = corsHeaderOriginAll
	} else {
		headers["Access-Control-Allow-Origin"] = r.Header["Origin"]
	}

	if len(c.exposedHeaders) > 0 {
		headers["Access-Control-Expose-Headers"] = c.exposedHeaders
	}

	if c.AllowCredentials {
		headers["Access-Control-Allow-Credentials"] = corsHeaderTrue
	}
}

// UseCrossOriginResourceSharing is a middleware that implements the [CORS spec]. It will
// set the appropriate headers for the request based on the existence of CORS headers.
// Follows the [CORS spec] as close as possible. [See also].
//
// [CORS spec]: https://fetch.spec.whatwg.org/#http-cors-protocol
// [See also]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CORS
func UseCrossOriginResourceSharing(config *CORSConfig) func(next http.Handler) http.Handler {
	if config == nil {
		config = DefaultCORSConfig()
	}
	if err := config.Validate(); err != nil {
		panic(fmt.Errorf("failed to validate CORS config: %w", err))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headers := w.Header()
			origin := r.Header.Get("Origin")

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				config.handlePreflight(r, headers, origin)

				if config.PassthroughPreflight {
					next.ServeHTTP(w, r)
					return
				}

				w.WriteHeader(config.PreflightStatus)
				return
			}

			config.handleRequest(r, headers, origin)
			next.ServeHTTP(w, r)
		})
	}
}
