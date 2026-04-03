// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package vcslocator

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
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
			"simple", Locator("https://github.com/example/test"),
			&Components{Transport: "https", Hostname: "github.com", RepoPath: "/example/test"}, nil, false,
		},
		{
			"commit", Locator("https://github.com/example/test@25c779ba165d1f4fac6fc2ce938bf40c1f8ab1a6"),
			&Components{
				Transport: "https", Hostname: "github.com", RepoPath: "/example/test",
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
		{
			// This test ensures it is all a big file path (not host)
			"file-no-host", Locator("file:///github.com/example/test"),
			&Components{Transport: "file", Hostname: "", RepoPath: "/github.com/example/test", Tool: "git"}, nil, false,
		},
		{
			// This test ensures it is all a big file path (not host)
			"file-host-to-relative", Locator("file://github.com/example/test"),
			&Components{Transport: "file", Hostname: "", RepoPath: "github.com/example/test", Tool: "git"}, nil, false,
		},
		{
			// This test ensures it is all a big file path (not host)
			"file-relative", Locator("file://."),
			&Components{Transport: "file", Hostname: "", RepoPath: ".", Tool: "git"}, nil, false,
		},
		{
			// This test ensures it is all a big file path (not host)
			"file-relative-rev", Locator("file://.@ca3dc240593e102219b70cd0c590b1dfce5e3006"),
			&Components{
				Transport: "file", Hostname: "", RepoPath: ".", Tool: "git",
				Commit:    "ca3dc240593e102219b70cd0c590b1dfce5e3006",
				RefString: "ca3dc240593e102219b70cd0c590b1dfce5e3006",
			}, nil, false,
		},
		{
			"file-revision", Locator("file://test@ca3dc240593e102219b70cd0c590b1dfce5e3006"),
			&Components{
				Transport: "file", Hostname: "", RepoPath: "test", Tool: "git",
				Commit:    "ca3dc240593e102219b70cd0c590b1dfce5e3006",
				RefString: "ca3dc240593e102219b70cd0c590b1dfce5e3006",
			}, nil, false,
		},
		{
			"file-ref", Locator("file:///home/user/repo@refs/notes/commits#28/a0276dde459992f3d8bbb4cb41cd34313a99ff"),
			&Components{
				Transport: "file", Hostname: "", RepoPath: "/home/user/repo", Tool: "git",
				RefString: "refs/notes/commits", SubPath: "28/a0276dde459992f3d8bbb4cb41cd34313a99ff",
			}, nil, false,
		},
		{
			"slug-normal", Locator("kubernetes/release-sdk"),
			&Components{
				Transport: "https", Hostname: "github.com", RepoPath: "kubernetes/release-sdk", Tool: "git",
				RefString: "", SubPath: "",
			}, nil, false,
		},
		{
			"slug-ref", Locator("kubernetes/release-sdk@main"),
			&Components{
				Transport: "https", Hostname: "github.com", RepoPath: "kubernetes/release-sdk", Tool: "git",
				RefString: "main", Tag: "main", SubPath: "",
			}, nil, false,
		},
		{
			"slug-refAsBranch", Locator("kubernetes/release-sdk@main"),
			&Components{
				Transport: "https", Hostname: "github.com", RepoPath: "kubernetes/release-sdk", Tool: "git",
				RefString: "main", Tag: "", Branch: "main", SubPath: "",
			},
			[]fnOpt{WithRefAsBranch(true)},
			false,
		},
		{
			"slug-fragment", Locator("kubernetes/release-sdk#home/"),
			&Components{
				Transport: "https", Hostname: "github.com", RepoPath: "kubernetes/release-sdk", Tool: "git",
				RefString: "", SubPath: "home/",
			}, nil, false,
		},
		{
			"slug-fragment-ref", Locator("kubernetes/release-sdk@chido/one#home/"),
			&Components{
				Transport: "https", Hostname: "github.com", RepoPath: "kubernetes/release-sdk", Tool: "git",
				RefString: "chido/one", Tag: "chido/one", SubPath: "home/",
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
				"20e3b6fc9aa329d3860391b5addb836902d55599fd2f97a7a49fe6a9325f18c1",
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
				"20e3b6fc9aa329d3860391b5addb836902d55599fd2f97a7a49fe6a9325f18c1",
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
				"20e3b6fc9aa329d3860391b5addb836902d55599fd2f97a7a49fe6a9325f18c1",
				"abf988eca60b353c1a1a030219466acc8d355c35a1e40c508e392dd3496be525",
				"e8d84a48c5240adeb41ba5d66fce91cc6df2ef87031debacdd8ab56f40c2227b",
				"17635be05f865e1efeeaba6c83db9c80bfdd09be56c4fe8504eacc55cfd3fd88",
				"7ee3bf580d7f9d45767502618f3c91e88626311f05c9f807208d6bef8ca4b0df",
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

// initTestRepo creates a git repo in dir with an "origin" remote and one commit,
// returning the repo. The caller owns the temp directory cleanup.
func initTestRepo(t *testing.T, dir, remoteURL string) *git.Repository {
	t.Helper()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteURL},
	})
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	// Create a file and commit so HEAD exists.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0o600))
	_, err = wt.Add("README.md")
	require.NoError(t, err)
	_, err = wt.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Now()},
	})
	require.NoError(t, err)

	return repo
}

