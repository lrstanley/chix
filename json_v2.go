// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

//go:build goexperiment.jsonv2

package chix

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"net/http"
	"strconv"
)

func DefaultJSONDecoder(opts ...json.Options) JSONDecoder {
	return func(r *http.Request, v any) error {
		return json.UnmarshalRead(r.Body, v, opts...)
	}
}

func DefaultJSONEncoder(opts ...json.Options) JSONEncoder {
	return func(w http.ResponseWriter, r *http.Request, v any) error {
		var err error
		if pretty, _ := strconv.ParseBool(r.FormValue("pretty")); pretty {
			err = json.MarshalWrite(w, v, append(opts, jsontext.WithIndent("    "))...)
		} else {
			err = json.MarshalWrite(w, v)
		}
		return err
	}
}
