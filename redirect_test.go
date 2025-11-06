// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUseNextURL_WithSecureRedirectOrNext(t *testing.T) {
	mw := UseNextURL()
	req := httptest.NewRequest(http.MethodGet, "https://example.com/login?next=%2Fafter", http.NoBody)
	res := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SecureRedirectOrNext(w, r, http.StatusFound, "/fallback")
	})).ServeHTTP(res, req)

	if res.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusFound)
	}

	if loc := res.Header().Get("Location"); loc != "/after" {
		t.Fatalf("location = %q, want %q", loc, "/after")
	}
}

func TestUseNextURL_WithSecureRedirectOrNext_MultiRequests(t *testing.T) {
	mw := UseNextURL()
	req := httptest.NewRequest(http.MethodGet, "https://example.com/login?next=%2Fafter", http.NoBody)

	res := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("foo"))
	})).ServeHTTP(res, req)

	req = httptest.NewRequest(http.MethodGet, "https://example.com/callback", http.NoBody)
	for _, cookie := range res.Result().Cookies() {
		req.AddCookie(cookie)
	}

	res = httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SecureRedirectOrNext(w, r, http.StatusFound, "/fallback")
	})).ServeHTTP(res, req)

	if res.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusFound)
	}
	if loc := res.Header().Get("Location"); loc != "/after" {
		t.Fatalf("location = %q, want %q", loc, "/after")
	}
}

func TestUseNextURL_NoNextParam(t *testing.T) {
	mw := UseNextURL()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/login", http.NoBody)
	res := httptest.NewRecorder()

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SecureRedirectOrNext(w, r, http.StatusFound, "/home")
	})).ServeHTTP(res, req)

	if res.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusFound)
	}
	if loc := res.Header().Get("Location"); loc != "/home" {
		t.Fatalf("location = %q, want %q", loc, "/home")
	}
}

func TestUseNextURL_SkipNextURL(t *testing.T) {
	mw := UseNextURL()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/login?next=%2Fprofile", http.NoBody)
	res := httptest.NewRecorder()

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = SkipNextURL(r)
		SecureRedirect(w, r, http.StatusFound, "/fallback")
	})).ServeHTTP(res, req)

	if res.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusFound)
	}
	if loc := res.Header().Get("Location"); loc != "/fallback" {
		t.Fatalf("location = %q, want %q", loc, "/fallback")
	}
}

func TestSecureRedirect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		requestURL string
		status     int
		target     string
		wantCode   int
		wantLoc    string
	}{
		{
			name:       "relative-path",
			requestURL: "http://example.com:1234/start",
			status:     http.StatusFound,
			target:     "/foo",
			wantCode:   http.StatusFound,
			wantLoc:    "/foo",
		},
		{
			name:       "relative-no-slash",
			requestURL: "http://example.com:1234/start",
			status:     http.StatusFound,
			target:     "foo",
			wantCode:   http.StatusFound,
			wantLoc:    "/foo",
		},
		{
			name:       "absolute-http-same-host",
			requestURL: "http://example.com:1234/start",
			status:     http.StatusFound,
			target:     "http://example.com/foo",
			wantCode:   http.StatusFound,
			wantLoc:    "http://example.com/foo",
		},
		{
			name:       "absolute-https-same-host",
			requestURL: "http://example.com:1234/start",
			status:     http.StatusFound,
			target:     "https://example.com/bar",
			wantCode:   http.StatusFound,
			wantLoc:    "https://example.com/bar",
		},
		{
			name:       "absolute-same-host-different-port",
			requestURL: "http://example.com:1234/start",
			status:     http.StatusFound,
			target:     "http://example.com:8080/baz",
			wantCode:   http.StatusFound,
			wantLoc:    "http://example.com:8080/baz",
		},
		{
			name:       "scheme-relative-different-host",
			requestURL: "http://example.com:1234/start",
			status:     http.StatusFound,
			target:     "//evil.com/x",
			wantCode:   http.StatusTemporaryRedirect,
			wantLoc:    "/",
		},
		{
			name:       "absolute-different-host",
			requestURL: "http://example.com:1234/start",
			status:     http.StatusFound,
			target:     "https://evil.com/foo",
			wantCode:   http.StatusTemporaryRedirect,
			wantLoc:    "/",
		},
		{
			name:       "invalid-scheme",
			requestURL: "http://example.com:1234/start",
			status:     http.StatusFound,
			target:     "ftp://example.com/foo",
			wantCode:   http.StatusTemporaryRedirect,
			wantLoc:    "/",
		},
		{
			name:       "unparseable-url",
			requestURL: "http://example.com:1234/start",
			status:     http.StatusFound,
			target:     "://bad",
			wantCode:   http.StatusTemporaryRedirect,
			wantLoc:    "/",
		},
		{
			name:       "https-request-forces-http-target-to-https",
			requestURL: "https://example.com/start",
			status:     http.StatusFound,
			target:     "http://example.com/foo",
			wantCode:   http.StatusFound,
			wantLoc:    "https://example.com/foo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tc.requestURL, http.NoBody)
			if strings.HasPrefix(tc.requestURL, "https://") {
				req.TLS = &tls.ConnectionState{}
			}
			res := httptest.NewRecorder()

			SecureRedirect(res, req, tc.status, tc.target)

			if res.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", res.Code, tc.wantCode)
			}
			if loc := res.Header().Get("Location"); loc != tc.wantLoc {
				t.Fatalf("location = %q, want %q", loc, tc.wantLoc)
			}
		})
	}
}
