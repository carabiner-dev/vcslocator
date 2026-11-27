// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package vcslocator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
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

// addBranch creates a branch with one extra file committed on it and
// returns to the previous branch.
func addBranch(t *testing.T, repoDir, branch, file, content string) {
	t.Helper()
	repo, err := git.PlainOpen(repoDir)
	require.NoError(t, err)
	head, err := repo.Head()
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)

	require.NoError(t, wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch), Create: true,
	}))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, file), []byte(content), 0o600))
	_, err = wt.Add(file)
	require.NoError(t, err)
	_, err = wt.Commit("branch commit", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Now()},
	})
	require.NoError(t, err)
	require.NoError(t, wt.Checkout(&git.CheckoutOptions{Branch: head.Name()}))
}

func TestCloneOptionsAreHonored(t *testing.T) {
	t.Parallel()
	noAuth := WithSystemCredentials(false)
	repoDir, commitHash := initTestRepoWithFiles(t, map[string]string{"hello.txt": "hello"})
	addBranch(t, repoDir, "feature", "feature.txt", "on feature")

	t.Run("ref as branch", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		locator := fileLocator(repoDir, "feature", "feature.txt")

		// By default the ref is a tag, which does not exist
		require.ErrorIs(t, CopyFile(locator, &buf, noAuth), ErrRefNotFound)

		require.NoError(t, CopyFile(locator, &buf, noAuth, WithRefAsBranch(true)))
		require.Equal(t, "on feature", buf.String())

		buf.Reset()
		require.NoError(t, CopyFileGroup([]string{locator}, []io.Writer{&buf}, noAuth, WithRefAsBranch(true)))
		require.Equal(t, "on feature", buf.String())
	})

	t.Run("short commit sha", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, CopyFile(fileLocator(repoDir, commitHash[0:7], "hello.txt"), &buf, noAuth))
		require.Equal(t, "hello", buf.String())
	})

	t.Run("subpath with leading slash", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		require.NoError(t, CopyFile(fileLocator(repoDir, commitHash, "/hello.txt"), &buf, noAuth))
		require.Equal(t, "hello", buf.String())
	})

	t.Run("http credentials reach the clone auth", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions
		require.NoError(t, WithHttpAuth("user", "secret")(&opts))
		auth, err := authForTransport(TransportHTTPS, &opts)
		require.NoError(t, err)
		require.Equal(t, "http-basic-auth", auth.Name())

		auth, err = authForTransport(TransportFile, &opts)
		require.NoError(t, err)
		require.Nil(t, auth)

		auth, err = authForTransport(TransportHTTPS, &defaultOptions)
		require.NoError(t, err)
		require.Nil(t, auth)
	})
}

func TestCopyFileGroupSeparatesLocalRepos(t *testing.T) {
	t.Parallel()
	noAuth := WithSystemCredentials(false)
	repoA, shaA := initTestRepoWithFiles(t, map[string]string{"file.txt": "from A"})
	repoB, shaB := initTestRepoWithFiles(t, map[string]string{"file.txt": "from B"})

	// Two local repos read at the same ref must not share a clone
	var a, b bytes.Buffer
	require.NoError(t, CopyFileGroup([]string{
		fileLocator(repoA, "refs/heads/master", "file.txt"),
		fileLocator(repoB, "refs/heads/master", "file.txt"),
	}, []io.Writer{&a, &b}, noAuth))
	require.Equal(t, "from A", a.String())
	require.Equal(t, "from B", b.String())

	components, err := Locator(fileLocator(repoA, shaA, "file.txt")).Parse()
	require.NoError(t, err)
	require.Equal(t, "file://"+components.RepoPath, components.RepoURL())
	require.NotEqual(t, shaA, shaB)

	// Parse failures name the locator and keep the cause
	err = CopyFileGroup([]string{"://invalid"}, []io.Writer{&a}, noAuth)
	require.ErrorContains(t, err, "parsing locator 0")
	require.ErrorContains(t, err, "missing protocol scheme")
}

