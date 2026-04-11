// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package main

import (
	"context"
	"database/sql"
	"errors"
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
		// Set chix config at the root. You can also use modify the configuration
		// in child middleware using [chix.GetConfig], updating the config, and then using [Config.Use].
		chix.NewConfig().
			SetLogger(logger).
			SetErrorResolvers(CustomErrorResolver).
			// SetErrorHandler(YourCustomHandler). // Uncomment to completely control the error response (structure, status codes, etc).
			// SetMaskPrivateErrors(false). // Uncomment to disable masking of errors. Only do this special situations (like local development).
			Use(),
		chix.UseRequestID(),
		chix.UseStripSlashes(),
		chix.UseStructuredLogger(chix.DefaultLogConfig()),
	)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello, World!"))
	})

	r.Get("/database-error", func(w http.ResponseWriter, r *http.Request) {
		// This should be masked and replaced using our [CustomErrorResolver].
		chix.ErrorWithCode(w, r, http.StatusInternalServerError, sql.ErrNoRows)
	})

	r.Get("/special-error", func(w http.ResponseWriter, r *http.Request) {
		chix.ErrorWithCode(w, r, http.StatusBadRequest, &SpecialError{
			Message: "this is a special error",
			Foo:     42,
		})
	})

	r.Get("/resolved-error", func(w http.ResponseWriter, r *http.Request) {
		// Example of using a [chix.ResolvedError] directly, to control visibility.
		chix.Error(w, r, &chix.ResolvedError{
			Errs: []error{
				&SpecialError{Message: "this is a special error", Foo: 42},
				errors.New("this is a second error"),
				errors.New("this is a third error"),
			},
			StatusCode: http.StatusInternalServerError,
			Visibility: chix.ErrorPublic,
		})
	})

	// Example of a single error, which is masked.
	r.Get("/error", func(w http.ResponseWriter, r *http.Request) {
		// Using [chix.ErrorWithCode], means that the status code being <500 will
		// make the error public by default, and >=500 will make it private (masked),
		// so this will be masked.
		chix.ErrorWithCode(w, r, http.StatusInternalServerError, errors.New("custom error"))
	})

	// Example of multiple errors, which are not masked due to status code.
	r.Get("/errors", func(w http.ResponseWriter, r *http.Request) {
		// You can pass multiple errors. The response body will have a "error" field,
		// which is the joined version of all errors, and a separate "errors" field,
		// which is a list of all errors. When using something like [chix.Bind],
		// any validation errors will intelligently be split up, to make rendering
		// and troubleshooting easier.
		chix.ErrorWithCode(w, r, http.StatusForbidden, errors.New("custom error 1"), errors.New("custom error 2"))
	})

	// Panics in handlers are recovered by [UseStructuredLogger] when [LogConfig.RecoverPanics]
	// is true (the default from [DefaultLogConfig]). The client receives HTTP 500; the panic
	// is logged as an error with stack information. Can also use [UseRecoverer] middleware,
	// if you are not using [UseStructuredLogger] middleware.
	r.Get("/panic", func(_ http.ResponseWriter, _ *http.Request) {
		panic("intentional panic for error-handling demo")
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		// You can also pass no errors, and just a status code, and the error value
		// will be the defined status text for that status code. Visibility by default
		// will be controlled by the status code, when not using [chix.ResolvedError],
		// [chix.ExposableError] implementations, or error resolvers.
		chix.ErrorWithCode(w, r, http.StatusNotFound)
	})

	return &http.Server{
		Addr:         cli.Flags.HTTP.Bind,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
}

func CustomErrorResolver(oerr *chix.ResolvedError) *chix.ResolvedError {
	if oerr.Err == nil || !errors.Is(oerr.Err, sql.ErrNoRows) {
		// If nil is returned, it continues through the chain of resolvers. If no
		// resolvers modify the error, it will be passed to the error handler.
		return nil
	}
	// Return a new [chix.ResolvedError] completely, or update the existing one
	// and return it. This will skip all subsequent resolvers, and will be passed
	// to the error handler.
	return &chix.ResolvedError{
		Err:        errors.New("resource not found"),
		StatusCode: http.StatusNotFound,
		Visibility: chix.ErrorPublic,
	}
}

type SpecialError struct {
	Message string
	Foo     int
}

func (e *SpecialError) Error() string {
	return e.Message
}

// If your errors implement the [chix.ExposableError] interface, it will be used to
// determine if the error is public. Otherwise, it will return false by default,
// unless determined by something like [chix.ErrorWithCode]. You can also check
// if an error can be exposed to the client by using [chix.IsExposableError].
//
// You can also use error resolvers through [chix.Config.SetErrorResolvers], as
// shown above, to dynamically change the error entirely, control masking, status
// codes, etc.
func (e *SpecialError) Public() bool {
	return true
}
