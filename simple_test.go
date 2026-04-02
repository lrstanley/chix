// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUseHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
	}{
		{name: "empty", headers: map[string]string{}},
		{name: "single", headers: map[string]string{"Some-Header": "foo"}},
		{name: "multiple", headers: map[string]string{"Some-Header": "foo", "Another-Header": "bar"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)

			handler := UseHeaders(tt.headers)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				for k, v := range tt.headers {
					if w.Header().Get(k) != v {
						t.Errorf("UseHeaders() = %v, want %v", w.Header().Get(k), v)
					}
				}
			}))

			handler.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
}

func TestUseDebug(t *testing.T) {
	tests := []struct {
		name  string
		debug bool
	}{
		{name: "debug", debug: true},
		{name: "not debug", debug: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)

			handler := UseDebug(tt.debug)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				if v := IsDebug(r.Context()); v != tt.debug {
					t.Errorf("IsDebug() = %v, want %v", v, tt.debug)
				}
			}))

			handler.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
}

func TestUseIf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cond bool
	}{
		{name: "true", cond: true},
		{name: "false", cond: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)

			called := false
			setCalledHandler := func(next http.Handler) http.Handler {
				called = true

				return next
			}

			handler := UseIf(tt.cond, setCalledHandler)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
			handler.ServeHTTP(httptest.NewRecorder(), req)

			if tt.cond && !called {
				t.Errorf("UseIf() = %v, want %v", called, tt.cond)
			}

			if !tt.cond && called {
				t.Errorf("UseIf() = %v, want %v", called, tt.cond)
			}
		})
	}
}

func TestUseAPIVersionMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		version    string
		headers    map[string]string
		ok         bool
		statusCode int
	}{
		{
			name:       "empty",
			version:    "v1",
			headers:    map[string]string{},
			ok:         false,
			statusCode: http.StatusPreconditionFailed,
		},
		{
			name:    "mismatch",
			version: "v1",
			headers: map[string]string{
				"X-Api-Version": "v2",
			},
			ok:         false,
			statusCode: http.StatusPreconditionFailed,
		},
		{
			name:    "match",
			version: "v1",
			headers: map[string]string{
				"X-Api-Version": "v1",
			},
			ok:         true,
			statusCode: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			rec := httptest.NewRecorder()
			UseAPIVersionMatch(tt.version, "X-Api-Version")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if !tt.ok {
					t.Error("expected handler to not be invoked, but did")
					return
				}

				w.WriteHeader(http.StatusOK)
			})).ServeHTTP(rec, req)
			resp := rec.Result()

			if resp.StatusCode != tt.statusCode {
				t.Errorf("expected status code %d, got %d", tt.statusCode, resp.StatusCode)
			}
		})
	}
}

func TestUseAPIKeyRequired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		keys       []string
		headers    map[string]string
		ok         bool
		statusCode int
	}{
		{
			name:       "empty",
			keys:       []string{"R5HKAjpQFKNW4KUHF2M4", "O1YQbFh8x5tTpbg4uVhb"},
			headers:    map[string]string{},
			ok:         false,
			statusCode: http.StatusPreconditionFailed,
		},
		{
			name:       "mismatch",
			keys:       []string{"R5HKAjpQFKNW4KUHF2M4", "O1YQbFh8x5tTpbg4uVhb"},
			headers:    map[string]string{"X-Api-Key": "qx7zkX6EiONmslV3uIWH"},
			ok:         false,
			statusCode: http.StatusUnauthorized,
		},
		{
			name:       "match",
			keys:       []string{"R5HKAjpQFKNW4KUHF2M4", "O1YQbFh8x5tTpbg4uVhb"},
			headers:    map[string]string{"X-Api-Key": "R5HKAjpQFKNW4KUHF2M4"},
			ok:         true,
			statusCode: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			handler := UseAPIKeyRequired(tt.keys, "X-Api-Key")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if !tt.ok {
					t.Error("expected handler to not be invoked, but did")
					return
				}

				w.WriteHeader(http.StatusOK)
			}))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			resp := rec.Result()

			if resp.StatusCode != tt.statusCode {
				t.Errorf("expected status code %d, got %d", tt.statusCode, resp.StatusCode)
			}
		})
	}
}

func TestUseStripSlashes(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "http://example.com/foo/", http.NoBody)
	rec := httptest.NewRecorder()
	called := false
	UseStripSlashes()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/foo" {
			t.Errorf("expected path to be /foo, got %q", r.URL.Path)
		}
	})).ServeHTTP(rec, req)
	if !called {
		t.Fatal("expected handler to be called")
	}
}

func TestUseStripSlashes_DebugPrefix(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "http://example.com/debug/pprof/", http.NoBody)
	rec := httptest.NewRecorder()
	called := false
	UseStripSlashes()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/debug/pprof/" {
			t.Errorf("expected debug path to remain unchanged, got %q", r.URL.Path)
		}
	})).ServeHTTP(rec, req)
	if !called {
		t.Fatal("expected handler to be called")
	}
}
