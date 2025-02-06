// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package vcslocator

import (
	"net/url"
	"strings"
)

type Locator string

type Components struct {
	Tool      string
	Transport string
	Hostname  string
	RepoPath  string
	Ref       string
	SubPath   string
}

// Parses a VCS locator and returns its components
func (l Locator) Parse() (*Components, error) {
	// <vcs_tool>+<transport>://<host_name>[/<path_to_repository>][@<revision_tag_or_branch>][#<sub_path>]
	u, err := url.Parse(string(l))
	if err != nil {
		return nil, err
	}

	path, ref, _ := strings.Cut(u.Path, "@")
	tool, transport, si := strings.Cut(u.Scheme, "+")
	if !si {
		tool = transport
		transport = ""
	}

	return &Components{
		Tool:      tool,
		Transport: transport,
		Hostname:  u.Hostname(),
		RepoPath:  path,
		Ref:       ref,
		SubPath:   u.Fragment,
	}, nil
}
