// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricHTTPDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_duration_seconds",
			Help: "HTTP request latencies in seconds.",
		},
		[]string{"method", "path", "status"},
	)
	metricHTTPCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests made.",
		},
		[]string{"method", "path", "status"},
	)
	metricHTTPBytes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_response_bytes_total",
			Help: "Total number of bytes sent in response to HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)
)

func UsePrometheus(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrappedWriter := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		start := time.Now()
		next.ServeHTTP(wrappedWriter, r)
		elapsed := time.Since(start)

		rctx := chi.RouteContext(r.Context())

		labels := prometheus.Labels{
			"method": r.Method,
			"path":   rctx.RoutePattern(),
			"status": strconv.Itoa(wrappedWriter.Status()),
		}

		metricHTTPDuration.With(labels).Observe(elapsed.Seconds())
		metricHTTPCount.With(labels).Inc()
		metricHTTPBytes.With(labels).Add(float64(wrappedWriter.BytesWritten()))
	})
}
