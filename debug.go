// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"context"
	"net/http"
)

// UseDebug is a middleware that allows passing if debugging is enabled for the
// http server. Use IsDebug to check if debugging is enabled.
func UseDebug(debug bool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextDebug, debug)))
		})
	}
}

// IsDebug returns true if debugging for the server is enabled (gets the
// context from the request).
func IsDebug(r *http.Request) bool {
	return IsDebugCtx(r.Context())
}

// IsDebugCtx returns true if debugging for the server is enabled.
func IsDebugCtx(ctx context.Context) bool {
	// If it's not there, return false anyway.
	debug, _ := ctx.Value(contextDebug).(bool)
	return debug
}
