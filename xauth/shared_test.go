// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in the
// LICENSE file.

package xauth

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
)

// testSessionStore is shared across handler tests so [gothic.Store] (set once via
// [gothSessionStoreOnce]) always matches the store passed to each handler.
var testSessionStore sessions.Store

type testUser struct {
	Name string `json:"name"`
}

type mockBasicAuth struct {
	ident     *testUser
	validUser string
	validPass string
	basicErr  error
	getErr    error
}

func (m *mockBasicAuth) BasicAuth(_ context.Context, username, password string) (*testUser, error) {
	if m.basicErr != nil {
		return nil, m.basicErr
	}
	if username != m.validUser || password != m.validPass {
		return nil, errors.New("unauthorized")
	}
	return m.ident, nil
}

func (m *mockBasicAuth) Get(_ context.Context, username string) (*testUser, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if username != m.validUser {
		return nil, errors.New("not found")
	}
	return m.ident, nil
}

type mockGothService struct {
	ident  *testUser
	id     string
	setErr error
	getErr error
}

func (m *mockGothService) Get(_ context.Context, _ string) (*testUser, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.ident, nil
}

func (m *mockGothService) Set(_ context.Context, _ *goth.User) (string, error) {
	if m.setErr != nil {
		return "", m.setErr
	}
	return m.id, nil
}

func TestMain(m *testing.M) {
	authKey := strings.Repeat("ab", 32)    // 64 hex chars -> 32-byte auth key
	encryptKey := strings.Repeat("cd", 16) // 32 hex chars -> 16-byte encryption key
	testSessionStore = NewCookieStore(authKey, encryptKey)
	os.Exit(m.Run())
}
