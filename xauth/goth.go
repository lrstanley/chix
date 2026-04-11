// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package xauth

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/lrstanley/chix/v2"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

type GothConfig[Ident any, ID comparable] struct {
	// Service is the authentication service to use.
	Service Service[Ident, ID]

	// SessionStorage is the session storage to use. Take a look at [NewCookieStore] for a
	// convenient way to create a session storage, which doesn't require any server-side
	// state management. Note that this must be the same session storage across all auth
	// handlers, as markbates/goth currently only supports a global session store.
	SessionStorage sessions.Store

	// DisableSelfEndpoint disables the self endpoint.
	DisableSelfEndpoint bool
}

// Validate validates the Goth config. Use this to validate the config before using
// it, otherwise [NewGothHandler] will panic if an invalid config is provided.
func (c *GothConfig[Ident, ID]) Validate() error {
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

// NewGothHandler creates a new auth handler using markbates/goth.
//
// The following endpoints are implemented:
//   - GET: <mount>/self - returns the current user authentication info (if enabled).
//   - GET: <mount>/providers - returns a list of all available providers.
//   - GET: <mount>/providers/{provider} - initiates the provider authentication.
//   - GET: <mount>/providers/{provider}/callback - redirect target from the provider.
//   - GET: <mount>/logout - logs the user out.
func NewGothHandler[Ident any, ID comparable](config *GothConfig[Ident, ID]) http.Handler {
	if err := config.Validate(); err != nil {
		panic(err)
	}

	gothSessionStoreOnce.Do(func() {
		gothic.Store = config.SessionStorage
	})

	mux := http.NewServeMux()

	if !config.DisableSelfEndpoint {
		var self http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			chix.JSON(w, r, http.StatusOK, map[string]any{"auth": IdentFromContext[Ident](r.Context())})
		})
		self = UseAuthRequired[Ident]()(self)
		self = UseAuthContext(config.Service)(self)
		mux.Handle("GET /self", self)
	}

	mux.HandleFunc("GET /providers", func(w http.ResponseWriter, r *http.Request) {
		providers := goth.GetProviders()
		data := make([]string, 0, len(providers))
		for _, p := range providers {
			data = append(data, p.Name())
		}
		chix.JSON(w, r, http.StatusOK, map[string]any{"providers": data})
	})

	validateProvider := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := goth.GetProvider(r.PathValue("provider")) // Validate if provider exists.
			if err != nil {
				chix.ErrorWithCode(w, r, http.StatusBadRequest, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	mux.Handle("GET /providers/{provider}", validateProvider(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gothic.BeginAuthHandler(w, r)
	})))

	mux.Handle("GET /providers/{provider}/callback", validateProvider(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guser, err := gothic.CompleteUserAuth(w, r)
		if err != nil {
			chix.ErrorWithCode(w, r, http.StatusBadRequest, err)
			return
		}
		id, err := config.Service.Set(r.Context(), &guser)
		if err != nil {
			chix.ErrorWithCode(w, r, http.StatusUnauthorized, err)
			return
		}
		if err = gothic.StoreInSession(authSessionKey, fmt.Sprintf("%v", id), r, w); err != nil {
			chix.Error(w, r, err)
			return
		}
		chix.SecureRedirectOrNext(w, r, http.StatusTemporaryRedirect, "/")
	})))

	mux.HandleFunc("GET /logout", func(w http.ResponseWriter, r *http.Request) {
		_ = gothic.Logout(w, r)
		chix.SecureRedirectOrNext(w, r, http.StatusFound, "/")
	})

	return mux
}
