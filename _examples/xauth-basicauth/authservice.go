// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package main

import (
	"context"
	"errors"
	"sync"

	"github.com/lrstanley/chix/xauth/v2"
)

type User struct {
	Username string `json:"username"`
	Display  string `json:"display"`
}

var _ xauth.BasicAuthService[User] = (*AuthService)(nil)

// AuthService is a crude in-memory example service. The storage backend should be
// persisted somewhere else, such as a database, file, etc.
type AuthService struct {
	mu    sync.RWMutex
	users map[string]User
}

func newAuthService() *AuthService {
	return &AuthService{
		users: map[string]User{
			"alice": {Username: "alice", Display: "Alice Example"},
			"bob":   {Username: "bob", Display: "Bob Example"},
		},
	}
}

func (s *AuthService) BasicAuth(_ context.Context, username, password string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[username]
	if !ok || password != demoPassword(username) {
		return nil, errors.New("invalid credentials")
	}
	return &user, nil
}

func (s *AuthService) Get(_ context.Context, username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[username]
	if !ok {
		return nil, errors.New("user not found")
	}
	return &user, nil
}

// demoPassword returns the demo password for example users only.
func demoPassword(username string) string {
	switch username {
	case "alice":
		return "secret"
	case "bob":
		return "hunter2"
	default:
		return ""
	}
}
