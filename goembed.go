// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/apex/log"
)

// UseStatic returns a handler that serves static files from the provided embedded
// filesystem, with support for using the direct filesystem when debugging is
// enabled.
//
// Example usage:
//
//	//go:embed all:public/dist
//	var publicDist embed.FS
//	[...]
//	router.Mount("/static", chix.UseStatic(&chix.Static{
//		FS:         publicDist,
//		Prefix:     "/static",
//		AllowLocal: true,
//		Path:       "public/dist"
//	}))
func UseStatic(ctx context.Context, config *Static) http.Handler {
	logger := log.FromContext(ctx)

	var err error

	if config == nil {
		panic("config is nil")
	}

	if config.FS == nil {
		panic("config.FS is nil")
	}

	if config.LocalPath == "" {
		config.LocalPath = config.Path
	}

	config.Path = strings.Trim(config.Path, "/")
	config.LocalPath = strings.Trim(config.LocalPath, "/")

	if config.AllowLocal && config.LocalPath == "" {
		panic("config.AllowLocal is true, but config.LocalPath and config.Path is empty")
	}

	if config.Path != "" {
		config.FS, err = fs.Sub(config.FS, config.Path)
		if err != nil {
			panic(err)
		}
	}

	cwdLocal, _ := os.Stat(config.LocalPath)
	exeLocal, _ := os.Stat(path.Join(os.Args[0], config.LocalPath))

	if config.AllowLocal && cwdLocal != nil && cwdLocal.IsDir() {
		config.httpFS = http.Dir(config.LocalPath)
		logger.WithField("path", config.LocalPath).Debug("registering static assets in current working directory")
	} else if config.AllowLocal && exeLocal != nil && exeLocal.IsDir() {
		config.LocalPath = path.Join(os.Args[0], config.LocalPath)
		config.httpFS = http.Dir(config.LocalPath)
		logger.WithField("path", config.LocalPath).Debug("registering static assets in executable directory")
	} else {
		logger.WithField("path", config.Path).Debug("registering embedded static assets")
		walkPath := config.Path
		if config.Path == "" {
			walkPath = "."
		}

		_ = fs.WalkDir(config.FS, walkPath, func(path string, info fs.DirEntry, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}

			logger.Debugf("registering embedded asset: %v", path)
			return nil
		})
		config.httpFS = http.FS(config.FS)
	}

	config.handler = http.FileServer(config.httpFS)

	if config.Prefix != "" {
		// Don't wrap the internal handler, as any logic we do, we want the prefix
		// to be stripped first.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, config.Prefix)
			r.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, config.Prefix)
			config.ServeHTTP(w, r)
		})
	}
	return config
}

// Static is an http.Handler that serves static files from an embedded filesystem.
// See chix.UseStatic() for more information.
type Static struct {
	// fs is the filesystem to serve.
	FS fs.FS

	// Prefix is the prefix where the filesystem is mounted on your http router.
	Prefix string

	// CatchAll is a boolean that determines if chix.Static is being used as a
	// catch-all for not-found routes. If so, it will do extra validations for
	// using chix.Error when the route is related to an API endpoint (see
	// chix.DefaultAPIPrefix), as well as enforce specific methods.
	CatchAll bool

	// AllowLocal is a boolean that, if true, and chix.LocalPath exists, it will
	// bypass the provided filesystem and instead use the actual filesystem.
	AllowLocal bool

	// LocalPath is the subpath to use when AllowLocal is enabled. If empty, it
	// will default to Static.Path. It will check for this sub-directory in either
	// the current working directory, or the executable directory.
	LocalPath string

	// Path of the embedded filesystem, instead of the entire filesystem. go:embed
	// will include the target that gets embedded, as a prefix to the path.
	//
	// For example, given "go:embed all:public/dist", mounted at "/static", you
	// would normally have to access using "/static/public/dist/<etc>". Providing
	// path, where path is "public/dist", you can access the same files
	// via "/static/<etc>".
	Path string

	// SPA is a boolean that, if true, will serve a single page application, i.e.
	// redirecting all files not found, to the index.html file.
	SPA bool

	// Headers is a map of headers to set on the response (e.g. cache headers).
	// Example:
	//	&chix.Static{
	//		[...]
	//		Headers: map[string]string{
	//			"Vary": "Accept-Encoding",
	//			"Cache-Control": "public, max-age=7776000",
	//		},
	//	}
	Headers map[string]string

	httpFS  http.FileSystem
	handler http.Handler
}

func (s *Static) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.CatchAll {
		if strings.HasPrefix(r.URL.Path, DefaultAPIPrefix) {
			Error(w, r, WrapCode(http.StatusNotFound))
			return
		}

		if r.Method != http.MethodGet {
			Error(w, r, WrapCode(http.StatusMethodNotAllowed))
			return
		}
	}

	// Handle custom headers, if any.
	if s.Headers != nil {
		for k, v := range s.Headers {
			w.Header().Set(k, v)
		}
	}

	// Handle SPA, if enabled.
	if s.SPA {
		if !strings.HasPrefix(r.URL.Path, "/") {
			r.URL.Path = "/" + r.URL.Path
		}

		f, err := s.httpFS.Open(path.Clean(r.URL.Path))
		if err != nil && errors.Is(err, fs.ErrNotExist) {
			r.URL.Path = "/"
		}
		if f != nil {
			_ = f.Close()
		}
	}

	s.handler.ServeHTTP(w, r)
}
