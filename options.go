// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package vcslocator

import (
	"context"
	"errors"
)

// options is the internal options struct used by the locator functions.
// The type is private as it is exposed and defined only with functional
// options.
type options struct {
	RefIsBranch bool
	ClonePath   string

	// ReadCredentials controls if the library loads the system git credentials
	ReadCredentials bool

	// Username and password for HTTP basic config
	HttpUsername, HttpPassword string

	// TopLevelPath sets the uppermost directory to search when walking up the
	// filesystem looking for a git repository. Defaults to the filesystem root.
	TopLevelPath string

	// Context bounds the network operations (clones and fetches) of a single
	// call. The options struct only lives for that call, so the context is
	// never retained beyond it.
	Context context.Context //nolint:containedctx // per-call functional option
}

// context returns the configured context or a background one when none is set.
func (o *options) context() context.Context {
	if o.Context == nil {
		return context.Background()
	}

	return o.Context
}

var defaultOptions = options{
	ReadCredentials: true,
	RefIsBranch:     false,
}

type fnOpt func(*options) error

// WithRefAsBranch instructs the parser to treat the ref as branch name instead
// of a tag name.
func WithRefAsBranch(sino bool) fnOpt { //nolint:revive
	return func(o *options) error {
		if o == nil {
			return errors.New("options are nil")
		}

		o.RefIsBranch = sino

		return nil
	}
}

// WithClonePath specifies the directory to clone the repository. When
func WithClonePath(path string) fnOpt {
	return func(o *options) error {
		if o == nil {
			return errors.New("options are nil")
		}

		o.ClonePath = path

		return nil
	}
}

// WithSystemCredentials controls if cloning uses the system credentials
func WithSystemCredentials(yesno bool) fnOpt {
	return func(o *options) error {
		if o == nil {
			return errors.New("options are nil")
		}
		o.ReadCredentials = yesno
		return nil
	}
}

// WithTopLevelPath sets the uppermost directory the repository search will
// walk up to. The path must be a parent of the starting directory.
func WithTopLevelPath(path string) fnOpt {
	return func(o *options) error {
		if o == nil {
			return errors.New("options are nil")
		}
		o.TopLevelPath = path
		return nil
	}
}

// WithContext bounds the network operations of the call with a context.
// Cancelling the context or reaching its deadline aborts the clone or fetch
// in flight and the call returns the context error.
func WithContext(ctx context.Context) fnOpt {
	return func(o *options) error {
		if o == nil {
			return errors.New("options are nil")
		}

		if ctx == nil {
			return errors.New("context is nil")
		}

		o.Context = ctx

		return nil
	}
}

// WithHttpAuth configures basic authentication for http operations
func WithHttpAuth(user, password string) fnOpt {
	return func(o *options) error {
		if o == nil {
			return errors.New("options are nil")
		}

		o.HttpUsername = user
		o.HttpPassword = password
		return nil
	}
}
