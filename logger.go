// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/apex/log"
	"github.com/go-chi/chi/v5/middleware"
)

// LogHandler is a function type that can be used to add any additional
// custom fields to a request log entry.
type LogHandler func(r *http.Request) M

var logHandlers atomic.Value // []LogHandler

// AddLogHandler can be used to inject additional metadata/fields into the
// log context. Use this to add things like authentication information, or
// similar, to the log entry.
//
// NOTE: the request context will only include entries that were registered
// in the request context prior to the structured logger being loaded.
func AddLogHandler(h LogHandler) {
	handlers, ok := logHandlers.Load().([]LogHandler)
	if !ok {
		handlers = []LogHandler{}
	}

	handlers = append(handlers, h)
	logHandlers.Store(handlers)
}

// UseStructuredLogger wraps each request and writes a log entry with
// extra info. UseStructuredLogger also injects a logger into the request
// context that can be used by children middleware business logic.
func UseStructuredLogger(logger log.Interface) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			bfields := log.Fields{}
			bfields["src"] = "http"

			// RequestID middleware must be loaded before this is loaded into
			// the chain.
			if id := middleware.GetReqID(r.Context()); id != "" {
				bfields["rid"] = id
			}

			if ray := r.Header.Get("CF-Ray"); ray != "" {
				bfields["ray_id"] = ray
			}

			if country := r.Header.Get("CF-IPCountry"); country != "" {
				bfields["country"] = country
			}

			wrappedWriter := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			bfields["ip"] = r.RemoteAddr
			bfields["host"] = r.Host
			bfields["proto"] = r.Proto
			bfields["method"] = r.Method
			bfields["ua"] = r.Header.Get("User-Agent")
			bfields["bytes_in"] = r.Header.Get("Content-Length")

			logEntry := logger.WithFields(bfields)
			start := time.Now()
			defer func() {
				finish := time.Since(start)

				// If log handlers were provided, and they returned a map,
				// then we'll use that to add additional fields to the log
				// context.
				if handlers, ok := logHandlers.Load().([]LogHandler); ok {
					var fields M
					for _, fn := range handlers {
						if fields = fn(r); fields != nil {
							logEntry = logEntry.WithFields(fields)
						}
					}
				}

				logEntry.WithFields(log.Fields{
					"code":        wrappedWriter.Status(),
					"duration_ms": finish.Milliseconds(),
					"bytes_out":   wrappedWriter.BytesWritten(),
				}).Info(r.URL.RequestURI())
			}()

			next.ServeHTTP(wrappedWriter, r.WithContext(log.NewContext(r.Context(), logEntry)))
		}

		return http.HandlerFunc(fn)
	}
}

// Log is a helper for obtaining the structured logger from  the request
// context.
func Log(r *http.Request) log.Interface {
	return log.FromContext(r.Context())
}
