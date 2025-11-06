// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUseRequestID_and_Getters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setup      func(r *http.Request) http.Handler
		requestURL string
		assert     func(t *testing.T, r *http.Request)
	}{
		{
			name:       "with-header-default",
			requestURL: "http://example.com/",
			setup: func(r *http.Request) http.Handler {
				r.Header.Set("X-Request-Id", "abc-123")
				mw := UseRequestID()
				return mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
					idCtx := GetRequestID(r.Context())
					idHdr := GetRequestIDOrHeader(r.Context(), r)
					if idCtx != "abc-123" {
						t.Fatalf("GetRequestID = %q, want %q", idCtx, "abc-123")
					}
					if idHdr != "abc-123" {
						t.Fatalf("GetRequestIDOrHeader = %q, want %q", idHdr, "abc-123")
					}
				}))
			},
		},
		{
			name:       "without-header-generates",
			requestURL: "http://example.com/",
			setup: func(_ *http.Request) http.Handler {
				mw := UseRequestID()
				return mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
					idCtx := GetRequestID(r.Context())
					idHdr := GetRequestIDOrHeader(r.Context(), r)
					if idCtx == "" {
						t.Fatal("GetRequestID returned empty")
					}
					if idHdr != idCtx {
						t.Fatalf("GetRequestIDOrHeader = %q, want %q", idHdr, idCtx)
					}
					if !strings.Contains(idCtx, "-") {
						t.Fatalf("generated id %q does not contain '-'", idCtx)
					}
				}))
			},
		},
		{
			name:       "custom-header-used-when-configured",
			requestURL: "http://example.com/",
			setup: func(r *http.Request) http.Handler {
				r.Header.Set("X-Trace-Id", "trace-xyz")
				cfg := NewConfig().SetRequestIDHeader("X-Trace-Id")
				mwCfg := cfg.Use()
				mwReq := UseRequestID()
				return mwCfg(mwReq(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
					idCtx := GetRequestID(r.Context())
					idHdr := GetRequestIDOrHeader(r.Context(), r)
					if idCtx != "trace-xyz" {
						t.Fatalf("GetRequestID = %q, want %q", idCtx, "trace-xyz")
					}
					if idHdr != "trace-xyz" {
						t.Fatalf("GetRequestIDOrHeader = %q, want %q", idHdr, "trace-xyz")
					}
				})))
			},
		},
		{
			name:       "or-header-falls-back-without-context",
			requestURL: "http://example.com/",
			setup: func(r *http.Request) http.Handler {
				r.Header.Set("X-Request-Id", "only-in-header")
				return http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
					idHdr := GetRequestIDOrHeader(r.Context(), r)
					if idHdr != "only-in-header" {
						t.Fatalf("GetRequestIDOrHeader = %q, want %q", idHdr, "only-in-header")
					}
				})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, tc.requestURL, http.NoBody)
			res := httptest.NewRecorder()
			h := tc.setup(req)
			h.ServeHTTP(res, req)
		})
	}
}
