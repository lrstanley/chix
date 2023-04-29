// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

var (
	// DefaultCookieSecure allows enabling the secure flag on the session cookie.
	DefaultCookieSecure = false

	// DefaultCookieMaxAge is the max age for the session cookie.
	DefaulltCookieMaxAge = 30 * 86400

	gothInit sync.Once
)

// AuthService is the interface for the authentication service. This will
// need to be implemented to utilize AuthHandler.
type AuthService[Ident any, ID comparable] interface {
	Get(context.Context, ID) (*Ident, error)
	Set(context.Context, *goth.User) (ID, error)
	Roles(context.Context, ID) ([]string, error)
}

// NewAuthHandler creates a new AuthHandler. authKey is used to validate the
// session cookie. encryptKey is used to encrypt the session cookie.
//
// It is recommended to use an authentication key with 32 or 64 bytes. The
// encryption key, if set, must be either 16, 24, or 32 bytes to select
// AES-128, AES-192, or AES-256 modes. Provide the keys in hexadecimal string
// format. The following link can be used to generate a random key:
//   - https://go.dev/play/p/xwcJmQNU8ku
//
// The following endpoints are implemented:
//   - GET: <mount>/self - returns the current user authentication info.
//   - GET: <mount>/providers - returns a list of all available providers.
//   - GET: <mount>/providers/{provider} - initiates the provider authentication.
//   - GET: <mount>/providers/{provider}/callback - redirect target from the provider.
//   - GET: <mount>/logout - logs the user out.
func NewAuthHandler[Ident any, ID comparable](auth AuthService[Ident, ID], authKey, encryptKey string) *AuthHandler[Ident, ID] {
	authKeyBytes, err := hex.DecodeString(authKey)
	if err != nil {
		panic(err)
	}
	encryptKeyBytes, err := hex.DecodeString(encryptKey)
	if err != nil {
		panic(err)
	}

	gothInit.Do(func() {
		authStore := sessions.NewCookieStore(authKeyBytes, encryptKeyBytes)
		authStore.MaxAge(DefaulltCookieMaxAge)
		authStore.Options.Path = "/"
		authStore.Options.HttpOnly = true
		authStore.Options.Secure = DefaultCookieSecure
		gothic.Store = authStore
	})

	h := &AuthHandler[Ident, ID]{
		Auth:         auth,
		Ident:        new(Ident),
		ID:           new(ID),
		errorHandler: Error,
	}

	router := chi.NewRouter()
	router.With(h.AddToContext, h.AuthRequired).Get("/self", h.self)
	router.Get("/providers", h.providers)
	router.Get("/providers/{provider}", h.provider)
	router.Get("/providers/{provider}/callback", h.callback)
	router.Get("/logout", h.logout)
	h.router = router

	AddLogHandler(func(r *http.Request) M {
		id := h.getIDFromSession(r)
		return M{"user_id": id}
	})

	return h
}

// AuthHandler wraps all authentication logic.
type AuthHandler[Ident any, ID comparable] struct {
	Auth         AuthService[Ident, ID]
	Ident        *Ident
	ID           *ID
	router       http.Handler
	errorHandler ErrorHandler
}

// SetErrorHandler sets the error handler for AuthHandler. This error handler will
// only be used for errors that occur within the callback process, NOT for middleware,
// in which chix.Error() will still be used.
func (h *AuthHandler[Ident, ID]) SetErrorHandler(handler ErrorHandler) {
	h.errorHandler = handler
}

