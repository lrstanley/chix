// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// testJSONMarshalEqual is a helper function to test if the resulting http body equals
// the input data when marshalled and unmarshalled.
func testJSONMarshalEqual[T any](t *testing.T, input T, body io.ReadCloser, shouldEqual bool) (ok bool) {
	var in, out any

	inBytes, err := json.Marshal(input)
	if err != nil {
		t.Errorf("error marshaling input data: %v", err)
		return false
	}

	if err = json.Unmarshal(inBytes, &in); err != nil {
		t.Errorf("error unmarshaling input data: %v", err)
		return false
	}

	dec := json.NewDecoder(body)
	if err = dec.Decode(&out); err != nil {
		t.Errorf("error decoding response body: %v", err)
		return false
	}

	if reflect.DeepEqual(in, out) && !shouldEqual {
		t.Errorf("expected %#v to not equal %#v", in, out)
		return false
	} else if !reflect.DeepEqual(in, out) && shouldEqual {
		t.Errorf("expected %#v to equal %#v", in, out)
		return false
	}

	return true
}

func TestJSON(t *testing.T) {
	tests := []struct {
		name       string
		data       any
		headers    map[string]string
		statusCode int
	}{
		{
			name:       "empty",
			data:       M{},
			headers:    map[string]string{"Content-Type": "application/json"},
			statusCode: http.StatusOK,
		},
		{
			name:       "base-object",
			data:       M{"foo": "bar"},
			headers:    map[string]string{"Content-Type": "application/json"},
			statusCode: http.StatusOK,
		},
		{
			name:       "base-array",
			data:       []string{"foo", "bar"},
			headers:    map[string]string{"Content-Type": "application/json"},
			statusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com/?pretty=true", http.NoBody)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				JSON(w, r, tt.statusCode, tt.data)
			})

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			resp := rec.Result()

			if resp.StatusCode != tt.statusCode {
				t.Errorf("expected status code %d, got %d", tt.statusCode, resp.StatusCode)
			}

			for k, v := range tt.headers {
				if resp.Header.Get(k) != v {
					t.Errorf("expected header %s to be %s, got %s", k, v, resp.Header.Get(k))
				}
			}

			_ = testJSONMarshalEqual(t, tt.data, resp.Body, true)
		})
	}
}
