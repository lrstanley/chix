// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
		chix.UseDebug(cli.Debug),
		chix.UseRealIP(nil),
		chix.UseContextIP(),
		chix.UseRequestID(),
		chix.UseStripSlashes(),
		chix.UseStructuredLogger(chix.DefaultLogConfig()),
		chix.UseNextURL(),
		chix.UseCrossOriginProtection("*"),
		chix.UseCrossOriginResourceSharing(nil),
		chix.UseHeaders(map[string]string{
			"Content-Security-Policy": "default-src 'self'; img-src * data:; media-src * data:; style-src 'self' 'unsafe-inline'; object-src 'none'; child-src 'none'; frame-src 'none'; worker-src 'none'",
			"X-Frame-Options":         "DENY",
			"X-Content-Type-Options":  "nosniff",
			"Referrer-Policy":         "no-referrer-when-downgrade",
			"Permissions-Policy":      "clipboard-write=(self)",
		}),
		chix.UseRobotsText(&chix.RobotsTextConfig{
			Rules: []chix.RobotsTextRule{
				{UserAgent: "*", Disallow: []string{"/"}},
			},
		}),
		chix.UseSecurityText(chix.SecurityTextConfig{ // TODO: pointer vs no pointer?
			ExpiresIn: 182 * 24 * time.Hour,
			Contacts: []string{
				"https://liam.sh/chat",
				"https://github.com/lrstanley",
			},
			KeyLinks:  []string{"https://github.com/lrstanley.gpg"},
			Languages: []string{"en"},
		}),
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

	r.Get("/error", func(w http.ResponseWriter, r *http.Request) {
		chix.ErrorWithCode(w, r, http.StatusInternalServerError, errors.New("internal server error"))
	})

	r.Get("/errors", func(w http.ResponseWriter, r *http.Request) {
		chix.ErrorWithCode(w, r, http.StatusInternalServerError, errors.New("internal server error"), errors.New("another error"))
	})

	if cli.Debug {
		r.With(chix.UsePrivateIP()).Mount("/debug", middleware.Profiler())
	}

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