// ServeHTTP implements http.Handler.
func (h *AuthHandler[Ident, ID]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

func (h *AuthHandler[Ident, ID]) providers(w http.ResponseWriter, r *http.Request) {
	providers := goth.GetProviders()
	var data []string
	for _, p := range providers {
		data = append(data, p.Name())
	}

	JSON(w, r, http.StatusOK, M{"providers": data})
}

func (h *AuthHandler[Ident, ID]) provider(w http.ResponseWriter, r *http.Request) {
	gothic.BeginAuthHandler(w, gothic.GetContextWithProvider(r, chi.URLParam(r, "provider")))
}

func (h *AuthHandler[Ident, ID]) callback(w http.ResponseWriter, r *http.Request) {
	guser, err := gothic.CompleteUserAuth(w, gothic.GetContextWithProvider(r, chi.URLParam(r, "provider")))
	if err != nil {
		h.errorHandler(w, r, err)
		return
	}

	id, err := h.Auth.Set(r.Context(), &guser)
	if err != nil {
		h.errorHandler(w, r, err)
		return
	}

	if err := gothic.StoreInSession(authSessionKey, fmt.Sprintf("%v", id), r, w); err != nil {
		h.errorHandler(w, r, err)
		return
	}
	SecureRedirect(w, r, http.StatusTemporaryRedirect, "/")
}

func (h *AuthHandler[Ident, ID]) logout(w http.ResponseWriter, r *http.Request) {
	_ = gothic.Logout(w, r)
	SecureRedirect(w, r, http.StatusFound, "/")
}

func (h *AuthHandler[Ident, ID]) self(w http.ResponseWriter, r *http.Request) {
	JSON(w, r, http.StatusOK, M{"auth": h.FromContext(r.Context())})
}

// convertID converts the string stored in session cookies, to the ID type
// provided by the caller. Only basic types are currently supported.
func (h *AuthHandler[Ident, ID]) convertID(in string) (ID, error) {
	var v any
	var err error

	switch any(h.ID).(type) {
	case *string:
		v = in
	case *int:
		v, err = strconv.Atoi(in)
	case *int64:
		v, err = strconv.ParseInt(in, 10, 64)
	case *float64:
		v, err = strconv.ParseFloat(in, 64)
	case *uint:
		v, err = strconv.ParseUint(in, 10, 64)
	case *uint16:
		v, err = strconv.ParseUint(in, 10, 16)
	case *uint32:
		v, err = strconv.ParseUint(in, 10, 32)
	case *uint64:
		v, err = strconv.ParseUint(in, 10, 64)
	default:
		panic("unsupported ID type")
	}
	if err != nil {
		return *new(ID), err
	}
	return v.(ID), nil
}

// getIDFromSession returns the ID from the session cookie.
func (h *AuthHandler[Ident, ID]) getIDFromSession(r *http.Request) *ID {
	key, _ := gothic.GetFromSession(authSessionKey, r)
	if key == "" {
		return nil
	}

	id, err := h.convertID(key)
	if err != nil {
		return nil
	}
	return &id
}

// FromContext returns the user authentication info from the request context.
// Note that this will only work if the AddToContext() middleware has been
// loaded prior to this middleware.
func (h *AuthHandler[Ident, ID]) FromContext(ctx context.Context) (auth *Ident) {
	auth, _ = ctx.Value(contextAuth).(*Ident)
	return auth
}

// RolesFromContext returns the user roles from the request context. Note that
// this will only work if the AddToContext() middleware has been loaded prior
// to this middleware. You can also call RolesFromContext (non-AuthHandler),
// which will return the same information, for use where you can't easily
// inject the AuthHandler.
func (h *AuthHandler[Ident, ID]) RolesFromContext(ctx context.Context) (roles AuthRoles) {
	return RolesFromContext(ctx)
}

// AddToContext adds the user authentication info to the request context, using
// the cookie session information. If used more than once in the same request
// middleware chain, it will be a no-op.
func (h *AuthHandler[Ident, ID]) AddToContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := r.Context().Value(contextAuth).(*Ident)
		if ok { // Already in the context.
			next.ServeHTTP(w, r)
			return
		}

		id := h.getIDFromSession(r)
		if id == nil {
			next.ServeHTTP(w, r)
			return
		}

		ident, err := h.Auth.Get(r.Context(), *id)
		if err != nil {
			Log(r).WithError(err).WithField("user_id", *id).Warn("failed to get ident from session (but id set)")
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), contextAuth, ident)
		ctx = context.WithValue(ctx, contextAuthID, *id)

		roles, err := h.Auth.Roles(r.Context(), *id)
		if err != nil {
			Log(r).WithError(err).WithField("user_id", *id).Warn("failed to get roles from session (but id set)")
		} else {
			ctx = context.WithValue(ctx, contextAuthRoles, roles)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthRequired is a middleware that requires the user to be authenticated.
// Note that this requires the AddToContext() middleware to be loaded prior to
// this middleware.
func (h *AuthHandler[Ident, ID]) AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := r.Context().Value(contextAuth).(*Ident)
		if ok { // Already in the context.
			next.ServeHTTP(w, r)
			return
		}

		_ = Error(w, r, WrapCode(http.StatusUnauthorized))
	})
}

// RolesRequired is a middleware that requires the user to have the given roles,
// provided via AuthService.Roles(). Note that this requires the AddToContext()
// middleware to be loaded prior to this middleware.
func (h *AuthHandler[Ident, ID]) RoleRequired(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := h.getIDFromSession(r)
			if id == nil {
				if role == "anonymous" {
					next.ServeHTTP(w, r)
					return
				}

				_ = Error(w, r, WrapError(ErrAuthMissingRole, http.StatusUnauthorized))
				return
			}

			roles, err := h.Auth.Roles(r.Context(), *id)
			if err != nil {
				_ = Error(w, r, err)
				return
			}

			for _, roleName := range roles {
				if roleName == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			_ = Error(w, r, WrapError(ErrAuthMissingRole, http.StatusUnauthorized))
		})
	}
}

// RolesFromContext returns the user roles from the request context, if any.
// Note that this will only work if the AddToContext() middleware has been
// loaded, and the user is authenticated.
func RolesFromContext(ctx context.Context) (roles AuthRoles) {
	roles, _ = ctx.Value(contextAuthRoles).([]string)
	return AuthRoles(roles)
}

// IDFromContext returns the user ID from the request context, if any. Note that
// this will only work if the AddToContext() middleware has been loaded, and the
// user is authenticated.
//
// Returns 0 if the user is not authenticated or the ID was not found in the
// context.
func IDFromContext[ID comparable](ctx context.Context) (id ID) {
	id, _ = ctx.Value(contextAuthID).(ID)
	return id
}

// IdentFromContext returns the ident from the request context, if any. Note that
// this will only work if the AddToContext() middleware has been loaded, and the
// user is authenticated. Provided Ident type MUST match what is used in AuthHandler.
//
// Returns nil if the user is not authenticated or the ident was not found in the
// context.
func IdentFromContext[Ident any](ctx context.Context) (auth *Ident) {
	auth, _ = ctx.Value(contextAuth).(*Ident)
	return auth
}

// AuthRoles provides helper methods for working with roles.
type AuthRoles []string

// Has returns true if the given role is present for the authenticated identity
// in the context.
func (r AuthRoles) Has(role string) bool {
	if len(r) == 0 {
		return false
	}

	for _, r := range r {
		if strings.EqualFold(r, role) {
			return true
		}
	}

	return false
}