func TestReadFromRepo(t *testing.T) {
	t.Parallel()

	t.Run("finds repo in start directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		initTestRepo(t, dir, "https://github.com/example/repo.git")

		loc, err := ReadFromRepo(dir)
		require.NoError(t, err)
		require.Contains(t, string(loc), "example/repo")
		require.Contains(t, string(loc), "git+https://")
	})

	t.Run("finds repo by walking up", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		initTestRepo(t, dir, "https://github.com/example/repo.git")

		// Create nested subdirectories inside the repo.
		nested := filepath.Join(dir, "a", "b", "c")
		require.NoError(t, os.MkdirAll(nested, 0o750))

		loc, err := ReadFromRepo(nested)
		require.NoError(t, err)
		require.Contains(t, string(loc), "example/repo")
	})

	t.Run("respects WithTopLevelPath", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		initTestRepo(t, dir, "https://github.com/example/repo.git")

		nested := filepath.Join(dir, "a", "b")
		require.NoError(t, os.MkdirAll(nested, 0o750))

		loc, err := ReadFromRepo(nested, WithTopLevelPath(dir))
		require.NoError(t, err)
		require.Contains(t, string(loc), "example/repo")
	})

	t.Run("stops at top level path before finding repo", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		initTestRepo(t, dir, "https://github.com/example/repo.git")

		// Set the top-level to a child dir so the walk never reaches the repo root.
		child := filepath.Join(dir, "sub")
		require.NoError(t, os.MkdirAll(child, 0o750))
		nested := filepath.Join(child, "deep")
		require.NoError(t, os.MkdirAll(nested, 0o750))

		_, err := ReadFromRepo(nested, WithTopLevelPath(child))
		require.Error(t, err)
		require.Contains(t, err.Error(), "no git repository found")
	})

	t.Run("errors when top level path is not parent of start", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		other := t.TempDir()

		_, err := ReadFromRepo(dir, WithTopLevelPath(other))
		require.Error(t, err)
		require.Contains(t, err.Error(), "is not a parent of")
	})

	t.Run("errors when start directory does not exist", func(t *testing.T) {
		t.Parallel()
		_, err := ReadFromRepo("/nonexistent/path/that/does/not/exist")
		require.Error(t, err)
	})

	t.Run("no repo found returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		_, err := ReadFromRepo(dir, WithTopLevelPath(dir))
		require.Error(t, err)
		require.Contains(t, err.Error(), "no git repository found")
	})

	t.Run("ssh remote URL", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		initTestRepo(t, dir, "git@github.com:example/repo.git")

		loc, err := ReadFromRepo(dir)
		require.NoError(t, err)
		require.Contains(t, string(loc), "git+ssh://github.com/example/repo")
	})

	t.Run("branch ref in locator", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		initTestRepo(t, dir, "https://github.com/example/repo.git")

		loc, err := ReadFromRepo(dir)
		require.NoError(t, err)
		// HEAD should be on a branch (master or main), so the locator
		// should contain refs/heads/ as the ref part.
		require.Contains(t, string(loc), "refs/heads/")
	})

	t.Run("detects symlink divergence", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		initTestRepo(t, dir, "https://github.com/example/repo.git")

		// Create a directory outside the repo and a symlink inside the repo
		// that points to it.
		outside := t.TempDir()
		outsideChild := filepath.Join(outside, "child")
		require.NoError(t, os.MkdirAll(outsideChild, 0o750))

		linkDir := filepath.Join(dir, "link")
		require.NoError(t, os.Symlink(outsideChild, linkDir))

		// Starting from the symlinked path — after EvalSymlinks it will
		// resolve to the outside directory and the walk should not find
		// the repo (because the resolved path is outside the repo tree).
		_, err := ReadFromRepo(linkDir, WithTopLevelPath(outside))
		require.Error(t, err)
	})
}

func TestRemoteURLToLocator(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		remoteURL string
		ref       string
		expected  string
		mustErr   bool
	}{
		{
			"https",
			"https://github.com/example/repo.git",
			"refs/heads/main",
			"git+https://github.com/example/repo@refs/heads/main",
			false,
		},
		{
			"https-no-dotgit",
			"https://github.com/example/repo",
			"abc1234",
			"git+https://github.com/example/repo@abc1234",
			false,
		},
		{
			"ssh-scp",
			"git@github.com:org/project.git",
			"refs/heads/main",
			"git+ssh://github.com/org/project@refs/heads/main",
			false,
		},
		{
			"ssh-scheme",
			"ssh://git@github.com/org/project.git",
			"refs/heads/main",
			"git+ssh://github.com/org/project@refs/heads/main",
			false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := remoteURLToLocator(tc.remoteURL, tc.ref)
			if tc.mustErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}
