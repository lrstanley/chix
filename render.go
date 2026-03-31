// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"iter"
	"net/http"
	"strconv"

	"github.com/lrstanley/x/sync/pool"
)

// M is a convenience alias for quickly building a map structure that is going
// out to a responder. Just a short-hand.
type M map[string]any

var renderBufferPool = pool.Pool[*bytes.Buffer]{
	New: func() *bytes.Buffer { return &bytes.Buffer{} },
	Prepare: func(v *bytes.Buffer) *bytes.Buffer {
		v.Reset()
		return v
	},
}

// JSON marshals 'v' to JSON, and setting the Content-Type as application/json.
// Note that this does NOT auto-escape HTML.
//
// JSON also supports indented output when the origin request has "?pretty=true"
// or similar.
func JSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := GetConfig(r.Context()).GetJSONEncoder()(w, r, v); err != nil {
		ErrorWithCode(w, r, http.StatusInternalServerError, err)
	}
}

// XML marshals 'v' to XML, and setting the Content-Type as application/xml.
//
// XML also supports indented output when the origin request has "?pretty=true"
// or similar.
func XML(w http.ResponseWriter, r *http.Request, status int, v any) {
	buf := renderBufferPool.Get()
	defer renderBufferPool.Put(buf)

	enc := xml.NewEncoder(buf)
	if pretty, _ := strconv.ParseBool(r.FormValue("pretty")); pretty {
		enc.Indent("", "    ")
	}
	if err := enc.Encode(v); err != nil {
		ErrorWithCode(w, r, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// CSV marshals 'rows' to CSV, and setting the Content-Type as text/csv.
func CSV(w http.ResponseWriter, r *http.Request, status int, rows [][]string) {
	buf := renderBufferPool.Get()
	defer renderBufferPool.Put(buf)

	enc := csv.NewWriter(buf)
	if err := enc.WriteAll(rows); err != nil {
		ErrorWithCode(w, r, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// CSVIter marshals rows from the associated iterator to CSV, and setting the
// Content-Type as text/csv. If the iterator returns an error, the error will be
// returned and the response will be set to a 500 internal server error, otherwise
// the rows are written to a buffer, which will be written once the iterator is
// exhausted.
func CSVIter(w http.ResponseWriter, r *http.Request, status int, it iter.Seq2[[]string, error]) {
	buf := renderBufferPool.Get()
	defer renderBufferPool.Put(buf)
	enc := csv.NewWriter(buf)

	for row, err := range it {
		if err == nil {
			err = enc.Write(row)
		}
		if err != nil {
			ErrorWithCode(w, r, http.StatusInternalServerError, err)
			return
		}
	}
	enc.Flush()

	w.Header().Set("Content-Type", "text/csv")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
