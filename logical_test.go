// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUseIf(t *testing.T) {
	tests := []struct {
		name string
		cond bool
	}{
		{name: "true", cond: true},
		{name: "false", cond: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)

			called := false
			setCalledHandler := func(next http.Handler) http.Handler {
				called = true

				return next
			}

			handler := UseIf(tt.cond, setCalledHandler)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
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
