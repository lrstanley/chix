// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package xauth

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/lrstanley/chix/v2"
	"github.com/markbates/goth/gothic"
)

// BasicAuthService is the interface for the basic authentication service.
type BasicAuthService[Ident any] interface {
	BasicAuth(ctx context.Context, username, password string) (*Ident, error)
	Get(ctx context.Context, username string) (*Ident, error)
}

type BasicAuthConfig[Ident any] struct {
	// Service is the authentication service to use.
	Service BasicAuthService[Ident]

	// SessionStorage is the session storage to use. Take a look at [NewCookieStore] for a
	// convenient way to create a session storage, which doesn't require any server-side
	// state management. Note that this must be the same session storage across all auth
	// handlers, as markbates/goth currently only supports a global session store.
	SessionStorage sessions.Store

	// DisableSelfEndpoint disables the self endpoint.
	DisableSelfEndpoint bool
}

// Validate validates the basic auth config. Use this to validate the config before using
// it, otherwise [NewBasicAuthHandler] will panic if an invalid config is provided.
func (c *BasicAuthConfig[Ident]) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}
	if c.Service == nil {
		return errors.New("service is nil")
	}
	if c.SessionStorage == nil {
		return errors.New("session storage is nil")
	}
	return nil
}

// NewBasicAuthHandler creates a new AuthHandler.
// The following endpoints are implemented:
//   - GET: <mount>/self - returns the current user authentication info (if enabled).
//   - GET: <mount>/login - initiates the provider authentication, using basic auth.
//   - GET: <mount>/logout - logs the user out.
func NewBasicAuthHandler[Ident any](config *BasicAuthConfig[Ident]) http.Handler {
	if err := config.Validate(); err != nil {
		panic(err)
	}

	gothSessionStoreOnce.Do(func() {
		gothic.Store = config.SessionStorage
	})

	router := chi.NewRouter()

	if !config.DisableSelfEndpoint {
		router.With(
			UseAuthContext(config.Service),
			UseAuthRequired[Ident](),
		).Get("/self", func(w http.ResponseWriter, r *http.Request) {
			chix.JSON(w, r, http.StatusOK, map[string]any{"auth": IdentFromContext[Ident](r.Context())})
		})
	}

	router.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		// Check if they've already logged in.
		if IdentFromContext[Ident](r.Context()) != nil {
			chix.SecureRedirectOrNext(w, r, http.StatusTemporaryRedirect, "/")
			return
		}

		user, pass, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
			chix.ErrorWithCode(w, r, http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized)))
			return
		}

		_, err := config.Service.BasicAuth(r.Context(), user, pass)
		if err != nil {
			chix.ErrorWithCode(w, r, http.StatusUnauthorized, err)
			return
		}

		if err = gothic.StoreInSession(authSessionKey, user, r, w); err != nil {
			chix.Error(w, r, err)
			return
		}
		chix.SecureRedirectOrNext(w, r, http.StatusTemporaryRedirect, "/")
	})

	router.Get("/logout", func(w http.ResponseWriter, r *http.Request) {
		_ = gothic.Logout(w, r)
		chix.SecureRedirectOrNext(w, r, http.StatusFound, "/")
	})

	return router
}
