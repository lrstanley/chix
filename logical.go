// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import "net/http"

// UseIf is a conditional middleware that only uses the provided middleware if
// the condition is true, otherwise continues as normal.
func UseIf(cond bool, handler func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if !cond {
			return next
		}

		return handler(next)
	}
}
