// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

var (
	// DefaultMaskError is a flag that can be used to mask errors in the
	// default error handler. This only impacts errors from 500 onwards.
	// If debug is enabled via the UseDebug middleware, this flag will be
	// ignored.
	DefaultMaskError = true

	// ErrMatchStatus can be used to use the status code text as the error message.
	ErrMatchStatus = errors.New("no-op status code error")

	ErrAccessDenied       = errors.New("access denied")
	ErrInvalidAPIKey      = errors.New("invalid api key provided")
	ErrAPIVersionMissing  = errors.New("api version not specified")
	ErrAPIVersionMismatch = errors.New("server and client version mismatch")
	ErrNoAPIKeys          = errors.New("no api keys provided")
	ErrRealIPNoOpts       = errors.New("realip: no options specified")
	ErrRealIPNoSource     = errors.New("realip: no real IP source specified (OptUseXForwardedFor, OptUseXRealIP, or OptUseTrueClientIP)")
	ErrRealIPNoTrusted    = errors.New("realip: no trusted proxies or bogon IPs specified")
	ErrAuthNotFound       = errors.New("auth: no authentiation found")
	ErrAuthMissingRole    = errors.New("auth: missing necessary role")
)

type ErrRealIPInvalidIP struct {
	Err error
}

func (e ErrRealIPInvalidIP) Error() string {
	return fmt.Sprintf("realip: invalid IP or range specified: %s", e.Err.Error())
}

func (e ErrRealIPInvalidIP) Unwrap() error {
	return e.Err
}

// ErrorResolver is a function that converts an error to a status code. If
// 0 is returned, the originally provided status code will be used. Resolvers
// are useful in situations where you want to return a different error when
// the error contains a database-related error (like duplicate key already
// exists, returning a 400 by default), of when you can check if the  error
// is due to user input.
type ErrorResolver func(err error) (status int)

var errorResolvers atomic.Value // []ErrorResolver

// AddErrorResolver can be used to add additional error resolvers to the
// default error handler. These will not be used if a custom error handler
// is used.
func AddErrorResolver(r ErrorResolver) {
	resolvers, ok := errorResolvers.Load().([]ErrorResolver)
	if !ok {
		resolvers = []ErrorResolver{}
	}

	resolvers = append(resolvers, r)
	errorResolvers.Store(resolvers)
}

// ErrorHandler is a function that, depending on the input, will either
// respond to a request with a given response structured based off an error
// or do nothing, if there isn't actually an error.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, status int, err error) (ok bool)

// Error handles the error (if any). Handler WILL respond to the request
// with a header and a response if there is an error. The return boolean tells
// the caller if the handler has responded to the request or not. If the
// request includes /api/ as the prefix (see DefaultAPIPrefix), the response
// will be JSON.
var Error ErrorHandler = defaultErrorHandler

// defaultErrorHandler is the default ErrorHandler implementation.
func defaultErrorHandler(w http.ResponseWriter, r *http.Request, statusCode int, err error) bool {
	if err != nil {
		if resolvers, ok := errorResolvers.Load().([]ErrorResolver); ok {
			for _, fn := range resolvers {
				var code int
				if code = fn(err); code != 0 {
					statusCode = code
					break
				}
			}
		}
	}

	if statusCode == http.StatusNotFound && err == nil {
		err = errors.New("the requested resource was not found")
	}

	if errors.Is(err, ErrMatchStatus) {
		err = errors.New(http.StatusText(statusCode))
	}

	if err == nil {
		return false
	}

	w.WriteHeader(statusCode)

	statusText := http.StatusText(statusCode)

	id := middleware.GetReqID(r.Context())
	if id == "" {
		id = "unknown"
	}

	if statusCode >= 500 {
		Log(r).WithError(err)

		if !IsDebug(r) && DefaultMaskError {
			err = errors.New("internal server error")
		}
	}

	if DefaultAPIPrefix != "" && strings.HasPrefix(r.URL.Path, DefaultAPIPrefix) {
		JSON(w, r, M{
			"error":      err.Error(),
			"type":       statusText,
			"code":       statusCode,
			"request_id": id,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		})
	} else {
		http.Error(w, fmt.Sprintf(
			"%s: %s (id: %s)", statusText, err.Error(), id,
		), statusCode)
	}

	return true
}
