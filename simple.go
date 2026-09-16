// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
)

// UseHeaders is a convenience handler to set multiple response header key/value
// pairs. Similar to go-chi's SetHeader, but allows for multiple headers to be set
// at once.
func UseHeaders(headers map[string]string) func(next http.Handler) http.Handler {
	if headers == nil {
		headers = map[string]string{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for k, v := range headers {
				w.Header().Set(k, v)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// UseStripSlashes is a middleware that strips trailing slashes from the URL path,
// and accounts for the pprof /debug/ path, which breaks when slashes are stripped.
// Use [github.com/go-chi/chi/v5/middleware.StripSlashes] instead if you don't
// need this functionality.
func UseStripSlashes() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/debug/") {
				next.ServeHTTP(w, r)
				return
			}
			var path string
			rctx := chi.RouteContext(r.Context())
			if rctx != nil && rctx.RoutePath != "" {
				path = rctx.RoutePath
			} else {
				path = r.URL.Path
			}
			if len(path) > 1 && path[len(path)-1] == '/' {
				path = path[:len(path)-1]
				if rctx == nil {
					r.URL.Path = path
				} else {
					rctx.RoutePath = path
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

type contextKeyDebug struct{}

// UseDebug is a middleware that allows passing if debugging is enabled for the
// http server. Use IsDebug to check if debugging is enabled.
func UseDebug(debug bool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKeyDebug{}, debug)))
		})
	}
}

// IsDebug returns true if debugging for the server is enabled.
func IsDebug(ctx context.Context) bool {
	// If it's not there, return false anyway.
	debug, _ := ctx.Value(contextKeyDebug{}).(bool)
	return debug
}

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

// UseIfFunc is a conditional middleware that only uses the provided middleware if
// the condition function returns true, otherwise continues as normal.
func UseIfFunc(mw func(next http.Handler) http.Handler, cond func(r *http.Request) bool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cond(r) {
				mw(next).ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

var (
	ErrAccessDenied       = errors.New("access denied")
	ErrAPIKeyInvalid      = errors.New("invalid api key provided")
	ErrAPIKeyMissing      = errors.New("api key not specified")
	ErrAPIVersionMissing  = errors.New("api version not specified")
	ErrAPIVersionMismatch = errors.New("server and client version mismatch")
)

// UseAPIVersionMatch is a middleware that checks if the request has the correct
// API version provided in the associated header.
//
// If no header is provided, the default header "X-Api-Version" will be used.
func UseAPIVersionMatch(version, header string) func(next http.Handler) http.Handler {
	if header == "" {
		header = "X-Api-Version"
	}
	header = http.CanonicalHeaderKey(header)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientVersion := r.Header.Get(header)

			if clientVersion == "" {
				ErrorWithCode(w, r, http.StatusPreconditionFailed, ErrAPIVersionMissing)
				return
			}

			if clientVersion != version {
				ErrorWithCode(w, r, http.StatusPreconditionFailed, ErrAPIVersionMismatch)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// UseAPIKeyRequired is a middleware that checks if the request has the correct
// API keys provided in the associated header. Returns [net/http.StatusUnauthorized]
// if an invalid key is provided, and [net/http.StatusPreconditionFailed] if no key
// header is provided.
//
// If no header is provided, the default header "X-Api-Key" will be used.
func UseAPIKeyRequired(keys []string, header string) func(next http.Handler) http.Handler {
	if header == "" {
		header = "X-Api-Key"
	}
	header = http.CanonicalHeaderKey(header)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			providedKey := r.Header.Get(header)

			if providedKey == "" {
				ErrorWithCode(w, r, http.StatusPreconditionFailed, ErrAPIKeyMissing)
				return
			}

			if slices.Contains(keys, providedKey) {
				next.ServeHTTP(w, r)
				return
			}

			ErrorWithCode(w, r, http.StatusUnauthorized, ErrAPIKeyInvalid)
		})
	}
}

// UseWithContext is a middleware that allows hooking into the request context
// for modification purposes (e.g. injecting values, etc).
func UseWithContext(fn func(ctx context.Context) context.Context) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(fn(r.Context())))
		})
	}
}

// UseCancelOnShutdown cancels the request context when parent is done.
// [net/http.Server.Shutdown] does not cancel in-flight requests, so long-lived
// handlers (long-polling, WebSockets) that only select on [http.Request.Context]
// never return and block graceful shutdown. Mount this only on those routes;
// using it globally would abort ordinary requests at the start of shutdown
// instead of letting them drain.
//
// Pass the same context given to [Run] or [NewServer]. If you only call
// [http.Server.Shutdown], cancel parent from [http.Server.RegisterOnShutdown].
// For Server-Sent Events, [SSEConfig.Shutdown] is the equivalent hook.
func UseCancelOnShutdown(parent context.Context) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithCancel(r.Context())
			defer cancel()
			stop := context.AfterFunc(parent, cancel)
			defer stop()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
