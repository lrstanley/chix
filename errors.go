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

//nolint:lll
var (
	// DefaultMaskError is a flag that can be used to mask errors in the
	// default error handler. This only impacts errors from 500 onwards.
	// If debug is enabled via the UseDebug middleware, this flag will be
	// ignored.
	DefaultMaskError = true

	ErrAccessDenied       = errors.New("access denied")
	ErrAPIKeyInvalid      = errors.New("invalid api key provided")
	ErrAPIKeyMissing      = errors.New("api key not specified")
	ErrNoAPIKeys          = errors.New("no api keys provided in initialization")
	ErrAPIVersionMissing  = errors.New("api version not specified")
	ErrAPIVersionMismatch = errors.New("server and client version mismatch")
	ErrRealIPNoOpts       = errors.New("realip: no options specified")
	ErrRealIPNoSource     = errors.New("realip: no real IP source specified (OptUseXForwardedFor, OptUseXRealIP, or OptUseTrueClientIP, OptUseCFConnectingIP)")
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
type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error) (ok bool)

// Error handles the error (if any). Handler WILL respond to the request
// with a header and a response if there is an error. The return boolean tells
// the caller if the handler has responded to the request or not. If the
// request includes /api/ as the prefix (see DefaultAPIPrefix), the response
// will be JSON.
//
// If you'd like a specific status code to be returned, there are four options:
//  1. Use AddErrorResolver() to add a custom resolver for err -> status code.
//  2. Use WrapError() to wrap the error with a given status code.
//  3. Use WrapCode() to make an error from a given status code (if you don't
//     have an error that you can provide).
//  4. If none of the above apply, http.StatusInternalServerError will be returned.
//
// NOTE: if you override this function, you must call chix.UnwrapError() on the
// error to get the original error, and the status code, if any of the above are
// used.
var Error = defaultErrorHandler

// defaultErrorHandler is the default ErrorHandler implementation.
func defaultErrorHandler(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}

	var statusCode int
	err, statusCode = UnwrapError(err)
	statusText := http.StatusText(statusCode)

	id := middleware.GetReqID(r.Context())
	if id == "" {
		id = "-"
	}

	if statusCode >= http.StatusInternalServerError {
		Log(r).WithError(err)

		if !IsDebug(r) && DefaultMaskError {
			err = errors.New("internal server error")
		}
	}

	if DefaultAPIPrefix != "" && strings.HasPrefix(r.URL.Path, DefaultAPIPrefix) {
		JSON(w, r, statusCode, M{
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

// ErrorCode is a helper function for Error() that includes a status code in the
// response. See also chix.WrapError() and chix.WrapCode().
func ErrorCode(w http.ResponseWriter, r *http.Request, statusCode int, err error) bool {
	return Error(w, r, WrapError(err, statusCode))
}

// ErrWithStatusCode is an error wrapper that bundles a given status code, that
// can be used by chix.Error() as the response code. See chix.WrapError() and
// chix.WrapCode().
type ErrWithStatusCode struct {
	Err  error
	Code int
}

func (e ErrWithStatusCode) Error() string {
	return fmt.Sprintf("%s (status: %d)", e.Err.Error(), e.Code)
}

func (e ErrWithStatusCode) Unwrap() error {
	return e.Err
}

// UnwrapError is a helper function for retrieving the underlying error and status
// code from an error that has been wrapped.
func UnwrapError(err error) (resultErr error, statusCode int) { //nolint:revive
	if err == nil {
		return nil, 0
	}

	statusCode = http.StatusInternalServerError

	// If the user has wrapped the error, this will override any other code
	// we have.
	var codeErr *ErrWithStatusCode
	if errors.As(err, &codeErr) {
		statusCode = codeErr.Code
		err = codeErr.Unwrap()
		return err, statusCode
	}

	// First try with resolvers.
	if resolvers, ok := errorResolvers.Load().([]ErrorResolver); ok {
		for _, fn := range resolvers {
			var code int
			if code = fn(err); code != 0 {
				statusCode = code
				break
			}
		}
	}
	return err, statusCode
}

// WrapError wraps an error with an http status code, which chix.Error() can use
// as the response code. Example usage:
//
//	if chix.Error(w, r, chix.WrapError(err, http.StatusBadRequest)) {
//	    return
//	}
//
//	if chix.Error(w, r, chix.WrapError(err, 500)) {
//	    return
//	}
func WrapError(err error, code int) error {
	return &ErrWithStatusCode{Err: err, Code: code}
}

// WrapCode is a helper function that returns an error using the status text of the
// given http status code. This is useful if you don't have an explicit error to
// respond with. Example usage:
//
//	chix.Error(w, r, chix.WrapCode(http.StatusBadRequest))
//	return
//
//	chix.Error(w, r, chix.WrapCode(500))
//	return
func WrapCode(code int) error {
	out := http.StatusText(code)
	if out == "" {
		out = fmt.Sprintf("unknown error (%d)", code)
	}

	return &ErrWithStatusCode{Err: errors.New(out), Code: code}
}
