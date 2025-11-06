// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package main

import (
	"context"
	"embed"
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

var (
	cli = clix.NewWithDefaults[Flags]()
	//go:embed all:public
	frontendFS embed.FS
)

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
			SetAPIBasePath("/api").
			Use(),
		chix.UseContextIP(),
		chix.UseRequestID(),
		chix.UseStripSlashes(),
		chix.UseStructuredLogger(chix.DefaultLogConfig()),
		chix.UseHeaders(map[string]string{
			"Content-Security-Policy": "default-src 'self'; script-src 'self' 'unsafe-inline'; img-src * data:; media-src * data:; style-src 'self' 'unsafe-inline'; object-src 'none'; child-src 'none'; frame-src 'none'; worker-src 'none'",
			"X-Frame-Options":         "DENY",
			"X-Content-Type-Options":  "nosniff",
			"Referrer-Policy":         "no-referrer-when-downgrade",
			"Permissions-Policy":      "clipboard-write=(self)",
		}),
	)

	r.Get("/api/foo", func(w http.ResponseWriter, r *http.Request) {
		chix.JSON(w, r, http.StatusOK, map[string]any{"message": "Hello, World!"})
	})

	r.With(chix.UseHeaders(map[string]string{
		"Vary":          "Accept-Encoding",
		"Cache-Control": "public, max-age=3600",
	})).Mount("/", chix.UseStatic(&chix.StaticConfig{
		FS:     frontendFS,
		Prefix: "/",
		SPA:    true,
		Path:   "public",
	}))

	return &http.Server{
		Addr:         cli.Flags.HTTP.Bind,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
}
