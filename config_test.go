// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newMockRouter(t testing.TB, middleware []func(http.Handler) http.Handler) *chi.Mux {
	t.Helper()

	router := chi.NewRouter()
	router.Use(middleware...)

	router.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello, world!"))
	})

	router.Get("/json", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, r, http.StatusOK, map[string]any{
			"foo": strings.Repeat("foo bar baz abc1234567890\n", 100),
			"baz": 123,
			"qux": []string{"foo", "bar", "baz"},
			"quz": map[string]any{
				"foo": "bar",
				"baz": 123,
				"qux": []string{"foo", "bar", "baz"},
			},
			"message": "Hello, world!",
		})
	})

	router.Get("/json-large", func(w http.ResponseWriter, r *http.Request) {
		data := []struct {
			Foo string
			Bar int
			Baz string
		}{
			{
				Foo: "foo bar baz abc1234567890\n",
				Bar: 123,
				Baz: "qux",
			},
		}
		JSON(w, r, http.StatusOK, slices.Repeat(data, 10000))
	})

	router.Get("/panic", func(_ http.ResponseWriter, _ *http.Request) {
		panic("intentional panic")
	})

	router.Get("/error", func(w http.ResponseWriter, r *http.Request) {
		ErrorWithCode(w, r, http.StatusInternalServerError, errors.New("intentional error"))
	})

	router.Get("/errors", func(w http.ResponseWriter, r *http.Request) {
		ErrorWithCode(
			w, r, http.StatusInternalServerError,
			errors.New("intentional error"),
			errors.New("another error"),
		)
	})

	router.NotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ErrorWithCode(w, r, http.StatusNotFound)
	}))

	return router
}