func TestDownloadSubpathBoundary(t *testing.T) {
	t.Parallel()
	noAuth := WithSystemCredentials(false)
	repoDir, commitHash := initTestRepoWithFiles(t, map[string]string{
		"go/pred/inside.txt":      "inside",
		"go/predicates/other.txt": "sibling with the same prefix",
		"go/pred-other/other.txt": "another sibling",
	})

	for _, sub := range []string{"go/pred", "go/pred/", "/go/pred"} {
		destDir := t.TempDir()
		require.NoError(t, Download(fileLocator(repoDir, commitHash, sub), destDir, noAuth), sub)
		content, err := os.ReadFile(filepath.Join(destDir, "go", "pred", "inside.txt"))
		require.NoError(t, err, sub)
		require.Equal(t, "inside", string(content), sub)
		require.NoFileExists(t, filepath.Join(destDir, "go", "predicates", "other.txt"), sub)
		require.NoFileExists(t, filepath.Join(destDir, "go", "pred-other", "other.txt"), sub)
	}
}

// TestWithContext proves the context option reaches the clone behind every
// copy entry point: a cancelled context fails with context.Canceled before the
// remote is contacted, while a live context leaves the call working.
func TestWithContext(t *testing.T) {
	t.Parallel()

	noAuth := WithSystemCredentials(false)
	repoDir, commitHash := initTestRepoWithFiles(t, map[string]string{"hello.txt": "hello"})
	local := fileLocator(repoDir, commitHash, "hello.txt")

	// An unreachable remote: any error other than the context's own means
	// the context never reached the clone.
	remote := "git+https://127.0.0.1:1/org/repo@" + commitHash + "#hello.txt"

	live := t.Context()
	cancelled, cancel := context.WithCancel(live)
	cancel()

	for name, call := range map[string]func(locator string, opts ...fnOpt) error{
		"CloneRepository": func(locator string, opts ...fnOpt) error {
			_, err := CloneRepository(locator, opts...)

			return err
		},
		"CopyFile": func(locator string, opts ...fnOpt) error {
			return CopyFile(locator, io.Discard, opts...)
		},
		"CopyFileGroup": func(locator string, opts ...fnOpt) error {
			return CopyFileGroup([]string{locator}, []io.Writer{io.Discard}, opts...)
		},
		"GetGroup": func(locator string, opts ...fnOpt) error {
			_, err := GetGroup([]string{locator}, opts...)

			return err
		},
		"Download": func(locator string, opts ...fnOpt) error {
			return Download(locator, t.TempDir(), opts...)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, call(remote, noAuth, WithContext(cancelled)), context.Canceled)
			require.NoError(t, call(local, noAuth, WithContext(live)))
		})
	}
}

// TestWithContextRejectsNil proves a nil context is refused at option time
// rather than panicking deep inside go-git.
func TestWithContextRejectsNil(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	//nolint:staticcheck // passing a nil context on purpose
	err := CopyFile("git+https://github.com/org/repo#file", &buf, WithContext(nil))
	require.ErrorContains(t, err, "context is nil")
}

// addCommit commits a new version of a file on the current branch and
// returns the hash of the commit.
func addCommit(t *testing.T, repoDir, file, content string) string {
	t.Helper()
	repo, err := git.PlainOpen(repoDir)
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, file), []byte(content), 0o600))
	_, err = wt.Add(file)
	require.NoError(t, err)
	hash, err := wt.Commit("another commit", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Now()},
	})
	require.NoError(t, err)

	return hash.String()
}

// allowFetchingCommits configures a repository to serve the commits its
// references lead to when they are requested by hash, as GitHub does. A git
// server refuses those requests by default.
func allowFetchingCommits(t *testing.T, repoDir string) {
	t.Helper()
	repo, err := git.PlainOpen(repoDir)
	require.NoError(t, err)
	cfg, err := repo.Config()
	require.NoError(t, err)
	cfg.Raw.Section("uploadpack").SetOption("allowReachableSHA1InWant", "true")
	require.NoError(t, repo.SetConfig(cfg))
}

