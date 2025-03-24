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
		opts    []fnOpt
		mustErr bool
	}{
		{
			"simple", Locator("http://github.com/example/test"),
			&Components{Transport: "http", Hostname: "github.com", RepoPath: "/example/test"}, nil, false,
		},
		{
			"commit", Locator("http://github.com/example/test@25c779ba165d1f4fac6fc2ce938bf40c1f8ab1a6"),
			&Components{Transport: "http", Hostname: "github.com", RepoPath: "/example/test",
				Commit: "25c779ba165d1f4fac6fc2ce938bf40c1f8ab1a6", RefString: "25c779ba165d1f4fac6fc2ce938bf40c1f8ab1a6",
			}, nil, false,
		},
		{
			"full-branch", Locator("git+http://github.com/example/test@abcd#%2egithub/dependabot.yaml"),
			&Components{
				Tool: "git", Transport: "http", Hostname: "github.com",
				RepoPath: "/example/test", RefString: "abcd", SubPath: ".github/dependabot.yaml",
				Tag: "", Branch: "abcd", Commit: "",
			}, []fnOpt{WithRefAsBranch(true)}, false,
		},
		{
			"full-tag", Locator("git+http://github.com/example/test@abcd#%2egithub/dependabot.yaml"),
			&Components{
				Tool: "git", Transport: "http", Hostname: "github.com",
				RepoPath: "/example/test", RefString: "abcd", SubPath: ".github/dependabot.yaml",
				Tag: "abcd", Branch: "", Commit: "",
			}, []fnOpt{WithRefAsBranch(false)}, false,
		},
		{
			"unescaped-fragment", Locator("git+http://github.com/example/test@abcd#.github/dependabot.yaml"),
			&Components{
				Tool: "git", Transport: "http", Hostname: "github.com",
				RepoPath: "/example/test", RefString: "abcd", SubPath: ".github/dependabot.yaml",
				Branch: "", Tag: "abcd", Commit: "",
			}, nil, false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res, err := tc.locator.Parse(tc.opts...)
			if tc.mustErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expect.Transport, res.Transport, "tool mismatch")
			require.Equal(t, tc.expect.Tool, res.Tool, "tool mismatch")
			require.Equal(t, tc.expect.Hostname, res.Hostname, "hostname mismatch")
			require.Equal(t, tc.expect.RepoPath, res.RepoPath, "Repo path mismatch")
			require.Equal(t, tc.expect.RefString, res.RefString, "Refstring mismatch")
			require.Equal(t, tc.expect.SubPath, res.SubPath, "subpath mismatch")
			require.Equal(t, tc.expect.Commit, res.Commit, "Commit mismatch")
			require.Equal(t, tc.expect.Branch, res.Branch, "Branch mismatch")
			require.Equal(t, tc.expect.Tag, res.Tag, "Tag mismatch")
		})
	}
}
