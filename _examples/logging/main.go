// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lrstanley/chix/v2"
	"github.com/lrstanley/clix/v2"
)

type Flags struct {
	HTTP struct {
		Bind string `name:"bind" env:"BIND" default:":8080" help:"The address:port to bind to."`
	} `embed:"" prefix:"http." envprefix:"HTTP_" group:"HTTP server flags"`
}

var cli = clix.NewWithDefaults[Flags]()

func main() {
	ctx := context.Background()
	logger := cli.GetLogger()

	logger.Info("starting server", "bind", cli.Flags.HTTP.Bind)
	err := chix.Run(ctx, logger, httpServer(logger))
	if err != nil {
		logger.Error("run failed", "error", err)
		os.Exit(1)
	}
}

func httpServer(logger *slog.Logger) *http.Server {
	r := chi.NewRouter()
	r.Use(
		chix.NewConfig().
			SetLogger(logger).
			Use(),
		chix.UseRequestID(),
		chix.UseStripSlashes(),
		chix.UseStructuredLogger(chix.DefaultLogConfig()),
	)

	// Example of injecting custom attributes into the log entry within a middleware,
	// used for both structured logger's standard logging, in addition to explicit
	// logging calls through the `chix.Log*` functions.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			chix.AppendLogAttrs(r.Context(), slog.String("custom", "attribute"))
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello, World!"))
		chix.LogInfo(r.Context(), "hello, world")
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		chix.ErrorWithCode(w, r, http.StatusNotFound)
	})

	return &http.Server{
		Addr:         cli.Flags.HTTP.Bind,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
}
