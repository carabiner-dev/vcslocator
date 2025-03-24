// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package vcslocator

import "fmt"

type Options struct {
	RefIsBranch bool
	ClonePath   string
}

var defaultOptions = Options{
	RefIsBranch: false,
}

type fnOpt func(*Options) error

// WithRefAsBranch instructs the parser to treat the ref as branch name instead
// of a tag name.
func WithRefAsBranch(sino bool) fnOpt {
	return func(o *Options) error {
		if o == nil {
			return fmt.Errorf("options are nil")
		}
		o.RefIsBranch = sino
		return nil
	}
}

// WithClonePath specifies the directory to clone the repository. When
func WithClonePath(path string) fnOpt {
	return func(o *Options) error {
		if o == nil {
			return fmt.Errorf("options are nil")
		}
		o.ClonePath = path
		return nil
	}
}
