// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/go-chi/chi/v5/middleware"
)

// Deprecated: Recoverer is deprecated, and will be removed in a future release.
// Please use UseRecoverer instead.
func Recoverer(next http.Handler) http.Handler {
	return UseRecoverer(next)
}

// UseRecoverer is a middleware that recovers from panics, and returns a chix.Error
// with HTTP 500 status (Internal Server Error) if possible. If debug is enabled,
// through UseDebug(), a stack trace will be printed to stderr, otherwise to
// standard structured logging.
//
// NOTE: This middleware should be loaded after logging/request-id/use-debug, etc
// middleware, but before the handlers that may panic.
func UseRecoverer(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				if rvr == http.ErrAbortHandler {
					panic(rvr)
				}

				err := fmt.Errorf("panic recovered: %v", rvr)
				if IsDebug(r) {
					middleware.PrintPrettyStack(rvr)
				} else {
					Log(r).WithError(err).Error(string(debug.Stack()))
				}

				ErrorCode(w, r, http.StatusInternalServerError, err)
			}
		}()

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
