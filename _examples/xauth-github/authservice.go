// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package main

import (
	"context"
	"errors"
	"sync"

	"github.com/lrstanley/chix/xauth/v2"
	"github.com/markbates/goth"
)

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Email    string `json:"email"`
}

var _ xauth.Service[User, string] = (*AuthService)(nil)

// AuthService is a crude in-memory example service. The storage backend should be
// persisted somewhere else, such as a database, file, etc.
type AuthService struct {
	mu    sync.RWMutex
	users map[string]*User
}

func (s *AuthService) Get(_ context.Context, id string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (s *AuthService) Set(_ context.Context, user *goth.User) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[user.UserID] = &User{
		ID:       user.UserID,
		Username: user.Name,
		Avatar:   user.AvatarURL,
		Email:    user.Email,
	}
	return user.UserID, nil
}
