// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package vcslocator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLocator(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		locator Locator
		expect  *Components
		mustErr bool
	}{
		{
			"simple", Locator("http://github.com/example/test"),
			&Components{Transport: "http", Hostname: "github.com", RepoPath: "/example/test"}, false,
		},
		{
			"full", Locator("git+http://github.com/example/test@abcd#%2egithub/dependabot.yaml"),
			&Components{
				Tool: "git", Transport: "http", Hostname: "github.com",
				RepoPath: "/example/test", Ref: "abcd", SubPath: ".github/dependabot.yaml",
			}, false,
		},
		{
			"unescaped-fragment", Locator("git+http://github.com/example/test@abcd#.github/dependabot.yaml"),
			&Components{
				Tool: "git", Transport: "http", Hostname: "github.com",
				RepoPath: "/example/test", Ref: "abcd", SubPath: ".github/dependabot.yaml",
			}, false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res, err := tc.locator.Parse()
			if tc.mustErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expect.Tool, res.Tool)
			require.Equal(t, tc.expect.Hostname, res.Hostname)
			require.Equal(t, tc.expect.RepoPath, res.RepoPath)
			require.Equal(t, tc.expect.Ref, res.Ref)
			require.Equal(t, tc.expect.SubPath, res.SubPath)
		})
	}
}
