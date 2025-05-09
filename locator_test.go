// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package vcslocator

import (
	"crypto/sha256"
	"fmt"
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
			&Components{
				Transport: "http", Hostname: "github.com", RepoPath: "/example/test",
				Commit: "25c779ba165d1f4fac6fc2ce938bf40c1f8ab1a6", RefString: "25c779ba165d1f4fac6fc2ce938bf40c1f8ab1a6",
			}, nil, false,
		},
		{
			"full-branch", Locator("git+http://github.com/example/test@abcd#%2egithub/dependabot.yaml"),
			&Components{
				Tool: "git", Transport: "http", Hostname: "github.com",
				RepoPath: "/example/test", RefString: "abcd", SubPath: ".github/dependabot.yaml",
				Tag: "", Branch: "abcd", Commit: "",
			},
			[]fnOpt{WithRefAsBranch(true)},
			false,
		},
		{
			"full-tag", Locator("git+http://github.com/example/test@abcd#%2egithub/dependabot.yaml"),
			&Components{
				Tool: "git", Transport: "http", Hostname: "github.com",
				RepoPath: "/example/test", RefString: "abcd", SubPath: ".github/dependabot.yaml",
				Tag: "abcd", Branch: "", Commit: "",
			},
			[]fnOpt{WithRefAsBranch(false)},
			false,
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

func TestGetGroup(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		locators []string
		expect   []string
		mustErr  bool
	}{
		{
			"single",
			[]string{"git+https://github.com/carabiner-dev/vcslocator@76241a877eb3374f6017224c61d6a167c337de4d#.gitignore"},
			[]string{"b319f85e4a246c38474a242ecaee46ca514c4abcfae781f0f7e2a7a58b3e5a4f"},
			false,
		},
		{
			"two",
			[]string{
				"git+https://github.com/carabiner-dev/vcslocator@b145fcf66fe321522ca093de00646f8c1e482e8d#components.go",
				"git+https://github.com/carabiner-dev/vcslocator@cb1adf0eb1179e26228091c3a347d037ae7b4460#components.go",
			},
			[]string{
				"58c76f62c2d403aa2d946f53b381f4948f12a6814482d50fb4fd3d87f45e38d3",
				"58c76f62c2d403aa2d946f53b381f4948f12a6814482d50fb4fd3d87f45e38d3",
			},
			false,
		},
		{
			"two-and-two-repos",
			[]string{
				"git+https://github.com/carabiner-dev/vcslocator@b145fcf66fe321522ca093de00646f8c1e482e8d#components.go",
				"git+https://github.com/carabiner-dev/vcslocator@cb1adf0eb1179e26228091c3a347d037ae7b4460#components.go",
				"git+https://github.com/carabiner-dev/actions@ecdd8b03b5c1bad78d5d89ab71e1ca9bb5ad31c9#drop/action.yml",
				"git+https://github.com/carabiner-dev/actions@ecdd8b03b5c1bad78d5d89ab71e1ca9bb5ad31c9#install/ampel/action.yml",
			},
			[]string{
				"58c76f62c2d403aa2d946f53b381f4948f12a6814482d50fb4fd3d87f45e38d3",
				"58c76f62c2d403aa2d946f53b381f4948f12a6814482d50fb4fd3d87f45e38d3",
				"abf988eca60b353c1a1a030219466acc8d355c35a1e40c508e392dd3496be525",
				"e8d84a48c5240adeb41ba5d66fce91cc6df2ef87031debacdd8ab56f40c2227b",
			},
			false,
		},
		{
			"takes-six-to-tango-the-throttler",
			[]string{
				"git+https://github.com/carabiner-dev/vcslocator@b145fcf66fe321522ca093de00646f8c1e482e8d#components.go",
				"git+https://github.com/carabiner-dev/vcslocator@cb1adf0eb1179e26228091c3a347d037ae7b4460#components.go",
				"git+https://github.com/carabiner-dev/actions@ecdd8b03b5c1bad78d5d89ab71e1ca9bb5ad31c9#drop/action.yml",
				"git+https://github.com/carabiner-dev/actions@ecdd8b03b5c1bad78d5d89ab71e1ca9bb5ad31c9#install/ampel/action.yml",
				"git+https://github.com/carabiner-dev/actions@ecdd8b03b5c1bad78d5d89ab71e1ca9bb5ad31c9#install/bnd/action.yml",
				"git+https://github.com/carabiner-dev/actions@3a2820538c0bfe5be1ad7589a68d03823f403c5c#install/ampel/action.yml",
			},
			[]string{
				"58c76f62c2d403aa2d946f53b381f4948f12a6814482d50fb4fd3d87f45e38d3",
				"58c76f62c2d403aa2d946f53b381f4948f12a6814482d50fb4fd3d87f45e38d3",
				"abf988eca60b353c1a1a030219466acc8d355c35a1e40c508e392dd3496be525",
				"e8d84a48c5240adeb41ba5d66fce91cc6df2ef87031debacdd8ab56f40c2227b",
				"17635be05f865e1efeeaba6c83db9c80bfdd09be56c4fe8504eacc55cfd3fd88",
				"e8d84a48c5240adeb41ba5d66fce91cc6df2ef87031debacdd8ab56f40c2227b",
			},
			false,
		},
		{
			"one-errs",
			[]string{"git+https://github.com/carabiner-dev/vcslocator@76241a877eb3374f6017224c61d6a167c337de4d#.gitignore2"},
			[]string{},
			true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dataGroup, err := GetGroup(tc.locators)
			if tc.mustErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, dataGroup, len(tc.locators))

			for i, data := range dataGroup {
				h := sha256.New()
				h.Write(data)
				require.Equal(t, tc.expect[i], fmt.Sprintf("%x", h.Sum(nil)))
			}
		})
	}
}
