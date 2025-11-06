// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// ExposableError is an interface that can be implemented by errors that can
// indicate if they are safe to be exposed to the client. If an error implements
// this interface, it will be used to determine if the error will be masked or
// not.
type ExposableError interface {
	Public() bool
}

// IsExposableError returns true if the error is safe to be exposed to the client.
// If the error implements the [ExposableError] interface, it will be used to
// determine if the error is public. Otherwise, it will return false. Accounts for
// [ResolvedError]s, and will return the [Public] field.
func IsExposableError(err error) bool {
	if err == nil {
		return false
	}
	if rerr, ok := IsResolvedError(err); ok {
		return rerr.Public
	}
	if ee, ok := err.(ExposableError); ok {
		return !ee.Public()
	}
	return false
}

// ResolvedError is an error that has been resolved (indicating we have some
// information about if it can be exposed to the client, what the resulting
// status code may be, etc).
type ResolvedError struct {
	// Err is the final error that will be used, instead of the original error.
	// If not provided, the original error will be used.
	Err error `json:"error"`

	// Errs is a list of errors that contributed to the final error. If this is
	// provided directly, it will be used to fill the [Err] field.
	Errs []error `json:"errors"`

	// StatusCode is the status code that will be used, instead of the original status
	// code. If not provided, the original status code will be used.
	StatusCode int `json:"status_code"`

	// Public is a flag that indicates if the error is public (can be rendered safely).
	// If false, the error will not be logged, and a generic error message will be
	// returned.
	Public bool `json:"public"`
}

func (e *ResolvedError) Error() string {
	return e.Err.Error()
}

func (e *ResolvedError) Unwrap() []error {
	if len(e.Errs) == 0 {
		return []error{e.Err}
	}
	return e.Errs
}

func (e *ResolvedError) LogAttrs() []slog.Attr {
	if e == nil {
		return []slog.Attr{slog.Bool("error", false)}
	}

	switch {
	case len(e.Errs) == 0:
		return []slog.Attr{slog.String("error", e.Err.Error())}
	case len(e.Errs) == 1:
		return []slog.Attr{slog.String("error", e.Errs[0].Error())}
	default:
		return []slog.Attr{
			slog.String("error", e.Err.Error()),
			slog.Any("errors", errorStringSlice(e.Errs)),
		}
	}
}

// IsResolvedError returns true if the error is a [ResolvedError].
func IsResolvedError(err error) (resolved *ResolvedError, ok bool) {
	var rerr *ResolvedError
	if errors.As(err, &rerr) {
		return rerr, true
	}
	return nil, false
}

// ErrorResolverFn is a function that resolves an error to a client-facing safe error,
// or adjusts the status code based on if it's caused by incorrect user input, etc.
// Resolvers are useful in situations where you want to return a different error when
// the error contains a database-related error (like duplicate key already exists,
// returning a 400 by default), for example. If the function returns nil, it will
// continue through the chain.
type ErrorResolverFn func(oerr *ResolvedError) *ResolvedError

// ErrorHandler is a function that, depending on the input, will either
// respond to a request with a given response structured based off an error
// or do nothing, if there isn't actually an error.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, rerr *ResolvedError)

// Error handles errors in an HTTP request. At least one error must be specified
// (see [IfError] if you want to simplify if-error-then-respond logic). This
// function will do some cursory resolution of the errors based on what was provided
// (e.g. you can pass a [ResolvedError] directly to customize how something is
// resolved, e.g. custom status code, marking an error as public, etc), in addition
// to passing the error to the configured list of error resolvers (see
// [Config.SetErrorResolvers]). It will then pass the resolved error to the configured
// error handler (see [Config.SetErrorHandler]).
//
// Panics if no errors were provided.
func Error(w http.ResponseWriter, r *http.Request, errs ...error) {
	// Remove any nil errors by updating the existing slice.
	for i := 0; i < len(errs); i++ {
		if errs[i] == nil {
			errs = append(errs[:i], errs[i+1:]...)
			i--
		}
	}

	if len(errs) == 0 {
		panic("no error provided")
	}

	resolved := &ResolvedError{
		Public: true,
	}

	if len(errs) == 1 {
		if rerr, ok := IsResolvedError(errs[0]); ok {
			resolved = rerr
		} else {
			resolved.Err = errs[0]
			resolved.Public = IsExposableError(errs[0])
		}
	} else {
		for _, err := range errs {
			if rerr, ok := IsResolvedError(err); ok {
				resolved.Errs = append(resolved.Errs, rerr.Err)
				if rerr.StatusCode > resolved.StatusCode {
					resolved.StatusCode = rerr.StatusCode
				}
				if !rerr.Public {
					resolved.Public = false
				}
				continue
			}

			resolved.Errs = append(resolved.Errs, err)
			if resolved.Public && !IsExposableError(err) {
				resolved.Public = false
			}
		}
	}

	if len(resolved.Errs) > 0 && resolved.Err == nil {
		resolved.Err = errors.Join(resolved.Errs...)
	}

	if resolved.StatusCode == 0 {
		resolved.StatusCode = http.StatusInternalServerError
	}

	for _, fn := range GetConfig(r.Context()).GetErrorResolvers() {
		if fn(resolved) != nil {
			resolved = fn(resolved)
			break
		}
	}

	SetLogError(r.Context(), resolved)
	GetConfig(r.Context()).GetErrorHandler()(w, r, resolved)
}

