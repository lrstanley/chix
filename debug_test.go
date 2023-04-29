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
	type args struct {
		debug bool
	}
	tests := []struct {
		name string
		args args
	}{
		{name: "debug", args: args{debug: true}},
		{name: "not debug", args: args{debug: false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)

			handler := UseDebug(tt.args.debug)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if IsDebug(r) != tt.args.debug {
					t.Errorf("IsDebug() = %v, want %v", IsDebug(r), tt.args.debug)
				}

				if IsDebugCtx(r.Context()) != tt.args.debug {
					t.Errorf("IsDebugCtx() = %v, want %v", IsDebugCtx(r.Context()), tt.args.debug)
				}
			}))

			handler.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
}
