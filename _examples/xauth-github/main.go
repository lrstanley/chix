// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

// For this example, in Github, create a new OAuth application, and set the callback URL to
// "http://localhost:8080/-/auth/providers/github/callback".
//
// Create an .env file with the following variables (for session/encryption keys, run without these,
// and they will be printed to the console for you):
//
//	AUTH_CLIENT_ID="YOUR_CLIENT_ID"
//	AUTH_CLIENT_SECRET="YOUR_CLIENT_SECRET"
//	AUTH_SESSION_KEY="YOUR_SESSION_KEY"
//	AUTH_SESSION_ENCRYPT_KEY="YOUR_SESSION_ENCRYPT_KEY"
//
// Once setup, start the dev server, and navigate to http://localhost:8080/-/auth/providers/github to
// authenticate. Once authenticated, you should be able to access the / route, and see "Hello, {username}!".
// Additionally, you can navigate to http://localhost:8080/-/auth/self to see the authenticated user's
// information.
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
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/github"
)

type Flags struct {
	HTTP struct {
		Bind    string `name:"bind" env:"BIND" default:":8080" help:"The address:port to bind to."`
		BaseURL string `name:"base-url" env:"BASE_URL" default:"http://localhost:8080" help:"The base URL of the server."`
	} `embed:"" prefix:"http." envprefix:"HTTP_" group:"HTTP server flags"`

	Auth struct {
		ClientID          string `name:"client-id" env:"CLIENT_ID" required:"" help:"The GitHub client ID."`
		ClientSecret      string `name:"client-secret" env:"CLIENT_SECRET" required:"" help:"The GitHub client secret."`
		SessionKey        string `name:"session-key" env:"SESSION_KEY" help:"The authentication key."`
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
	goth.UseProviders(
		github.New(
			cli.Flags.Auth.ClientID,
			cli.Flags.Auth.ClientSecret,
			cli.Flags.HTTP.BaseURL+"/-/auth/providers/github/callback",
			"read:user",
			"user:email",
		),
	)

	authSvc := &AuthService{
		users: make(map[string]*User),
	}

	r := chi.NewRouter()
	r.Use(
		chix.NewConfig().
			SetLogger(logger).
			Use(),
		chix.UseContextIP(),
		chix.UseRequestID(),
		chix.UseStripSlashes(),
		chix.UseStructuredLogger(chix.DefaultLogConfig()),
		// This ensures you can fetch the authentication information from any child
		// handler/process/etc of a request, using [xauth.IdentFromContext] and
		// [xauth.IDFromContext].
		xauth.UseAuthContext(authSvc),
	)

	// Register the auth handler itself, which allows logging in/out, listing providers,
	// and allows the user (or frontend, for example) to acquire session information.
	r.Mount("/-/auth", xauth.NewGothHandler(&xauth.GothConfig[User, string]{
		Service: authSvc,
		// We'll use encrypted cookies to store the session information. Using encrypted
		// cookies means that we can securely store session information through the
		// users client, without having to have server-side state management. Though,
		// it has the downside that it by default doesn't allow things like
		// session-invalidation. It's possible to do, but requires some additional
		// state management on the server-side.
		//
		// Alternatively, you could use file storage, a database, etc.
		SessionStorage: xauth.NewCookieStore(
			cli.Flags.Auth.SessionKey,
			cli.Flags.Auth.SessionEncryptKey,
		),
	}))

	// This is a simple example of how to use the authentication information to require
	// authentication for a route.
	r.With(xauth.UseAuthRequired[User]()).Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Because we're using [xauth.UseAuthRequired], we can be sure that the user will
		// be returned here. However, for endpoints that don't require authentication, you
		// should check if the user is nil, and handle it accordingly.
		user := xauth.IdentFromContext[User](r.Context())
		_, _ = fmt.Fprintf(w, "Hello, %s!", user.Username)
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
