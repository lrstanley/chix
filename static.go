// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// StaticConfig is a [net/http.Handler] that serves static files from an embedded
// filesystem. See [UseStatic] for more information. It also supports serviing
// Single Page Applications (SPA) by redirecting all files not found, to the
// index.html file, if configured through [StaticConfig.SPA].
type StaticConfig struct {
	// fs is the filesystem to serve. Note that setting this to anything but an
	// embedded filesystem, [os.OpenRoot] should be used.
	FS fs.FS

	// Prefix is the prefix where the filesystem is mounted on your http router.
	Prefix string

	// CatchAll is a boolean that determines if [StaticConfig] is being used as a
	// catch-all for not-found routes. If so, it will do extra validations for
	// using [Error] when the route is related to an API endpoint (see
	// [Config.APIBasePath]), as well as enforce specific methods.
	CatchAll bool

	// AllowLocal is a boolean that, if true, and [StaticConfig.LocalPath] exists,
	// it will bypass the provided filesystem and instead use the actual filesystem.
	AllowLocal bool

	// LocalPath is the subpath to use when [StaticConfig.AllowLocal] is enabled. If
	// empty, it will default to [StaticConfig.Path]. It will check for this
	// sub-directory in either the current working directory, or the executable
	// directory.
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
}

// Validate validates the static config. Use this to validate the config before using
// it, otherwise [UseStatic] will panic if an invalid config is provided.
func (c *StaticConfig) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}

	if c.FS == nil {
		return errors.New("FS is nil")
	}

	if c.LocalPath == "" {
		c.LocalPath = c.Path
	}

	c.Path = strings.Trim(c.Path, "/")
	c.LocalPath = strings.Trim(c.LocalPath, "/")

	if c.AllowLocal && c.LocalPath == "" {
		c.AllowLocal = false
	}

	var err error

	if c.Path != "" {
		c.FS, err = fs.Sub(c.FS, c.Path)
		if err != nil {
			return fmt.Errorf("failed to use subdirectory of filesystem: %w", err)
		}
	}

	if c.FS != nil {
		return nil
	}

	_, srcPath, _, _ := runtime.Caller(1)
	srcPath = path.Join(filepath.Dir(srcPath), c.LocalPath)

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exePath = path.Join(filepath.Dir(exePath), c.LocalPath)

	cwdLocal, _ := os.Stat(c.LocalPath) // Path to the current working directory.
	srcLocal, _ := os.Stat(srcPath)     // Path to source file, if it's still on the filesystem.
	exeLocal, _ := os.Stat(exePath)     // Path to the current executable.

	switch {
	case c.AllowLocal && cwdLocal != nil && cwdLocal.IsDir():
		var root *os.Root
		root, err = os.OpenRoot(c.LocalPath)
		if err != nil {
			return fmt.Errorf("failed to open root: %w", err)
		}
		c.FS = root.FS()
	case c.AllowLocal && srcLocal != nil && srcLocal.IsDir():
		c.LocalPath = srcPath
		var root *os.Root
		root, err = os.OpenRoot(c.LocalPath)
		if err != nil {
			return fmt.Errorf("failed to open root: %w", err)
		}
		c.FS = root.FS()
	case c.AllowLocal && exeLocal != nil && exeLocal.IsDir():
		c.LocalPath = exePath
		var root *os.Root
		root, err = os.OpenRoot(c.LocalPath)
		if err != nil {
			return fmt.Errorf("failed to open root: %w", err)
		}
		c.FS = root.FS()
	}

	if c.FS == nil {
		return errors.New("failed to find static assets")
	}

	return nil
}

// UseStatic returns a handler that serves static files from the provided embedded
// filesystem, with support for using the direct filesystem when debugging is
// enabled. It also supports serviing Single Page Applications (SPA) by redirecting
// all files not found, to the index.html file, if configured through
// [StaticConfig.SPA].
//
// Example usage:
//
//	//go:embed all:public/dist
//	var publicDist embed.FS
//	[...]
//	router.Mount("/static", chix.UseStatic(&chix.StaticConfig{
//		FS:         publicDist,
//		Prefix:     "/static",
//		AllowLocal: true,
//		Path:       "public/dist"
//	}))
func UseStatic(config *StaticConfig) http.Handler { //nolint:gocognit,funlen
	if err := config.Validate(); err != nil {
		panic(err)
	}

	httpFS := http.FS(config.FS)
	fsHandler := http.FileServer(httpFS)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if config.CatchAll {
			if strings.HasPrefix(r.URL.Path, GetConfig(r.Context()).GetAPIBasePath()) {
				ErrorWithCode(w, r, http.StatusNotFound, errors.New("resource not found"))
				return
			}

			if r.Method != http.MethodGet {
				ErrorWithCode(w, r, http.StatusMethodNotAllowed)
				return
			}
		}

		if !config.SPA {
			fsHandler.ServeHTTP(w, r)
			return
		}

		if !strings.HasPrefix(r.URL.Path, "/") {
			r.URL.Path = "/" + r.URL.Path
		}

		f, err := httpFS.Open(path.Clean(r.URL.Path))
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				ErrorWithCode(w, r, http.StatusInternalServerError, err)
				return
			}

			// If the requested route has an extension, try and see if it matches
			// any mime types that aren't text/html, and if so, explicitly return a 404. This
			// isn't perfect, but it's a good enough heuristic to avoid serving the index.html file
			// for non-HTML routes and causing oddities with things like /favicon.ico when it doesn't
			// exist.
			if mime := mime.TypeByExtension(path.Ext(r.URL.Path)); mime != "" && mime != "text/html" {
				ErrorWithCode(w, r, http.StatusNotFound, errors.New("resource not found"))
				return
			}

			r.URL.Path = "/"
			fsHandler.ServeHTTP(w, r)
			return
		}

		// Check if the path is a sub-directory, and if so, still load the index.html file,
		// even though the directory exists.
		var stat fs.FileInfo
		stat, err = f.Stat()
		if err != nil {
			ErrorWithCode(w, r, http.StatusInternalServerError, err)
			return
		}
		if stat.IsDir() {
			r.URL.Path = "/"
		}

		_ = f.Close()
		fsHandler.ServeHTTP(w, r)
	})

	if config.Prefix != "" {
		// Don't wrap the internal handler, as any logic we do, we want the prefix
		// to be stripped first.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, config.Prefix)
			r.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, config.Prefix)
			handler.ServeHTTP(w, r)
		})
	}

	return handler
}
