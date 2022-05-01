// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"net/http"
)

// APIVersionMatch is a middleware that checks if the request has the correct
// API version provided in the DefaultAPIVersionHeader.
func APIVersionMatch(version string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientVersion := r.Header.Get(DefaultAPIVersionHeader)
			if clientVersion == "" {
				_ = Error(w, r, http.StatusPreconditionFailed, ErrAPIVersionMissing)
				return
			} else if clientVersion != version {
				_ = Error(w, r, http.StatusPreconditionFailed, ErrAPIVersionMismatch)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

var (
	// DefaultAPIVersionHeader is the default header name for the API version.
	DefaultAPIVersionHeader = "X-Api-Version"

	// DefaultErrorHandler is the default header where we should look for the
	// API key.
	DefaultAPIKeyHeader = "X-Api-Key"

	// DefaultAPIPrefix is the default prefix for your API. Set to an empty
	// string to disable checks that change depending on if the request has
	// the provided prefix.
	DefaultAPIPrefix = "/api/"
)

// UseAPIKeyRequired is a middleware that checks if the request has the correct
// API keys provided in the DefaultAPIKeyHeader header. Panics if no keys
// are provided.
func UseAPIKeyRequired(keys []string) func(next http.Handler) http.Handler {
	if len(keys) == 0 {
		panic(ErrNoAPIKeys)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, key := range keys {
				if r.Header.Get(DefaultAPIKeyHeader) == key {
					next.ServeHTTP(w, r)
					return
				}
			}

			Error(w, r, http.StatusUnauthorized, ErrInvalidAPIKey)
		})
	}
}
