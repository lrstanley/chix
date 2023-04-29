// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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

			handler := UseDebug(tt.debug)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if IsDebug(r) != tt.debug {
					t.Errorf("IsDebug() = %v, want %v", IsDebug(r), tt.debug)
				}

				if IsDebugCtx(r.Context()) != tt.debug {
					t.Errorf("IsDebugCtx() = %v, want %v", IsDebugCtx(r.Context()), tt.debug)
				}
			}))

			handler.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
}
