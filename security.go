// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	securityExpires = time.Now()
	robotsTxt       = "User-agent: *\nDisallow: %s\nAllow: /\n"
)

// UseRobotsTxt returns a handler that serves a robots.txt file. When custom
// is empty, the default robots.txt is served (disallow <DefaultAPIPrefix>*, allow /).
//
// You can also use go:embed to embed the robots.txt file into your binary.
// Example:
//
//	//go:embed your/robots.txt
//	var robotsTxt string
//	[...]
//	chix.UseRobotsTxt(router, robotsTxt)
func UseRobotsTxt(custom string) func(next http.Handler) http.Handler {
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

			if custom == "" {
				fmt.Fprintf(w, robotsTxt, DefaultAPIPrefix)
				return
			}

			_, _ = w.Write([]byte(custom))
		})
	}
}

// UseSecurityTxt returns a handler that serves a security.txt file at the
// standardized path(s). Only the provided fields will be included in the
// response.
func UseSecurityTxt(config *SecurityConfig) func(next http.Handler) http.Handler {
	if config == nil {
		panic("SecurityConfig is nil")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/security.txt") || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
				next.ServeHTTP(w, r)
				return
			}

			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}

			config.ServeHTTP(w, r)
		})
	}
}

// SecurityConfig configures the security.txt middleware.
type SecurityConfig struct {
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

func (h *SecurityConfig) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	for _, entry := range h.Contacts {
		if strings.Contains(entry, "@") && !strings.Contains(entry, "mailto:") {
			entry = "mailto:" + entry
		}

		_, _ = w.Write([]byte("Contact: " + entry + "\n"))
	}

	for _, entry := range h.KeyLinks {
		_, _ = w.Write([]byte("Encryption: " + entry + "\n"))
	}

	if len(h.Languages) > 0 {
		_, _ = w.Write([]byte("Preferred-Languages: " + strings.Join(h.Languages, ", ") + "\n"))
	}

	for _, entry := range h.Acknowledgements {
		_, _ = w.Write([]byte("Acknowledgements: " + entry + "\n"))
	}

	for _, entry := range h.Policies {
		_, _ = w.Write([]byte("Policy: " + entry + "\n"))
	}

	for _, entry := range h.Canonical {
		_, _ = w.Write([]byte("Canonical: " + entry + "\n"))
	}

	if !h.Expires.IsZero() {
		_, _ = w.Write([]byte("Expires: " + h.Expires.Format(time.RFC3339) + "\n"))
	} else if h.ExpiresIn > 0 {
		_, _ = w.Write([]byte("Expires: " + securityExpires.Add(h.ExpiresIn).Format(time.RFC3339) + "\n"))
	}
}
