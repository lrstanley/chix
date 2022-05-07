// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/apex/log"
)

// M is a convenience alias for quickly building a map structure that is going
// out to a responder. Just a short-hand.
type M map[string]any

// Fields satisfies the log.Fielder interface.
func (m M) Fields() (f log.Fields) {
	if m == nil {
		return nil
	}

	f = make(log.Fields)
	for k, v := range m {
		f[k] = v
	}

	return f
}

// JSON marshals 'v' to JSON, and setting the Content-Type as application/json.
// Note that this does NOT auto-escape HTML.
//
// JSON also supports prettification when the origin request has "?pretty=true"
// or similar.
func JSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)

	if pretty, _ := strconv.ParseBool(r.FormValue("pretty")); pretty {
		enc.SetIndent("", "    ")
	}

	if err := enc.Encode(v); err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
