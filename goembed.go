// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"io/fs"
	"net/http"
	"strings"
)

// TODO: if debugging is enabled, try and read from the actual filesystem
// first.
// TODO: if debugging, walk filesystem and log a list of all files.

// UseStatic returns a handler that serves static files from the provided embedded
// filesystem.
//
// Example usage:
//
//	//go:embed all:public/dist
//	var publicDist embed.FS
//	[...]
//	router.Mount("/static", chix.UseStatic("/static", publicDist))
func UseStatic(path string, embed fs.FS) http.Handler {
	return http.StripPrefix(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "Accept-Encoding")
		w.Header().Set("Cache-Control", "public, max-age=7776000")
		http.FileServer(http.FS(embed)).ServeHTTP(w, r)
	}))
}

// UseStaticDir is similar to UseStatic, but allows providing a subdirectory inside
// of the embedded filesystem, instead of the entire filesystem. go:embed
// will include the target that gets embedded, as a prefix to the path.
//
// For example, given "go:embed all:public/dist", mounted at "/static", you
// would normally have to access using "/static/public/dist/<etc>". Using
// UseStaticDir, where subdir is "public/dist", you can access the same files
// via "/static/<etc>".
func UseStaticDir(path string, embed fs.FS, subdir string) http.Handler {
	sub, err := fs.Sub(embed, strings.Trim(subdir, "/"))
	if err != nil {
		panic(err)
	}

	return http.StripPrefix(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "Accept-Encoding")
		w.Header().Set("Cache-Control", "public, max-age=7776000")
		http.FileServer(http.FS(sub)).ServeHTTP(w, r)
	}))
}