// IfError is a helper function that allows you to check if an error is present,
// and if so, call [Error] to handle it, and returning true.
//
// Example:
//
//	if chix.IfError(w, r, Something(foo, bar)) {
//		return // Response already handled.
//	}
//	// [... more request-specific logic...]
func IfError(w http.ResponseWriter, r *http.Request, errs ...error) bool {
	if len(errs) == 0 {
		return false
	}
	hasError := false
	for _, err := range errs {
		if err != nil {
			hasError = true
			break
		}
	}
	if !hasError {
		return false
	}
	Error(w, r, errs...)
	return true
}

// ErrorWithCode is a helper function that allows you to set a specific status code
// for an error. It will wrap the error in a [ResolvedError] and pass it to the
// [Error] function. If the status code is <500, the error will be marked as public.
// If this is not desired, you can use [Error] directly instead, and provide your
// own [ResolvedError] with the [Public] field set to false.
func ErrorWithCode(w http.ResponseWriter, r *http.Request, statusCode int, errs ...error) {
	switch {
	case len(errs) == 0:
		Error(w, r, &ResolvedError{
			Err:        errors.New(http.StatusText(statusCode)),
			StatusCode: statusCode,
			Public:     statusCode < 500,
		})
	case len(errs) == 1:
		Error(w, r, &ResolvedError{Err: errs[0], StatusCode: statusCode, Public: statusCode < 500})
	default:
		Error(w, r, &ResolvedError{Errs: errs, StatusCode: statusCode, Public: statusCode < 500})
	}
}

// DefaultErrorBody is the default error body that will be used to render the error,
// used by [DefaultErrorHandler].
type DefaultErrorBody struct {
	Error     string   `json:"error"`
	Errors    []string `json:"errors,omitempty"`
	Type      string   `json:"type"`
	Code      int      `json:"code"`
	RequestID string   `json:"request_id,omitempty"`
	Timestamp string   `json:"timestamp"`
}

// DefaultErrorHandler is the default error handler that will be used if no other error
// handler is provided. It will automatically mask 5xx+ errors if they are not
// configured to be public. If a custom API base path is provided through
// [Config.SetAPIBasePath], if the request matches that base path, it will respond with
// [DefaultErrorBody] as JSON, otherwise will be a generic plain-text error response.
func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, rerr *ResolvedError) {
	cfg := GetConfig(r.Context())

	statusText := http.StatusText(rerr.StatusCode)
	id := GetRequestIDOrHeader(r.Context(), r)

	if !cfg.GetMaskPrivateErrors() || (!rerr.Public && (!IsDebug(r.Context()) || cfg.GetMaskErrorsDebug())) {
		rerr.Err = errors.New("internal server error")
		rerr.Errs = nil
	}

	if apiBasePath := cfg.GetAPIBasePath(); apiBasePath != "" && strings.HasPrefix(r.URL.Path, apiBasePath) {
		JSON(w, r, rerr.StatusCode, DefaultErrorBody{
			Error:     rerr.Err.Error(),
			Errors:    errorStringSlice(rerr.Errs),
			Type:      statusText,
			Code:      rerr.StatusCode,
			RequestID: id,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	if len(rerr.Errs) > 0 {
		http.Error(w, fmt.Sprintf("multiple errors occurred (%s, id: %s):\n%s", statusText, id, strings.Join(errorStringSlice(rerr.Errs), "\n")), rerr.StatusCode)
		return
	}

	http.Error(w, fmt.Sprintf("%s: %s (id: %s)", statusText, rerr.Err.Error(), id), rerr.StatusCode)
}

func errorStringSlice(errs []error) []string {
	if len(errs) == 0 {
		return nil
	}
	s := make([]string, len(errs))
	for i, err := range errs {
		s[i] = err.Error()
	}
	return s
}

// UseRecoverer is a middleware that recovers from panics, and responds with a 500
// status code and appropriate error message. If debug is enabled, through [UseDebug],
// a stack trace will be printed to stderr. Do not use this middleware if you use
// [UseStructuredLogger] middleware, as it already handles panics (in a similar way).
//
// NOTE: This middleware should be loaded after logging/request-id/use-debug, etc
// middleware, but before the handlers that may panic.
func UseRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				if e, ok := rvr.(error); ok && errors.Is(e, http.ErrAbortHandler) {
					panic(rvr)
				}
				if IsDebug(r.Context()) {
					middleware.PrintPrettyStack(rvr)
				}
				ErrorWithCode(w, r, http.StatusInternalServerError, errors.New(string(debug.Stack())))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
