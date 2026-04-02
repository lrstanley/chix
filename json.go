// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import "net/http"

// JSONDecoder is a function that is used to unmarshal JSON data (e.g. request body),
// to a value.
type JSONDecoder func(r *http.Request, v any) error

// JSONEncoder is a function that is used to marshal JSON data (e.g. response body),
// from a value.
type JSONEncoder func(w http.ResponseWriter, r *http.Request, v any) error
