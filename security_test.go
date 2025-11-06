// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

// func TestUseCrossOriginResourceSharing(t *testing.T) {
// 	cases := []struct {
// 		name     string
// 		config   *CORSConfig
// 		method   string
// 		headers  map[string]string
// 		expected map[string]string
// 	}{
// 		{
// 			name:     "no-config",
// 			config:   nil,
// 			method:   http.MethodGet,
// 			headers:  map[string]string{},
// 			expected: map[string]string{"Vary": "Origin"},
// 		},
// 	}
// }

var testHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("bar"))
})

var allHeaders = []string{
	"Vary",
	"Access-Control-Allow-Origin",
	"Access-Control-Allow-Methods",
	"Access-Control-Allow-Headers",
	"Access-Control-Allow-Credentials",
	"Access-Control-Max-Age",
	"Access-Control-Expose-Headers",
}

func assertHeaders(t *testing.T, resHeaders http.Header, expHeaders map[string]string) {
	t.Helper()
	for _, name := range allHeaders {
		got := strings.Join(resHeaders[name], ", ")
		want := expHeaders[name]
		if got != want {
			t.Errorf("Response header %q = %q, want %q", name, got, want)
		}
	}
}

func TestUseCrossOriginResourceSharing(t *testing.T) {
	cases := []struct {
		name       string
		config     *CORSConfig
		method     string
		reqHeaders map[string]string
		resHeaders map[string]string
	}{
		{
			name:   "no-config",
			config: &CORSConfig{
				// Intentionally left blank.
			},
			method:     "GET",
			reqHeaders: map[string]string{},
			resHeaders: map[string]string{
				"Vary": "Origin",
			},
		},
		{
			name: "match-all-origin",
			config: &CORSConfig{
				AllowedOrigins: []string{"*"},
			},
			method: http.MethodGet,
			reqHeaders: map[string]string{
				"Origin": "http://foobar.com",
			},
			resHeaders: map[string]string{
				"Vary":                        "Origin",
				"Access-Control-Allow-Origin": "*",
			},
		},
		{
			name: "match-all-origin-with-credentials",
			config: &CORSConfig{
				AllowedOrigins:   []string{"*"},
				AllowCredentials: true,
			},
			method: http.MethodGet,
			reqHeaders: map[string]string{
				"Origin": "http://foobar.com",
			},
			resHeaders: map[string]string{
				"Vary":                             "Origin",
				"Access-Control-Allow-Origin":      "*",
				"Access-Control-Allow-Credentials": "true",
			},
		},
		{
			name: "allowed-origin",
			config: &CORSConfig{
				AllowedOrigins: []string{"http://foobar.com"},
			},
			method: http.MethodGet,
			reqHeaders: map[string]string{
				"Origin": "http://foobar.com",
			},
			resHeaders: map[string]string{
				"Vary":                        "Origin",
				"Access-Control-Allow-Origin": "http://foobar.com",
			},
		},
		{
			name: "wildcard-origin",
			config: &CORSConfig{
				AllowedOrigins: []string{"http://*.bar.com"},
			},
			method: http.MethodGet,
			reqHeaders: map[string]string{
				"Origin": "http://foo.bar.com",
			},
			resHeaders: map[string]string{
				"Vary":                        "Origin",
				"Access-Control-Allow-Origin": "http://foo.bar.com",
			},
		},
		{
			name: "disallowed-origin",
			config: &CORSConfig{
				AllowedOrigins: []string{"http://foobar.com"},
			},
			method: http.MethodGet,
			reqHeaders: map[string]string{
				"Origin": "http://barbaz.com",
			},
			resHeaders: map[string]string{
				"Vary": "Origin",
			},
		},
		{
			name: "disallowed-wildcard-origin",
			config: &CORSConfig{
				AllowedOrigins: []string{"http://*.bar.com"},
			},
			method: http.MethodGet,
			reqHeaders: map[string]string{
				"Origin": "http://foo.baz.com",
			},
			resHeaders: map[string]string{
				"Vary": "Origin",
			},
		},
		{
			name: "allowed-origin-func-match",
			config: &CORSConfig{
				AllowOriginFunc: func(r *http.Request, o string) ([]string, bool) {
					return nil, regexp.MustCompile("^http://foo").MatchString(o) && r.Header.Get("Authorization") == "secret"
				},
			},
			method: http.MethodGet,
			reqHeaders: map[string]string{
				"Origin":        "http://foobar.com",
				"Authorization": "secret",
			},
			resHeaders: map[string]string{
				"Vary":                        "Origin",
				"Access-Control-Allow-Origin": "http://foobar.com",
			},
		},
		{
			name: "allow-origin-func-not-match",
			config: &CORSConfig{
				AllowOriginFunc: func(r *http.Request, o string) ([]string, bool) {
					return nil, regexp.MustCompile("^http://foo").MatchString(o) && r.Header.Get("Authorization") == "secret"
				},
			},
			method: http.MethodGet,
			reqHeaders: map[string]string{
				"Origin":        "http://foobar.com",
				"Authorization": "not-secret",
			},
			resHeaders: map[string]string{
				"Vary": "Origin",
			},
		},
		{
			name: "max-age",
			config: &CORSConfig{
				AllowedOrigins: []string{"http://example.com/"},
				AllowedMethods: []string{"GET"},
				MaxAge:         10 * time.Second,
			},
			method: http.MethodOptions,
			reqHeaders: map[string]string{
				"Origin":                        "http://example.com/",
				"Access-Control-Request-Method": "GET",
			},
			resHeaders: map[string]string{
				"Vary":                         "Origin, Access-Control-Request-Method, Access-Control-Request-Headers",
				"Access-Control-Allow-Origin":  "http://example.com/",
				"Access-Control-Allow-Methods": "GET",
				"Access-Control-Max-Age":       "10",
			},
		},
		{
			name: "allowed-method",
			config: &CORSConfig{
				AllowedOrigins: []string{"http://foobar.com"},
				AllowedMethods: []string{"PUT", "DELETE"},
			},
			method: http.MethodOptions,
			reqHeaders: map[string]string{
				"Origin":                        "http://foobar.com",
				"Access-Control-Request-Method": "PUT",
			},
			resHeaders: map[string]string{
				"Vary":                         "Origin, Access-Control-Request-Method, Access-Control-Request-Headers",
				"Access-Control-Allow-Origin":  "http://foobar.com",
				"Access-Control-Allow-Methods": "PUT",
			},
		},
		{
			name: "disallowed-method",
			config: &CORSConfig{
				AllowedOrigins: []string{"http://foobar.com"},
				AllowedMethods: []string{"PUT", "DELETE"},
			},
			method: http.MethodOptions,
			reqHeaders: map[string]string{
				"Origin":                        "http://foobar.com",
				"Access-Control-Request-Method": "PATCH",
			},
			resHeaders: map[string]string{
				"Vary": "Origin, Access-Control-Request-Method, Access-Control-Request-Headers",
			},
		},
		{
			name: "allowed-headers",
			config: &CORSConfig{
				AllowedOrigins: []string{"http://foobar.com"},
				AllowedHeaders: []string{"X-Header-1", "x-header-2"},
			},
			method: http.MethodOptions,
			reqHeaders: map[string]string{
				"Origin":                         "http://foobar.com",
				"Access-Control-Request-Method":  "GET",
				"Access-Control-Request-Headers": "X-Header-2, X-HEADER-1",
			},
			resHeaders: map[string]string{
				"Vary":                         "Origin, Access-Control-Request-Method, Access-Control-Request-Headers",
				"Access-Control-Allow-Origin":  "http://foobar.com",
				"Access-Control-Allow-Methods": "GET",
				"Access-Control-Allow-Headers": "X-Header-2, X-Header-1",
			},
		},
		{
			name: "default-allowed-headers",
			config: &CORSConfig{
				AllowedOrigins: []string{"http://foobar.com"},
				AllowedHeaders: []string{},
			},
			method: http.MethodOptions,
			reqHeaders: map[string]string{
				"Origin":                         "http://foobar.com",
				"Access-Control-Request-Method":  "GET",
				"Access-Control-Request-Headers": "Content-Type",
			},
			resHeaders: map[string]string{
				"Vary":                         "Origin, Access-Control-Request-Method, Access-Control-Request-Headers",
				"Access-Control-Allow-Origin":  "http://foobar.com",
				"Access-Control-Allow-Methods": "GET",
				"Access-Control-Allow-Headers": "Content-Type",
			},
		},
		{
			name: "allowed-wildcard-header",
			config: &CORSConfig{
				AllowedOrigins: []string{"http://foobar.com"},
				AllowedHeaders: []string{"*"},
			},
			method: http.MethodOptions,
			reqHeaders: map[string]string{
				"Origin":                         "http://foobar.com",
				"Access-Control-Request-Method":  "GET",
				"Access-Control-Request-Headers": "X-Header-2, X-HEADER-1",
			},
			resHeaders: map[string]string{
				"Vary":                         "Origin, Access-Control-Request-Method, Access-Control-Request-Headers",
				"Access-Control-Allow-Origin":  "http://foobar.com",
				"Access-Control-Allow-Methods": "GET",
				"Access-Control-Allow-Headers": "X-Header-2, X-Header-1",
			},
		},
		{
			name: "disallowed-header",
			config: &CORSConfig{
				AllowedOrigins: []string{"http://foobar.com"},
				AllowedHeaders: []string{"X-Header-1", "x-header-2"},
			},
			method: http.MethodOptions,
			reqHeaders: map[string]string{
				"Origin":                         "http://foobar.com",
				"Access-Control-Request-Method":  "GET",
				"Access-Control-Request-Headers": "X-Header-3, X-Header-1",
			},
			resHeaders: map[string]string{
				"Vary": "Origin, Access-Control-Request-Method, Access-Control-Request-Headers",
			},
		},
		{
			name: "origin-header",
			config: &CORSConfig{
				AllowedOrigins: []string{"http://foobar.com"},
			},
			method: http.MethodOptions,
			reqHeaders: map[string]string{
				"Origin":                         "http://foobar.com",
				"Access-Control-Request-Method":  "GET",
				"Access-Control-Request-Headers": "origin",
			},
			resHeaders: map[string]string{
				"Vary":                         "Origin, Access-Control-Request-Method, Access-Control-Request-Headers",
				"Access-Control-Allow-Origin":  "http://foobar.com",
				"Access-Control-Allow-Methods": "GET",
				"Access-Control-Allow-Headers": "Origin",
			},
		},
		{
			name: "exposed-header",
			config: &CORSConfig{
				AllowedOrigins: []string{"http://foobar.com"},
				ExposedHeaders: []string{"X-Header-1", "x-header-2"},
			},
			method: http.MethodGet,
			reqHeaders: map[string]string{
				"Origin": "http://foobar.com",
			},
			resHeaders: map[string]string{
				"Vary":                          "Origin",
				"Access-Control-Allow-Origin":   "http://foobar.com",
				"Access-Control-Expose-Headers": "X-Header-1, X-Header-2",
			},
		},
		{
			name: "allowed-credentials",
			config: &CORSConfig{
				AllowedOrigins:   []string{"http://foobar.com"},
				AllowCredentials: true,
			},
			method: http.MethodOptions,
			reqHeaders: map[string]string{
				"Origin":                        "http://foobar.com",
				"Access-Control-Request-Method": "GET",
			},
			resHeaders: map[string]string{
				"Vary":                             "Origin, Access-Control-Request-Method, Access-Control-Request-Headers",
				"Access-Control-Allow-Origin":      "http://foobar.com",
				"Access-Control-Allow-Methods":     "GET",
				"Access-Control-Allow-Credentials": "true",
			},
		},
		{
			name: "option-passthrough",
			config: &CORSConfig{
				PassthroughPreflight: true,
			},
			method: http.MethodOptions,
			reqHeaders: map[string]string{
				"Origin":                        "http://foobar.com",
				"Access-Control-Request-Method": "GET",
			},
			resHeaders: map[string]string{
				"Vary":                         "Origin, Access-Control-Request-Method, Access-Control-Request-Headers",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "GET",
			},
		},
		{
			name: "non-preflight-options",
			config: &CORSConfig{
				AllowedOrigins: []string{"http://foobar.com"},
			},
			method: http.MethodOptions,
			reqHeaders: map[string]string{
				"Origin": "http://foobar.com",
			},
			resHeaders: map[string]string{
				"Vary":                        "Origin",
				"Access-Control-Allow-Origin": "http://foobar.com",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := UseCrossOriginResourceSharing(tc.config)

			req := httptest.NewRequest(tc.method, "http://example.com/foo", http.NoBody)
			for name, value := range tc.reqHeaders {
				req.Header.Add(name, value)
			}

			res := httptest.NewRecorder()
			s(testHandler).ServeHTTP(res, req)
			assertHeaders(t, res.Header(), tc.resHeaders)
		})
	}
}

