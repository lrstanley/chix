// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

// Key to use when setting the request ID.
type contextKeyRequestID struct{}

var (
	requestIDTracker atomic.Uint64
	requestIDPrefix  = sync.OnceValue(func() string {
		hostname, err := os.Hostname()
		if hostname == "" || err != nil {
			hostname = "unknown"
		}
		var buf [12]byte
		var b64 string
		repl := strings.NewReplacer("+", "", "/", "")
		for len(b64) < 10 {
			_, _ = rand.Read(buf[:])
			b64 = base64.StdEncoding.EncodeToString(buf[:])
			b64 = repl.Replace(b64)
		}
		return fmt.Sprintf("%s/%s", hostname, b64[0:10])
	})
)

// UseRequestID is a middleware that injects a request ID into the context of each
// request. If the connecting client provides a request ID in the request header,
// it will be used instead. If no request ID is provided, a new request ID will be
// generated. To change which header is referenced, use [Config.SetRequestIDHeader].
func UseRequestID() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			requestID := r.Header.Get(GetConfig(ctx).GetRequestIDHeader())
			if requestID == "" {
				requestID = fmt.Sprintf("%s-%06d", requestIDPrefix(), requestIDTracker.Add(1))
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, contextKeyRequestID{}, requestID)))
		})
	}
}

// GetRequestID returns a request ID from the context if present, otherwise "".
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(contextKeyRequestID{}).(string); ok {
		return id
	}
	return ""
}

// GetRequestIDOrHeader returns a request ID from the context if present, otherwise
// the request header, if present. The header can be changed using [Config.SetRequestIDHeader].
func GetRequestIDOrHeader(ctx context.Context, r *http.Request) string {
	id := GetRequestID(ctx)
	if id == "" {
		id = r.Header.Get(GetConfig(ctx).GetRequestIDHeader())
	}
	return id
}
