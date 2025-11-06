// Copyright (c) Liam Stanley <liam@liam.sh>. All rights reserved. Use of
// this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pool

import "sync"

type Resetable interface {
	Reset()
}

// Pool is a generic wrapper around [sync.Pool]. If your T type implements [Resetable],
// the resetter function will be called after the entry is returned to the pool.
type Pool[T any] struct {
	entries sync.Pool
}

// New creates a new [Pool] with the given function to create new entries.
func New[T any](item func() T) Pool[T] {
	return Pool[T]{
		entries: sync.Pool{
			New: func() any {
				return item()
			},
		},
	}
}

// Get retrieves an entry from the pool.
func (p *Pool[T]) Get() T {
	return p.entries.Get().(T) //nolint:errcheck
}

// Put returns an entry to the pool, optionally resetting the entry if a resetter
// function is provided.
func (p *Pool[T]) Put(item T) {
	if v, ok := any(item).(Resetable); ok {
		v.Reset()
	}
	p.entries.Put(item)
}
