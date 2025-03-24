// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package vcslocator

import (
	"fmt"
	"strings"
)

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
	case "https", "":
		return fmt.Sprintf("https://%s/%s", c.Hostname, strings.TrimPrefix(c.RepoPath, "/"))
	case "ssh":
		return fmt.Sprintf("git@%s:%s", c.Hostname, strings.TrimPrefix(c.RepoPath, "/"))
	default:
		// return nil, errors.New("only HTTPS and SSH are supported as transports")
		return ""
	}
}
