// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

const nextURLExpiration = 86400 // 1 day.

// UseNextURL is a middleware that will store the current URL provided via
// the "next" query parameter, as a cookie in the response, for use with
// multi-step authentication flows. This allows the user to be redirected
// back to the original destination after authentication. Must use
// chix.SecureRedirect to redirect the user, which will pick up the url from
// the cookie.
func UseNextURL(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n := r.URL.Query().Get("next"); n != "" {
			host := r.Host
			if i := strings.Index(host, ":"); i > -1 {
				host = host[:i]
			}

			http.SetCookie(w, &http.Cookie{
				Name:     nextSessionKey,
				Value:    n,
				Path:     "/",
				MaxAge:   nextURLExpiration,
				Domain:   host,
				HttpOnly: true,
			})
		}
		next.ServeHTTP(w, r)
	})
}

// SecureRedirect supports validating that redirect requests fulfill the the
// following conditions:
//   - Target URL must match one of:
//   - Absolute/relative to the same host
//   - http or https, with a host matching the requested host (no cross-domain, no port matching).
//   - Target URL can be parsed by url.Parse().
//
// Additionally, if using chix.UseNextURL middleware, and the current session
// has a "next" URL stored, the redirect will be to that URL. This allows
// a multi-step authentication flow to be completed, then redirected to the
// original destination.
func SecureRedirect(w http.ResponseWriter, r *http.Request, status int, fallback string) {
	target := fallback

	if skip := r.Context().Value(contextSkipNextURL); skip == nil {
		n, err := r.Cookie(nextSessionKey)
		if err == nil && n.Value != "" {
			target = n.Value
		}
	}

	next, err := url.Parse(target)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	if next.Scheme == "" && next.Host == "" {
		http.Redirect(w, r, next.String(), status)
		return
	}

	if next.Scheme != "http" && next.Scheme != "https" {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	reqHost := r.Host
	if i := strings.Index(reqHost, ":"); i > -1 {
		reqHost = reqHost[:i]
	}

	nextHost := next.Host
	if i := strings.Index(nextHost, ":"); i > -1 {
		nextHost = nextHost[:i]
	}

	if !strings.EqualFold(reqHost, nextHost) {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     nextSessionKey,
		Path:     "/",
		MaxAge:   -1,
		Domain:   reqHost,
		HttpOnly: true,
	})

	http.Redirect(w, r, next.String(), status)
}

// SkipNextURL is a middleware that will prevent the next URL (if any), that
// is loaded by chix.UseNextURL() from being used during a redirect. This is
// useful when you have to redirect to another source first.
func SkipNextURL(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), contextSkipNextURL, true))
}
