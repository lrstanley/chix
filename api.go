// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"net/http"
)

// Deprecated: APIVersionMatch is deprecated, and will be removed in a future release.
// Please use UseAPIVersionMatch instead.
func APIVersionMatch(version string) func(next http.Handler) http.Handler {
	return UseAPIVersionMatch(version)
}

// UseAPIVersionMatch is a middleware that checks if the request has the correct
// API version provided in the DefaultAPIVersionHeader.
func UseAPIVersionMatch(version string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientVersion := r.Header.Get(DefaultAPIVersionHeader)
			if clientVersion == "" {
				_ = Error(w, r, WrapError(ErrAPIVersionMissing, http.StatusPreconditionFailed))
				return
			}

			if clientVersion != version {
				_ = Error(w, r, WrapError(ErrAPIVersionMismatch, http.StatusPreconditionFailed))
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
	DefaultAPIKeyHeader = "X-Api-Key" //nolint:gosec

	// DefaultAPIPrefix is the default prefix for your API. Set to an empty
	// string to disable checks that change depending on if the request has
	// the provided prefix.
	DefaultAPIPrefix = "/api/"
)

// UseAPIKeyRequired is a middleware that checks if the request has the correct
// API keys provided in the DefaultAPIKeyHeader header. Panics if no keys
// are provided. Returns http.StatusUnauthorized if an invalid key is provided,
// and http.StatusPreconditionFailed if no key header is provided.
func UseAPIKeyRequired(keys []string) func(next http.Handler) http.Handler {
	if len(keys) == 0 {
		panic(ErrNoAPIKeys)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			providedKey := r.Header.Get(DefaultAPIKeyHeader)

			for _, key := range keys {
				if providedKey == key {
					next.ServeHTTP(w, r)
					return
				}
			}

			if providedKey == "" {
				_ = Error(w, r, WrapError(ErrAPIKeyMissing, http.StatusPreconditionFailed))
				return
			}

			Error(w, r, WrapError(ErrAPIKeyInvalid, http.StatusUnauthorized))
		})
	}
}
