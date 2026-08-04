// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Create an .env file with the following variables (for session/encryption keys, run without these,
// and they will be printed to the console for you):
//
//	AUTH_SESSION_KEY="YOUR_SESSION_KEY"
//	AUTH_SESSION_ENCRYPT_KEY="YOUR_SESSION_ENCRYPT_KEY"
//
// Once setup, start the dev server and navigate to http://localhost:8080/-/auth/login.
// Your browser will prompt for credentials. Demo users:
//   - alice / secret
//   - bob / hunter2
//
// After login, you should be able to access the / route and see "Hello, {display}!".
// Additionally, navigate to http://localhost:8080/-/auth/self for session information.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lrstanley/chix/v2"
	"github.com/lrstanley/chix/xauth/v2"
	"github.com/lrstanley/clix/v2"
)

type Flags struct {
	HTTP struct {
		Bind string `name:"bind" env:"BIND" default:":8080" help:"The address:port to bind to."`
	} `embed:"" prefix:"http." envprefix:"HTTP_" group:"HTTP server flags"`

	Auth struct {
		SessionKey        string `name:"session-key"         env:"SESSION_KEY"         help:"The authentication key."`
		SessionEncryptKey string `name:"session-encrypt-key" env:"SESSION_ENCRYPT_KEY" help:"The encryption key."`
	} `embed:"" prefix:"auth." envprefix:"AUTH_" group:"Authentication flags"`
}

var cli = clix.NewWithDefaults[Flags]()

func main() {
	ctx := context.Background()
	logger := cli.GetLogger()

	if cli.Flags.Auth.SessionKey == "" || cli.Flags.Auth.SessionEncryptKey == "" {
		fmt.Printf( //nolint:forbidigo
			"initial setup, set the following environment variables:\n  export AUTH_SESSION_KEY=%s\n  export AUTH_SESSION_ENCRYPT_KEY=%s\n",
			xauth.GenerateAuthKey(),
			xauth.GenerateEncryptionKey(),
		)
		os.Exit(1)
	}

	logger.Info("starting server", "bind", cli.Flags.HTTP.Bind)
	err := chix.Run(ctx, logger, httpServer(logger))
	if err != nil {
		logger.Error("run failed", "error", err)
		os.Exit(1)
	}
}

func httpServer(logger *slog.Logger) *http.Server {
	authSvc := newAuthService()

	r := chi.NewRouter()
	r.Use(
		chix.NewConfig().
			SetLogger(logger).
			Use(),
		chix.UseContextIP(),
		chix.UseRequestID(),
		chix.UseStripSlashes(),
		chix.UseStructuredLogger(chix.DefaultLogConfig()),
		xauth.UseAuthContext(authSvc),
	)

	r.Mount("/-/auth", xauth.NewBasicAuthHandler(&xauth.BasicAuthConfig[User]{
		Service: authSvc,
		SessionStorage: xauth.NewCookieStore(
			cli.Flags.Auth.SessionKey,
			cli.Flags.Auth.SessionEncryptKey,
		),
	}))

	r.With(xauth.UseAuthRequired[User]()).Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		user := xauth.IdentFromContext[User](r.Context())
		_, _ = fmt.Fprintf(w, "Hello, %s!", user.Display)
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
