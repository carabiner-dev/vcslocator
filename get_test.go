// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package vcslocator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
)

// fileLocator builds a file:// locator string that works on all platforms.
// On Windows, backslashes are converted to forward slashes and the path
// is prefixed with a leading slash (e.g. file:///D:/tmp/repo@commit#sub).
func fileLocator(repoDir, commitHash, fragment string) string {
	p := filepath.ToSlash(repoDir)
	if runtime.GOOS == "windows" && len(p) > 0 && p[0] != '/' {
		p = "/" + p
	}
	loc := fmt.Sprintf("file://%s@%s", p, commitHash)
	if fragment != "" {
		loc += "#" + fragment
	}
	return loc
}

// initTestRepoWithFiles creates a bare-ish local git repo with multiple files
// committed, returning the absolute repo path and the commit hash.
func initTestRepoWithFiles(t *testing.T, files map[string]string) (repoDir, commitHash string) {
	t.Helper()
	repoDir = t.TempDir()

	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	for relPath, content := range files {
		abs := filepath.Join(repoDir, relPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o750))
		require.NoError(t, os.WriteFile(abs, []byte(content), 0o600))
		_, err := wt.Add(relPath)
		require.NoError(t, err)
	}

	hash, err := wt.Commit("test commit", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Now()},
	})
	require.NoError(t, err)

	return repoDir, hash.String()
}

func TestCopyFile(t *testing.T) {
	t.Parallel()

	repoDir, commitHash := initTestRepoWithFiles(t, map[string]string{
		"hello.txt":         "hello world",
		"docs/guide.md":     "# Guide\nSome content.",
		"src/main.go":       "package main\n",
		"src/util/utils.go": "package util\n",
	})

	t.Run("copies a top-level file", func(t *testing.T) {
		t.Parallel()
		locator := fileLocator(repoDir, commitHash, "hello.txt")
		var buf bytes.Buffer
		err := CopyFile(locator, &buf)
		require.NoError(t, err)
		require.Equal(t, "hello world", buf.String())
	})

	t.Run("copies a nested file", func(t *testing.T) {
		t.Parallel()
		locator := fileLocator(repoDir, commitHash, "docs/guide.md")
		var buf bytes.Buffer
		err := CopyFile(locator, &buf)
		require.NoError(t, err)
		require.Equal(t, "# Guide\nSome content.", buf.String())
	})

	t.Run("errors when no subpath", func(t *testing.T) {
		t.Parallel()
		locator := fileLocator(repoDir, commitHash, "")
		var buf bytes.Buffer
		err := CopyFile(locator, &buf)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no subpath defined")
	})

	t.Run("errors when file does not exist", func(t *testing.T) {
		t.Parallel()
		locator := fileLocator(repoDir, commitHash, "nonexistent.txt")
		var buf bytes.Buffer
		err := CopyFile(locator, &buf)
		require.Error(t, err)
		require.Contains(t, err.Error(), "opening file")
	})

	t.Run("errors on invalid locator", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		err := CopyFile("://invalid", &buf)
		require.Error(t, err)
	})
}

func TestDownload(t *testing.T) {
	t.Parallel()

	repoDir, commitHash := initTestRepoWithFiles(t, map[string]string{
		"hello.txt":         "hello world",
		"docs/guide.md":     "# Guide",
		"docs/faq.md":       "# FAQ",
		"src/main.go":       "package main\n",
		"src/util/utils.go": "package util\n",
	})

	t.Run("downloads a single file by subpath", func(t *testing.T) {
		t.Parallel()
		destDir := t.TempDir()
		locator := fileLocator(repoDir, commitHash, "hello.txt")
		err := Download(locator, destDir)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(destDir, "hello.txt"))
		require.NoError(t, err)
		require.Equal(t, "hello world", string(content))
	})

	t.Run("downloads a directory subtree", func(t *testing.T) {
		t.Parallel()
		destDir := t.TempDir()
		locator := fileLocator(repoDir, commitHash, "docs/")
		err := Download(locator, destDir)
		require.NoError(t, err)

		guide, err := os.ReadFile(filepath.Join(destDir, "docs", "guide.md"))
		require.NoError(t, err)
		require.Equal(t, "# Guide", string(guide))

		faq, err := os.ReadFile(filepath.Join(destDir, "docs", "faq.md"))
		require.NoError(t, err)
		require.Equal(t, "# FAQ", string(faq))
	})

	t.Run("downloads nested directory subtree", func(t *testing.T) {
		t.Parallel()
		destDir := t.TempDir()
		locator := fileLocator(repoDir, commitHash, "src/")
		err := Download(locator, destDir)
		require.NoError(t, err)

		mainGo, err := os.ReadFile(filepath.Join(destDir, "src", "main.go"))
		require.NoError(t, err)
		require.Equal(t, "package main\n", string(mainGo))

		utils, err := os.ReadFile(filepath.Join(destDir, "src", "util", "utils.go"))
		require.NoError(t, err)
		require.Equal(t, "package util\n", string(utils))
	})

	t.Run("errors when no subpath", func(t *testing.T) {
		t.Parallel()
		destDir := t.TempDir()
		locator := fileLocator(repoDir, commitHash, "")
		err := Download(locator, destDir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no subpath defined")
	})

	t.Run("errors on invalid locator", func(t *testing.T) {
		t.Parallel()
		destDir := t.TempDir()
		err := Download("://invalid", destDir)
		require.Error(t, err)
	})
}
