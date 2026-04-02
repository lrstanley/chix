// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"embed"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestStaticConfig_Validate(t *testing.T) {
	t.Parallel()

	t.Run("nil-config", func(t *testing.T) {
		t.Parallel()
		var c *StaticConfig
		err := c.Validate()
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "nil") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("nil-fs", func(t *testing.T) {
		t.Parallel()
		c := &StaticConfig{}
		err := c.Validate()
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "FS") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("embedded-fs-subpath", func(t *testing.T) {
		t.Parallel()
		c := &StaticConfig{
			FS:   spaFS,
			Path: "testdata/static/spa",
		}
		if err := c.Validate(); err != nil {
			t.Fatal(err)
		}
	})
}

//go:embed all:testdata/static/spa
var spaFS embed.FS

//go:embed all:testdata/static/non-spa
var nonSpaFS embed.FS

func TestUseStatic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		config          *StaticConfig
		requestPath     string
		requestMethod   string
		setupRouter     func(*StaticConfig) *chi.Mux
		expectedStatus  int
		bodyContains    string
		bodyNotContains string
	}{
		// SPA Mode Tests
		{
			name: "spa-fetch-index-via-root",
			config: &StaticConfig{
				FS:   spaFS,
				Path: "testdata/static/spa",
				SPA:  true,
			},
			requestPath:    "/",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			bodyContains:   "SPA Index Page",
		},
		{
			name: "spa-fetch-index-via-non-existent-path",
			config: &StaticConfig{
				FS:   spaFS,
				Path: "testdata/static/spa",
				SPA:  true,
			},
			requestPath:    "/example/page",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			bodyContains:   "SPA Index Page",
		},
		{
			name: "spa-missing-js-file-returns-404",
			config: &StaticConfig{
				FS:   spaFS,
				Path: "testdata/static/spa",
				SPA:  true,
			},
			requestPath:     "/missing.js",
			requestMethod:   http.MethodGet,
			expectedStatus:  http.StatusNotFound,
			bodyNotContains: "SPA Index Page",
		},
		{
			name: "spa-missing-css-file-returns-404",
			config: &StaticConfig{
				FS:   spaFS,
				Path: "testdata/static/spa",
				SPA:  true,
			},
			requestPath:     "/missing.css",
			requestMethod:   http.MethodGet,
			expectedStatus:  http.StatusNotFound,
			bodyNotContains: "SPA Index Page",
		},
		{
			name: "spa-missing-png-file-returns-404",
			config: &StaticConfig{
				FS:   spaFS,
				Path: "testdata/static/spa",
				SPA:  true,
			},
			requestPath:     "/missing.png",
			requestMethod:   http.MethodGet,
			expectedStatus:  http.StatusNotFound,
			bodyNotContains: "SPA Index Page",
		},
		{
			name: "spa-missing-ts-file-returns-404",
			config: &StaticConfig{
				FS:   spaFS,
				Path: "testdata/static/spa",
				SPA:  true,
			},
			requestPath:     "/missing.ts",
			requestMethod:   http.MethodGet,
			expectedStatus:  http.StatusNotFound,
			bodyNotContains: "SPA Index Page",
		},
		{
			name: "spa-existing-js-file-returns-file",
			config: &StaticConfig{
				FS:   spaFS,
				Path: "testdata/static/spa",
				SPA:  true,
			},
			requestPath:    "/app.js",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			bodyContains:   "console.log",
		},
		{
			name: "spa-custom-path-subdirectory",
			config: &StaticConfig{
				FS:   spaFS,
				Path: "testdata/static/spa/subpath",
				SPA:  true,
			},
			requestPath:    "/",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			bodyContains:   "SPA Subpath Index",
		},
		{
			name: "spa-custom-prefix-mount-point",
			config: &StaticConfig{
				FS:     spaFS,
				Path:   "testdata/static/spa",
				Prefix: "/static",
				SPA:    true,
			},
			requestPath:    "/static/",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			bodyContains:   "SPA Index Page",
		},
		{
			name: "spa-custom-prefix-with-non-existent-path",
			config: &StaticConfig{
				FS:     spaFS,
				Path:   "testdata/static/spa",
				Prefix: "/static",
				SPA:    true,
			},
			requestPath:    "/static/example/page",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			bodyContains:   "SPA Index Page",
		},
		{
			name: "spa-custom-prefix-with-missing-asset",
			config: &StaticConfig{
				FS:     spaFS,
				Path:   "testdata/static/spa",
				Prefix: "/static",
				SPA:    true,
			},
			requestPath:     "/static/missing.js",
			requestMethod:   http.MethodGet,
			expectedStatus:  http.StatusNotFound,
			bodyNotContains: "SPA Index Page",
		},
		// Non-SPA Mode Tests
		{
			name: "non-spa-existing-file-at-root-loads-correctly",
			config: &StaticConfig{
				FS:   nonSpaFS,
				Path: "testdata/static/non-spa",
				SPA:  false,
			},
			requestPath:    "/file.txt",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			bodyContains:   "root file content",
		},
		{
			name: "non-spa-existing-file-in-subdirectory-loads-correctly",
			config: &StaticConfig{
				FS:   nonSpaFS,
				Path: "testdata/static/non-spa",
				SPA:  false,
			},
			requestPath:    "/subdir/file.txt",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			bodyContains:   "subdirectory file content",
		},
		{
			name: "non-spa-non-existent-file-returns-404",
			config: &StaticConfig{
				FS:   nonSpaFS,
				Path: "testdata/static/non-spa",
				SPA:  false,
			},
			requestPath:    "/missing.txt",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "non-spa-index-html-loads-correctly-via-root",
			config: &StaticConfig{
				FS:   nonSpaFS,
				Path: "testdata/static/non-spa",
				SPA:  false,
			},
			requestPath:    "/",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			bodyContains:   "Non-SPA Index Page",
		},
		{
			name: "non-spa-custom-path-subdirectory",
			config: &StaticConfig{
				FS:   nonSpaFS,
				Path: "testdata/static/non-spa/subpath",
				SPA:  false,
			},
			requestPath:    "/file.txt",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			bodyContains:   "non-spa subpath file content",
		},
		{
			name: "non-spa-custom-prefix-mount-point",
			config: &StaticConfig{
				FS:     nonSpaFS,
				Path:   "testdata/static/non-spa",
				Prefix: "/static",
				SPA:    false,
			},
			requestPath:    "/static/file.txt",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			bodyContains:   "root file content",
		},
		{
			name: "non-spa-custom-prefix-with-subdirectory",
			config: &StaticConfig{
				FS:     nonSpaFS,
				Path:   "testdata/static/non-spa",
				Prefix: "/static",
				SPA:    false,
			},
			requestPath:    "/static/subdir/file.txt",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			bodyContains:   "subdirectory file content",
		},
		{
			name: "non-spa-custom-prefix-with-non-existent-file",
			config: &StaticConfig{
				FS:     nonSpaFS,
				Path:   "testdata/static/non-spa",
				Prefix: "/static",
				SPA:    false,
			},
			requestPath:    "/static/missing.txt",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusNotFound,
		},
		// API endpoint routing tests
		{
			name: "spa-api-endpoint-not-routed-to-static",
			config: &StaticConfig{
				FS:     spaFS,
				Path:   "testdata/static/spa",
				Prefix: "",
				SPA:    true,
			},
			requestPath:   "/api/users",
			requestMethod: http.MethodGet,
			setupRouter: func(config *StaticConfig) *chi.Mux {
				cfg := NewConfig().SetAPIBasePath("/api")
				router := chi.NewRouter()
				router.Use(cfg.Use())
				router.Mount("/", UseStatic(config))
				router.Get("/api/users", func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("api response"))
				})
				return router
			},
			expectedStatus:  http.StatusOK,
			bodyContains:    "api response",
			bodyNotContains: "SPA Index Page",
		},
		{
			name: "spa-api-endpoint-with-prefix-not-routed-to-static",
			config: &StaticConfig{
				FS:     spaFS,
				Path:   "testdata/static/spa",
				Prefix: "/static",
				SPA:    true,
			},
			requestPath:   "/api/users",
			requestMethod: http.MethodGet,
			setupRouter: func(config *StaticConfig) *chi.Mux {
				cfg := NewConfig().SetAPIBasePath("/api")
				router := chi.NewRouter()
				router.Use(cfg.Use())
				router.Mount("/static", UseStatic(config))
				router.Get("/api/users", func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("api response"))
				})
				return router
			},
			expectedStatus:  http.StatusOK,
			bodyContains:    "api response",
			bodyNotContains: "SPA Index Page",
		},
		{
			name: "non-spa-api-endpoint-not-routed-to-static",
			config: &StaticConfig{
				FS:     nonSpaFS,
				Path:   "testdata/static/non-spa",
				Prefix: "",
				SPA:    false,
			},
			requestPath:   "/api/users",
			requestMethod: http.MethodGet,
			setupRouter: func(config *StaticConfig) *chi.Mux {
				cfg := NewConfig().SetAPIBasePath("/api")
				router := chi.NewRouter()
				router.Use(cfg.Use())
				router.Mount("/", UseStatic(config))
				router.Get("/api/users", func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("api response"))
				})
				return router
			},
			expectedStatus: http.StatusOK,
			bodyContains:   "api response",
		},
		{
			name: "non-spa-api-endpoint-with-prefix-not-routed-to-static",
			config: &StaticConfig{
				FS:     nonSpaFS,
				Path:   "testdata/static/non-spa",
				Prefix: "/static",
				SPA:    false,
			},
			requestPath:   "/api/users",
			requestMethod: http.MethodGet,
			setupRouter: func(config *StaticConfig) *chi.Mux {
				cfg := NewConfig().SetAPIBasePath("/api")
				router := chi.NewRouter()
				router.Use(cfg.Use())
				router.Mount("/static", UseStatic(config))
				router.Get("/api/users", func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("api response"))
				})
				return router
			},
			expectedStatus: http.StatusOK,
			bodyContains:   "api response",
		},
		// Catch-all (SPA): API path under API base returns 404, not fallback HTML.
		{
			name: "spa-catchall-api-path-not-serve-fallback",
			config: &StaticConfig{
				FS:       spaFS,
				Path:     "testdata/static/spa",
				SPA:      true,
				CatchAll: true,
			},
			requestPath:   "/api/users",
			requestMethod: http.MethodGet,
			setupRouter: func(config *StaticConfig) *chi.Mux {
				cfg := NewConfig().SetAPIBasePath("/api")
				router := chi.NewRouter()
				router.Use(cfg.Use())
				router.Mount("/", UseStatic(config))
				return router
			},
			expectedStatus:  http.StatusNotFound,
			bodyNotContains: "SPA Index Page",
		},
		{
			name: "spa-catchall-non-get-method-not-allowed",
			config: &StaticConfig{
				FS:       spaFS,
				Path:     "testdata/static/spa",
				SPA:      true,
				CatchAll: true,
			},
			requestPath:   "/any/route",
			requestMethod: http.MethodPost,
			setupRouter: func(config *StaticConfig) *chi.Mux {
				cfg := NewConfig().SetAPIBasePath("/api")
				router := chi.NewRouter()
				router.Use(cfg.Use())
				router.Mount("/", UseStatic(config))
				return router
			},
			expectedStatus:  http.StatusMethodNotAllowed,
			bodyNotContains: "SPA Index Page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var router *chi.Mux
			if tt.setupRouter != nil {
				router = tt.setupRouter(tt.config)
			} else {
				router = chi.NewRouter()
				mountPoint := tt.config.Prefix
				if mountPoint == "" {
					mountPoint = "/"
				}
				router.Mount(mountPoint, UseStatic(tt.config))
			}

			req := httptest.NewRequest(tt.requestMethod, "http://example.com"+tt.requestPath, http.NoBody)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status code %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read response body: %v", err)
			}
			bodyStr := string(body)

			if tt.bodyContains != "" && !strings.Contains(bodyStr, tt.bodyContains) {
				t.Errorf("expected body to contain %q, got %q", tt.bodyContains, bodyStr)
			}

			if tt.bodyNotContains != "" && strings.Contains(bodyStr, tt.bodyNotContains) {
				t.Errorf("expected body to not contain %q, got %q", tt.bodyNotContains, bodyStr)
			}
		})
	}
}
