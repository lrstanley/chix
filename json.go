// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"net/http"
	"strconv"
)

// JSONDecoder is a function that is used to unmarshal JSON data (e.g. request body),
// to a value.
type JSONDecoder func(r *http.Request, v any) error

// JSONEncoder is a function that is used to marshal JSON data (e.g. response body),
// from a value.
type JSONEncoder func(w http.ResponseWriter, r *http.Request, v any) error

// DefaultJSONDecoder returns a [JSONDecoder] that unmarshals the request body with
// [encoding/json/v2.UnmarshalRead]. opts are applied to every decode. Request body
// size is limited by [Config.GetMaxRequestBodyBytes].
func DefaultJSONDecoder(opts ...json.Options) JSONDecoder {
	joined := json.JoinOptions(opts...)
	return func(r *http.Request, v any) error {
		if err := limitRequestBody(r, GetConfig(r.Context()).GetMaxRequestBodyBytes(), nil); err != nil {
			return err
		}
		return json.UnmarshalRead(r.Body, v, joined)
	}
}

// DefaultJSONEncoder returns a [JSONEncoder] that marshals values with
// [encoding/json/v2.MarshalWrite]. opts are applied to every encode. When the
// origin request has "?pretty=true" (or equivalent), output is indented. HTML is
// not escaped.
func DefaultJSONEncoder(opts ...json.Options) JSONEncoder {
	joined := json.JoinOptions(opts...)
	prettyOpts := json.JoinOptions(joined, jsontext.Multiline(true))
	return func(w http.ResponseWriter, r *http.Request, v any) error {
		if pretty, _ := strconv.ParseBool(r.FormValue("pretty")); pretty {
			return json.MarshalWrite(w, v, prettyOpts)
		}
		return json.MarshalWrite(w, v, joined)
	}
}
