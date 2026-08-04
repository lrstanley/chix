// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"context"
	"errors"
	"math"
	"net/http"
)

// DefaultMaxRequestBodyBytes is the default maximum request body size (4 MiB).
const DefaultMaxRequestBodyBytes = 4 << 20

type contextKeyBodyLimited struct{}

var errLimitResponseWritten = errors.New("body limit response written")

type nopResponseWriter struct{}

func (nopResponseWriter) Header() http.Header { return make(http.Header) }

func (nopResponseWriter) Write(p []byte) (int, error) { return len(p), nil }

func (nopResponseWriter) WriteHeader(int) {}

// limitRequestBody wraps r.Body with [http.MaxBytesReader] when a positive limit is
// configured. When the body was already limited, the stricter of the existing and
// requested limits is applied. When w is non-nil, early rejection and read overflow
// may write HTTP 413 directly; [errLimitResponseWritten] indicates the response was
// already written.
func limitRequestBody(r *http.Request, maxBytes int64, w http.ResponseWriter) error {
	if maxBytes <= 0 {
		return nil
	}

	existing, limited := isBodyLimited(r)
	if limited {
		if maxBytes >= existing {
			return nil
		}
	} else if r.ContentLength > maxBytes {
		return rejectOversizedBody(r, w, maxBytes)
	}

	limitWriter := w
	if limitWriter == nil {
		limitWriter = nopResponseWriter{}
	}

	r.Body = http.MaxBytesReader(limitWriter, r.Body, maxBytes)
	*r = *r.WithContext(context.WithValue(r.Context(), contextKeyBodyLimited{}, maxBytes))
	return nil
}

func rejectOversizedBody(r *http.Request, w http.ResponseWriter, maxBytes int64) error {
	err := &http.MaxBytesError{Limit: maxBytes}
	if w != nil {
		ErrorWithCode(w, r, http.StatusRequestEntityTooLarge, err)
		return errLimitResponseWritten
	}
	return requestEntityTooLargeError(err)
}

func isBodyLimited(r *http.Request) (int64, bool) {
	v, ok := r.Context().Value(contextKeyBodyLimited{}).(int64)
	return v, ok
}

func isLimitResponseWritten(err error) bool {
	return errors.Is(err, errLimitResponseWritten)
}

func requestEntityTooLargeError(err error) *ResolvedError {
	if err == nil {
		err = errors.New(http.StatusText(http.StatusRequestEntityTooLarge))
	}
	return &ResolvedError{
		Err:        err,
		StatusCode: http.StatusRequestEntityTooLarge,
		Visibility: ErrorPublic,
	}
}

func resolveBodyLimitError(err error) (*ResolvedError, bool) {
	if maxBytesErr, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return requestEntityTooLargeError(maxBytesErr), true
	}
	return nil, false
}

func bodyLimitErrorResolver(oerr *ResolvedError) *ResolvedError {
	if re, ok := resolveBodyLimitError(oerr.Err); ok {
		return re
	}
	for _, err := range oerr.Errs {
		if re, ok := resolveBodyLimitError(err); ok {
			return re
		}
	}
	return nil
}

func multipartMaxMemory(maxBytes int64) int64 {
	if maxBytes <= 0 {
		return math.MaxInt64
	}
	return maxBytes
}

// UseMaxBodyBytes is middleware that limits the size of every request body to n bytes.
// When the limit is exceeded the client receives HTTP 413 Request Entity Too Large.
func UseMaxBodyBytes(n int64) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := limitRequestBody(r, n, w); err != nil {
				if !isLimitResponseWritten(err) {
					ErrorWithCode(w, r, http.StatusRequestEntityTooLarge, err)
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