func TestHandleActualRequestInvalidOriginAbortion(t *testing.T) {
	s := UseCrossOriginResourceSharing(&CORSConfig{
		AllowedOrigins: []string{"http://foo.com"},
	})
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/foo", http.NoBody)
	req.Header.Add("Origin", "http://example.com/")

	s(testHandler).ServeHTTP(res, req)

	assertHeaders(t, res.Header(), map[string]string{
		"Vary": "Origin",
	})
}

func TestHandleActualRequestInvalidMethodAbortion(t *testing.T) {
	s := UseCrossOriginResourceSharing(&CORSConfig{
		AllowedMethods:   []string{http.MethodPost},
		AllowCredentials: true,
	})
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/foo", http.NoBody)
	req.Header.Add("Origin", "http://example.com/")

	s(testHandler).ServeHTTP(res, req)

	assertHeaders(t, res.Header(), map[string]string{
		"Vary": "Origin",
	})
}

func TestUseRobotsText(t *testing.T) {
	t.Run("get-robots-default", func(t *testing.T) {
		mw := UseRobotsText(nil)
		req := httptest.NewRequest(http.MethodGet, "http://example.com/robots.txt", http.NoBody)
		res := httptest.NewRecorder()

		mw(testHandler).ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
		}
		if ct := res.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
			t.Fatalf("content-type = %q, want %q", ct, "text/plain; charset=utf-8")
		}
		want := (&RobotsTextConfig{}).String()
		if body := res.Body.String(); body != want {
			t.Fatalf("body = %q, want %q", body, want)
		}
	})

	t.Run("head-robots", func(t *testing.T) {
		mw := UseRobotsText(&RobotsTextConfig{})
		req := httptest.NewRequest(http.MethodHead, "http://example.com/robots.txt", http.NoBody)
		res := httptest.NewRecorder()

		mw(testHandler).ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
		}
		if body := res.Body.Len(); body != 0 {
			t.Fatalf("expected empty body for HEAD, got %d bytes", body)
		}
	})

	t.Run("subpath-robots", func(t *testing.T) {
		cfg := &RobotsTextConfig{Rules: []RobotsTextRule{{UserAgent: "*", Disallow: []string{"/private"}}}}
		mw := UseRobotsText(cfg)
		req := httptest.NewRequest(http.MethodGet, "http://example.com/foo/robots.txt", http.NoBody)
		res := httptest.NewRecorder()

		mw(testHandler).ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
		}
		if ct := res.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
			t.Fatalf("content-type = %q, want %q", ct, "text/plain; charset=utf-8")
		}
		if body := res.Body.String(); body != cfg.String() {
			t.Fatalf("body = %q, want %q", body, cfg.String())
		}
	})

	t.Run("pass-through-non-match", func(t *testing.T) {
		mw := UseRobotsText(nil)
		req := httptest.NewRequest(http.MethodGet, "http://example.com/not-robots", http.NoBody)
		res := httptest.NewRecorder()

		mw(testHandler).ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
		}
		if body := res.Body.String(); body != "bar" {
			t.Fatalf("body = %q, want %q", body, "bar")
		}
	})
}

