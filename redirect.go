// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	nextSessionKey    = "_next"
	nextURLExpiration = 24 * time.Hour
)

// UseNextURL is a middleware that will store the current URL provided via the
// "next" query parameter, as a cookie in the response, for use with multi-step
// authentication flows. This allows the user to be redirected back to the original
// destination after authentication. Must use [SecureRedirect] to redirect the
// user, which will pick up the url from the cookie.
func UseNextURL() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if n := r.URL.Query().Get("next"); n != "" {
				host := r.Host
				if i := strings.Index(host, ":"); i > -1 {
					host = host[:i]
				}

				var secure bool
				if r.TLS != nil {
					secure = true
				}

				http.SetCookie(w, &http.Cookie{
					Name:     nextSessionKey,
					Value:    n,
					Path:     "/",
					MaxAge:   int(nextURLExpiration.Seconds()),
					Domain:   host,
					HttpOnly: true,
					Secure:   secure,
					SameSite: http.SameSiteLaxMode,
				})
			}
			next.ServeHTTP(w, r)
		})
	}
}

type contextKeySkipNextURL struct{}

// SkipNextURL is a help that will prevent the next URL (if any), that is loaded by
// [UseNextURL] from being used during a redirect. This is useful when you have to
// redirect to another source first.
func SkipNextURL(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), contextKeySkipNextURL{}, true))
}

// SecureRedirect supports validating that redirect requests fulfill the following
// conditions:
//
// Target URL must match one of:
//   - Absolute/relative to the same host
//   - http or https, with a host matching the requested host (no cross-domain, no
//     port matching).
//   - Target URL can be parsed by [net/url.Parse].
//
// Additionally, if using [UseNextURL] middleware, see [SecureRedirectOrNext] for
// advanced redirect logic.
func SecureRedirect(w http.ResponseWriter, r *http.Request, status int, target string) {
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

	// Enforce that HTTPS requests can only redirect to HTTPS when the target
	// includes an explicit scheme. Relative targets (no scheme) are allowed.
	if r.TLS != nil && next.Scheme == "http" {
		next.Scheme = "https"
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

// SecureRedirectOrNext is a helper function that will redirect to the next URL if it
// is stored in the session (using [UseNextURL]), otherwise it will redirect to the
// fallback URL, using [SecureRedirect]. See [SecureRedirect] for more details on the
// set of rules that are enforced when redirecting/security checks.
//
// Use this for advanced multi-step authentication flows, where a frontend (for example),
// can pass in "?next=<URL>" to redirect to after authentication is complete, which is
// persisted in a cookie across multiple requests.
func SecureRedirectOrNext(w http.ResponseWriter, r *http.Request, status int, fallback string) {
	if skip := r.Context().Value(contextKeySkipNextURL{}); skip == nil {
		// Check current query parameters first.
		if n := r.URL.Query().Get("next"); n != "" {
			SecureRedirect(w, r, status, n)
			return
		}

		// Check session cookie next.
		if n, err := r.Cookie(nextSessionKey); err == nil && n.Value != "" {
			// Clear cookie.
			reqHost := r.Host
			if i := strings.Index(reqHost, ":"); i > -1 {
				reqHost = reqHost[:i]
			}
			http.SetCookie(w, &http.Cookie{
				Name:     nextSessionKey,
				Path:     "/",
				Value:    "",
				Expires:  time.Unix(0, 0),
				Domain:   reqHost,
				HttpOnly: true,
			})
			SecureRedirect(w, r, status, n.Value)
			return
		}
	}
	SecureRedirect(w, r, status, fallback)
}
