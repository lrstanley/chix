// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package chix

// contextKey is a type that prevent key collisions in contexts. When the type
// is different, even if the key name is the same, it will never overlap with
// another package.
type contextKey string

const (
	contextDebug       contextKey = "debug"
	contextAuth        contextKey = "auth"
	contextAuthID      contextKey = "auth_id"
	contextAuthRoles   contextKey = "auth_roles"
	contextNextURL     contextKey = "next_url"
	contextSkipNextURL contextKey = "skip_next_url"
	contextIP          contextKey = "ip"

	authSessionKey = "_auth"
	nextSessionKey = "_next"
)