// TestClonePinnedCommit reads a file at a commit that is no longer the tip of
// its branch, from a remote that serves commits by hash and from one that
// does not. Both must return the file as it was at that commit: the first by
// fetching the commit alone, the second by falling back to a clone.
func TestClonePinnedCommit(t *testing.T) {
	t.Parallel()
	noAuth := WithSystemCredentials(false)

	for _, tc := range []struct {
		name          string
		servesCommits bool
	}{
		{"remote serves commits by hash", true},
		{"remote refuses commits by hash", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repoDir, pinned := initTestRepoWithFiles(t, map[string]string{"hello.txt": "first"})
			tip := addCommit(t, repoDir, "hello.txt", "second")
			if tc.servesCommits {
				allowFetchingCommits(t, repoDir)
			}

			// The commit is fetched on its own only when the remote allows it,
			// and then without the history around it.
			components, err := Locator(fileLocator(repoDir, pinned, "")).Parse()
			require.NoError(t, err)
			repo, err := fetchCommit(t.Context(), memfs.New(), components.RepoURL(), nil, pinned)
			require.NoError(t, err)
			if tc.servesCommits {
				require.NotNil(t, repo)
				_, err = repo.CommitObject(plumbing.NewHash(pinned))
				require.NoError(t, err)
				_, err = repo.CommitObject(plumbing.NewHash(tip))
				require.ErrorIs(t, err, plumbing.ErrObjectNotFound)
			} else {
				require.Nil(t, repo)
			}

			// Either way the locator reads the file at the pinned commit.
			var buf bytes.Buffer
			require.NoError(t, CopyFile(fileLocator(repoDir, pinned, "hello.txt"), &buf, noAuth))
			require.Equal(t, "first", buf.String())

			buf.Reset()
			require.NoError(t, CopyFile(fileLocator(repoDir, tip, "hello.txt"), &buf, noAuth))
			require.Equal(t, "second", buf.String())

			// A clone path on disk receives the files of the pinned commit.
			dir := t.TempDir()
			_, err = CloneRepository(fileLocator(repoDir, pinned, ""), noAuth, WithClonePath(dir))
			require.NoError(t, err)
			data, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
			require.NoError(t, err)
			require.Equal(t, "first", string(data))
		})
	}

	t.Run("unknown commit", func(t *testing.T) {
		t.Parallel()
		repoDir, _ := initTestRepoWithFiles(t, map[string]string{"hello.txt": "first"})
		allowFetchingCommits(t, repoDir)
		_, err := CloneRepository(
			fileLocator(repoDir, "0123456789abcdef0123456789abcdef01234567", "hello.txt"), noAuth,
		)
		require.Error(t, err)
	})
}

// TestClonePinnedCommitOffDefaultBranch pins a commit that only a side branch
// leads to. Cloning the default branch never brings that commit in, so
// reading it proves CloneRepository fetched the commit on its own; a remote
// that refuses to serve commits by hash leaves only the clone, which fails.
func TestClonePinnedCommitOffDefaultBranch(t *testing.T) {
	t.Parallel()
	noAuth := WithSystemCredentials(false)

	for _, tc := range []struct {
		name          string
		servesCommits bool
	}{
		{"remote serves commits by hash", true},
		{"remote refuses commits by hash", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repoDir, _ := initTestRepoWithFiles(t, map[string]string{"hello.txt": "hello"})
			addBranch(t, repoDir, "feature", "feature.txt", "on feature")
			if tc.servesCommits {
				allowFetchingCommits(t, repoDir)
			}

			repo, err := git.PlainOpen(repoDir)
			require.NoError(t, err)
			ref, err := repo.Reference(plumbing.NewBranchReferenceName("feature"), true)
			require.NoError(t, err)
			locator := fileLocator(repoDir, ref.Hash().String(), "feature.txt")

			var buf bytes.Buffer
			err = CopyFile(locator, &buf, noAuth)
			if !tc.servesCommits {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "on feature", buf.String())

			buf.Reset()
			require.NoError(t, CopyFileGroup([]string{locator}, []io.Writer{&buf}, noAuth))
			require.Equal(t, "on feature", buf.String())
		})
	}
}
