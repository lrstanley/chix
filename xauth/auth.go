// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package xauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/sessions"
	"github.com/lrstanley/chix/v2"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

const authSessionKey = "_auth"

var gothSessionStoreOnce sync.Once

var _ ServiceReader[int, string] = (Service[int, string])(nil) // Ensure [Service] implements [ServiceReader].

// ServiceReader is the read-only variant of the [Service] interface.
type ServiceReader[Ident any, ID comparable] interface {
	Get(context.Context, ID) (*Ident, error)
}

// Service is the interface for the authentication service.
type Service[Ident any, ID comparable] interface {
	Get(context.Context, ID) (*Ident, error)
	Set(context.Context, *goth.User) (ID, error)
}

// getAuthIDFromSession returns the ID from the session cookie. Behind the scenes,
// this converts the string stored in session cookies, to the ID type provided
// by the caller. Only basic types are currently supported.
func getAuthIDFromSession[ID comparable](r *http.Request) *ID {
	key, _ := gothic.GetFromSession(authSessionKey, r)
	if key == "" {
		return nil
	}

	var id ID
	var v any
	var err error

	switch any(&id).(type) {
	case *string:
		v = key
	case *int:
		v, err = strconv.Atoi(key)
	case *int64:
		v, err = strconv.ParseInt(key, 10, 64)
	case *float64:
		v, err = strconv.ParseFloat(key, 64)
	case *uint:
		v, err = strconv.ParseUint(key, 10, 64)
	case *uint16:
		v, err = strconv.ParseUint(key, 10, 16)
	case *uint32:
		v, err = strconv.ParseUint(key, 10, 32)
	case *uint64:
		v, err = strconv.ParseUint(key, 10, 64)
	default:
		panic("unsupported ID type")
	}
	if err != nil {
		return nil
	}

	id, _ = v.(ID)
	return &id
}

type (
	contextKeyAuth   struct{}
	contextKeyAuthID struct{}
)

func setContextAuth[Ident any](ctx context.Context, ident *Ident) context.Context {
	return context.WithValue(ctx, contextKeyAuth{}, ident)
}

func setContextAuthID[ID comparable](ctx context.Context, id ID) context.Context {
	return context.WithValue(ctx, contextKeyAuthID{}, id)
}

// OverrideContextAuth overrides the authentication information in the request,
// and returns a new context with the updated information. This is useful for
// when you want to temporarily override the authentication information in the
// request, such as when you want to impersonate another user, or for mocking in
// tests.
func OverrideContextAuth[Ident any, ID comparable](ctx context.Context, id ID, ident *Ident) context.Context {
	return setContextAuth(setContextAuthID(ctx, id), ident)
}

// UseAuthContext adds the user authentication info to the request context, using
// the cookie session information. If used more than once in the same request
// middleware chain, it will be a no-op. This will also add logging attributes
// through [github.com/lrstanley/chix/v2/chix.AppendLogAttrs] for the user.
func UseAuthContext[Ident any, ID comparable, Service ServiceReader[Ident, ID]](auth Service) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if IdentFromContext[Ident](ctx) != nil {
				next.ServeHTTP(w, r)
				return
			}

			id := getAuthIDFromSession[ID](r)
			if id == nil {
				next.ServeHTTP(w, r)
				return
			}

			ident, err := auth.Get(ctx, *id)
			if err != nil {
				chix.LogWarn(
					ctx,
					"failed to get ident from session (but id set)",
					slog.Any("auth_id", *id),
					slog.Any("error", err),
				)
				next.ServeHTTP(w, r)
				return
			}

			chix.AppendLogAttrs(
				ctx,
				slog.Any("auth", ident),
				slog.Any("auth_id", *id),
			)
			next.ServeHTTP(w, r.WithContext(setContextAuth(setContextAuthID(ctx, *id), ident)))
		})
	}
}

// IDFromContext returns the user ID from the request context, if any. Note that
// this will only work if the [UseAuthContext] middleware has been loaded, and the
// user is authenticated.
//
// Returns 0 if the user is not authenticated or the ID was not found in the
// context.
func IDFromContext[ID comparable](ctx context.Context) (id ID) {
	id, _ = ctx.Value(contextKeyAuthID{}).(ID)
	return id
}

// IdentFromContext returns the ident from the request context, if any. Note that
// this will only work if the [UseAuthContext] middleware has been loaded, and the
// user is authenticated. Provided Ident type MUST match what is used in AuthHandler.
//
// Returns nil if the user is not authenticated or the ident was not found in the
// context.
func IdentFromContext[Ident any](ctx context.Context) (auth *Ident) {
	auth, _ = ctx.Value(contextKeyAuth{}).(*Ident)
	return auth
}

// UseAuthRequired is a middleware that requires the user to be authenticated.
// Note that this requires the [UseAuthContext] middleware to be loaded prior to
// this middleware.
func UseAuthRequired[Ident any]() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if IdentFromContext[Ident](r.Context()) != nil {
				next.ServeHTTP(w, r)
				return
			}
			chix.ErrorWithCode(w, r, http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized)))
		})
	}
}

// NewCookieStore creates a new session storage that uses encrypted cookies. authKey
// is used to validate the session cookie, and encryptKey is used to encrypt the
// session cookie.
//
// It is recommended to use an authentication key with 32 or 64 bytes. The
// encryption key, if set, must be either 16, 24, or 32 bytes to select
// AES-128, AES-192, or AES-256 modes. Provide the keys in hexadecimal string
// format. The following link can be used to generate a random key:
//   - https://go.dev/play/p/xwcJmQNU8ku
//
// The same could be done using bash (on linux), for example:
//
//	head -c 64 /dev/urandom | xxd -p -c 64 # 64 bytes, auth key
//	head -c 32 /dev/urandom | xxd -p -c 32 # 32 bytes, encryption key
func NewCookieStore(authKey, encryptKey string) *sessions.CookieStore {
	if authKey == "" || encryptKey == "" {
		panic("authKey or encryptKey is empty")
	}
	authKeyBytes, err := hex.DecodeString(authKey)
	if err != nil {
		panic(err)
	}
	encryptKeyBytes, err := hex.DecodeString(encryptKey)
	if err != nil {
		panic(err)
	}
	store := sessions.NewCookieStore(authKeyBytes, encryptKeyBytes)
	store.MaxAge(int((30 * 24 * time.Hour).Seconds()))
	store.Options.Path = "/"
	store.Options.HttpOnly = true
	store.Options.SameSite = http.SameSiteLaxMode
	store.Options.Partitioned = true
	return store
}

// GenerateAuthKey generates a random authentication key. Will panic if the random
// number generator fails, which should generally only happen on systems with an
// insecure random number generator (which means you probably shouldn't be using
// that system for anything security related).
//
// The same could be done using bash (on linux), for example:
//
//	head -c 64 /dev/urandom | xxd -p -c 64
func GenerateAuthKey() string {
	k := make([]byte, 64)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		panic(fmt.Errorf("failed to generate auth key: %w", err))
	}
	return hex.EncodeToString(k)
}

// GenerateEncryptionKey generates a random encryption key. Will panic if the random
// number generator fails, which should generally only happen on systems with an
// insecure random number generator (which means you probably shouldn't be using
// that system for anything security related).
//
// The same could be done using bash (on linux), for example:
//
//	head -c 32 /dev/urandom | xxd -p -c 32
func GenerateEncryptionKey() string {
	k := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		panic(fmt.Errorf("failed to generate encryption key: %w", err))
	}
	return hex.EncodeToString(k)
}
