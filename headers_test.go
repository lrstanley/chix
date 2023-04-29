// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
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

			handler := UseHeaders(tt.headers)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
