// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

//go:build !goexperiment.jsonv2

package chix

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func DefaultJSONDecoder() JSONDecoder {
	return func(r *http.Request, v any) error {
		jdec := json.NewDecoder(r.Body)
		return jdec.Decode(v)
	}
}

func DefaultJSONEncoder() JSONEncoder {
	return func(w http.ResponseWriter, r *http.Request, v any) error {
		buf := renderBufferPool.Get()
		defer renderBufferPool.Put(buf)

		enc := json.NewEncoder(buf)
		if pretty, _ := strconv.ParseBool(r.FormValue("pretty")); pretty {
			enc.SetIndent("", "    ")
		}
		err := enc.Encode(v)
		if err == nil {
			_, _ = w.Write(buf.Bytes())
		}
		return err
	}
}
