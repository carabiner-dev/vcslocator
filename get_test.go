// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package vcslocator

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
)

// fileLocator builds a file:// locator string that works on all platforms.
// On Unix, paths are absolute (/tmp/...) so file:// + path gives file:///tmp/...
// On Windows, paths like D:\... need a leading slash after file:// to prevent
// the drive letter from being parsed as a URL scheme (file:///D:/...).
func fileLocator(repoDir, commitHash, fragment string) string {
	p := filepath.ToSlash(repoDir)
	// Ensure the path starts with / so the drive letter isn't a URL scheme.
	if p != "" && p[0] != '/' {
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

	noAuth := WithSystemCredentials(false)

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
		err := CopyFile(locator, &buf, noAuth)
		require.NoError(t, err)
		require.Equal(t, "hello world", buf.String())
	})

	t.Run("copies a nested file", func(t *testing.T) {
		t.Parallel()
		locator := fileLocator(repoDir, commitHash, "docs/guide.md")
		var buf bytes.Buffer
		err := CopyFile(locator, &buf, noAuth)
		require.NoError(t, err)
		require.Equal(t, "# Guide\nSome content.", buf.String())
	})

	t.Run("errors when no subpath", func(t *testing.T) {
		t.Parallel()
		locator := fileLocator(repoDir, commitHash, "")
		var buf bytes.Buffer
		err := CopyFile(locator, &buf, noAuth)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no subpath defined")
	})

	t.Run("errors when file does not exist", func(t *testing.T) {
		t.Parallel()
		locator := fileLocator(repoDir, commitHash, "nonexistent.txt")
		var buf bytes.Buffer
		err := CopyFile(locator, &buf, noAuth)
		require.Error(t, err)
		require.Contains(t, err.Error(), "opening file")
	})

	t.Run("errors on invalid locator", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		err := CopyFile("://invalid", &buf, noAuth)
		require.Error(t, err)
	})
}

func TestDownload(t *testing.T) {
	t.Parallel()

	noAuth := WithSystemCredentials(false)

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
		err := Download(locator, destDir, noAuth)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(destDir, "hello.txt"))
		require.NoError(t, err)
		require.Equal(t, "hello world", string(content))
	})

	t.Run("downloads a directory subtree", func(t *testing.T) {
		t.Parallel()
		destDir := t.TempDir()
		locator := fileLocator(repoDir, commitHash, "docs/")
		err := Download(locator, destDir, noAuth)
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
		err := Download(locator, destDir, noAuth)
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
		err := Download(locator, destDir, noAuth)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no subpath defined")
	})

	t.Run("errors on invalid locator", func(t *testing.T) {
		t.Parallel()
		destDir := t.TempDir()
		err := Download("://invalid", destDir, noAuth)
		require.Error(t, err)
	})
}

// addNote stores content as the git note of commitHash in the repository at
// repoDir, creating the notes reference if needed.
func addNote(t *testing.T, repoDir, commitHash, content string) {
	t.Helper()
	repo, err := git.PlainOpen(repoDir)
	require.NoError(t, err)

	blob := repo.Storer.NewEncodedObject()
	blob.SetType(plumbing.BlobObject)
	w, err := blob.Writer()
	require.NoError(t, err)
	_, err = w.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	blobHash, err := repo.Storer.SetEncodedObject(blob)
	require.NoError(t, err)

	tree := &object.Tree{Entries: []object.TreeEntry{
		{Name: commitHash, Mode: filemode.Regular, Hash: blobHash},
	}}
	treeObj := repo.Storer.NewEncodedObject()
	require.NoError(t, tree.Encode(treeObj))
	treeHash, err := repo.Storer.SetEncodedObject(treeObj)
	require.NoError(t, err)

	sig := object.Signature{Name: "test", Email: "test@test.com", When: time.Now()}
	commit := &object.Commit{Author: sig, Committer: sig, Message: "Notes added by test", TreeHash: treeHash}
	commitObj := repo.Storer.NewEncodedObject()
	require.NoError(t, commit.Encode(commitObj))
	notesHash, err := repo.Storer.SetEncodedObject(commitObj)
	require.NoError(t, err)

	require.NoError(t, repo.Storer.SetReference(
		plumbing.NewHashReference(plumbing.ReferenceName("refs/notes/commits"), notesHash),
	))
}

func TestErrorListUnwrap(t *testing.T) {
	t.Parallel()
	inner := errors.New("inner failure")
	list := &ErrorList{Errors: []error{nil, fmt.Errorf("locator 1: %w", inner), nil}}
	wrapped := fmt.Errorf("outer: %w", list)

	require.ErrorIs(t, wrapped, inner)
	require.NotErrorIs(t, wrapped, ErrRefNotFound)
	require.Len(t, list.Unwrap(), 1)
	require.Equal(t, "locator 1: inner failure", list.Error())

	var found *ErrorList
	require.ErrorAs(t, wrapped, &found)
	require.Len(t, found.Errors, 3)
}

func TestCloneRepositoryMissingRef(t *testing.T) {
	t.Parallel()
	noAuth := WithSystemCredentials(false)
	repoDir, commitHash := initTestRepoWithFiles(t, map[string]string{"hello.txt": "hello"})

	for _, tc := range []struct {
		name string
		ref  string
	}{
		{"missing notes ref", "refs/notes/commits"},
		{"missing branch", "refs/heads/nope"},
		{"missing tag", "refs/tags/nope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := CloneRepository(fileLocator(repoDir, tc.ref, "hello.txt"), noAuth)
			require.ErrorIs(t, err, ErrRefNotFound)
		})
	}

	t.Run("existing ref", func(t *testing.T) {
		t.Parallel()
		_, err := CloneRepository(fileLocator(repoDir, commitHash, "hello.txt"), noAuth)
		require.NoError(t, err)
	})
}

// TestCopyFileGroupNotes reads a commit note the way the attestation
// collectors do: one locator for the sharded path and one for the flat
// path, before and after the notes reference exists.
func TestCopyFileGroupNotes(t *testing.T) {
	t.Parallel()
	noAuth := WithSystemCredentials(false)
	repoDir, commitHash := initTestRepoWithFiles(t, map[string]string{"hello.txt": "hello"})
	locators := []string{
		fileLocator(repoDir, "refs/notes/commits", commitHash[0:2]+"/"+commitHash[2:]),
		fileLocator(repoDir, "refs/notes/commits", commitHash),
	}

	// Without a notes reference the clone fails and every locator reports it
	var shard, flat bytes.Buffer
	err := CopyFileGroup(locators, []io.Writer{&shard, &flat}, noAuth)
	require.ErrorIs(t, err, ErrRefNotFound)
	var list *ErrorList
	require.ErrorAs(t, err, &list)
	require.Len(t, list.Errors, 2)
	for _, e := range list.Errors {
		require.ErrorIs(t, e, ErrRefNotFound)
	}

	// With a note, the flat locator returns it and the sharded path is
	// reported as missing, which callers can check with errors.Is
	addNote(t, repoDir, commitHash, "the note")
	shard.Reset()
	flat.Reset()
	err = CopyFileGroup(locators, []io.Writer{&shard, &flat}, noAuth)
	require.ErrorIs(t, err, fs.ErrNotExist)
	require.NotErrorIs(t, err, ErrRefNotFound)
	require.ErrorAs(t, err, &list)
	require.Len(t, list.Errors, 2)
	require.ErrorIs(t, list.Errors[0], fs.ErrNotExist)
	require.NoError(t, list.Errors[1])
	require.Empty(t, shard.String())
	require.Equal(t, "the note", flat.String())
}
