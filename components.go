// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package vcslocator

import (
	"fmt"
	"strings"
)

// Components captures the parsed pieces of a VCS locator.
type Components struct {
	Tool      string
	Transport string
	Hostname  string
	RepoPath  string
	RefString string
	Commit    string
	Tag       string
	Branch    string
	SubPath   string
}

// RepoURL forms the repository URL to clone based on the defined components
func (c *Components) RepoURL() string {
	switch c.Transport {
	case TransportHTTPS, "":
		return fmt.Sprintf("https://%s/%s", c.Hostname, strings.TrimPrefix(c.RepoPath, "/"))
	case TransportSSH:
		return fmt.Sprintf("git@%s:%s", c.Hostname, strings.TrimPrefix(c.RepoPath, "/"))
	case TransportFile:
		// The path keeps its leading slash (or slash + drive letter on
		// Windows) so the URL is always file:///path.
		return TransportFile + "://" + c.RepoPath
	default:
		return ""
	}
}
