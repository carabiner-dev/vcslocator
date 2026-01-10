// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package vcslocator

import (
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
