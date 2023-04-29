// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUseAPIVersionMatch(t *testing.T) {
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
				DefaultAPIVersionHeader: "v2",
			},
			ok:         false,
			statusCode: http.StatusPreconditionFailed,
		},
		{
			name:    "match",
			version: "v1",
			headers: map[string]string{
				DefaultAPIVersionHeader: "v1",
			},
			ok:         true,
			statusCode: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			handler := UseAPIVersionMatch(tt.version)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestUseAPIKeyRequired(t *testing.T) {
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
			headers:    map[string]string{DefaultAPIKeyHeader: "qx7zkX6EiONmslV3uIWH"},
			ok:         false,
			statusCode: http.StatusUnauthorized,
		},
		{
			name:       "match",
			keys:       []string{"R5HKAjpQFKNW4KUHF2M4", "O1YQbFh8x5tTpbg4uVhb"},
			headers:    map[string]string{DefaultAPIKeyHeader: "R5HKAjpQFKNW4KUHF2M4"},
			ok:         true,
			statusCode: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			handler := UseAPIKeyRequired(tt.keys)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