func TestUseSecurityText(t *testing.T) {
	cfg := SecurityTextConfig{
		ExpiresIn: 24 * time.Hour,
		Contacts:  []string{"security@example.com", "https://example.com/security"},
		KeyLinks:  []string{"https://example.com/pgp.key"},
		Languages: []string{"en"},
		Policies:  []string{"https://example.com/policy"},
		Canonical: []string{"https://example.com/.well-known/security.txt"},
	}

	t.Run("get-security-well-known", func(t *testing.T) {
		mw := UseSecurityText(cfg)
		req := httptest.NewRequest(http.MethodGet, "http://example.com/.well-known/security.txt", http.NoBody)
		res := httptest.NewRecorder()

		mw(testHandler).ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
		}
		if ct := res.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
			t.Fatalf("content-type = %q, want %q", ct, "text/plain; charset=utf-8")
		}
		if body := res.Body.String(); body != cfg.String() {
			t.Fatalf("body = %q, want %q", body, cfg.String())
		}
	})

	t.Run("get-security-root-path", func(t *testing.T) {
		mw := UseSecurityText(cfg)
		req := httptest.NewRequest(http.MethodGet, "http://example.com/security.txt", http.NoBody)
		res := httptest.NewRecorder()

		mw(testHandler).ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
		}
		if ct := res.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
			t.Fatalf("content-type = %q, want %q", ct, "text/plain; charset=utf-8")
		}
		if body := res.Body.String(); body != cfg.String() {
			t.Fatalf("body = %q, want %q", body, cfg.String())
		}
	})

	t.Run("head-security", func(t *testing.T) {
		mw := UseSecurityText(cfg)
		req := httptest.NewRequest(http.MethodHead, "http://example.com/.well-known/security.txt", http.NoBody)
		res := httptest.NewRecorder()

		mw(testHandler).ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
		}
		if body := res.Body.Len(); body != 0 {
			t.Fatalf("expected empty body for HEAD, got %d bytes", body)
		}
	})

	t.Run("pass-through-non-match", func(t *testing.T) {
		mw := UseSecurityText(cfg)
		req := httptest.NewRequest(http.MethodGet, "http://example.com/not-security", http.NoBody)
		res := httptest.NewRecorder()

		mw(testHandler).ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
		}
		if body := res.Body.String(); body != "bar" {
			t.Fatalf("body = %q, want %q", body, "bar")
		}
	})
}
