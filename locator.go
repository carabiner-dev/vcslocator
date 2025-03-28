// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

// Package vcslocator offers functions and tools to parse SPDX VCS locator strings
// and access data referenced by them.
package vcslocator

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/iofs"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
)

const (
	sha1Pattern      = "^[a-f0-9]{40}$"
	sha1ShortPattern = "^[a-f0-9]{7}$"
)

var sha1Regex, sha1ShortRegex *regexp.Regexp

// Locator is a type that wraps a VCS locator string to add functionality to it.
type Locator string

// Parse a VCS locator and returns its components
func (l Locator) Parse(funcs ...fnOpt) (*Components, error) {
	// For reference, the format is:
	// <vcs_tool>+<transport>://<host_name>[/<path_to_repository>][@<revision_tag_or_branch>][#<sub_path>]
	opts := defaultOptions
	for _, fn := range funcs {
		if err := fn(&opts); err != nil {
			return nil, err
		}
	}

	u, err := url.Parse(string(l))
	if err != nil {
		return nil, err
	}

	var commitSha, tag, branch string
	path, ref, _ := strings.Cut(u.Path, "@")

	tool, transport, si := strings.Cut(u.Scheme, "+")
	if !si {
		transport = tool
		tool = ""
	}

	if ref != "" {
		if sha1Regex == nil || sha1ShortRegex == nil {
			sha1Regex = regexp.MustCompile(sha1Pattern)
			sha1ShortRegex = regexp.MustCompile(sha1ShortPattern)
		}

		// If the ref looks like a commit, we treat it as such. Other reference
		// types can be addressed by specifying the full path string (ie refs/tags/XX).
		if sha1Regex.MatchString(ref) || sha1ShortRegex.MatchString(ref) {
			commitSha = ref
		}

		switch {
		case strings.HasPrefix(ref, "refs/tags/"):
			tag = strings.TrimPrefix(ref, "refs/tags/")
		case strings.HasPrefix(ref, "refs/heads/"):
			branch = strings.TrimPrefix(ref, "refs/heads/")
		case commitSha == "" && opts.RefIsBranch:
			branch = ref
		case commitSha == "" && !opts.RefIsBranch:
			tag = ref
		}
	}

	return &Components{
		Tool:      tool,
		Transport: transport,
		Hostname:  u.Hostname(),
		RepoPath:  path,
		RefString: ref,
		Tag:       tag,
		Branch:    branch,
		Commit:    commitSha,
		SubPath:   u.Fragment,
	}, nil
}

// CopyFile downloads a file specified by the VCS locator and copies it
// to an io.Writer.
func CopyFile[T ~string](locator T, w io.Writer, funcs ...fnOpt) error {
	opts := defaultOptions
	for _, fn := range funcs {
		if err := fn(&opts); err != nil {
			return err
		}
	}

	l := Locator(locator)
	components, err := l.Parse(funcs...)
	if err != nil {
		return fmt.Errorf("parsing locator: %w", err)
	}
	if components.SubPath == "" {
		return errors.New("locator has no subpath defined")
	}

	fs, err := CloneRepository(locator, funcs...)
	if err != nil {
		return fmt.Errorf("cloning repository: %w", err)
	}

	f, err := fs.Open(components.SubPath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("copying data stream: %w", err)
	}
	return nil
}

// Download copies data from the git repository to the specified directory
func Download[T ~string](locator T, localDir string, funcs ...fnOpt) error {
	opts := defaultOptions
	for _, fn := range funcs {
		if err := fn(&opts); err != nil {
			return err
		}
	}

	l := Locator(locator)

	components, err := l.Parse(funcs...)
	if err != nil {
		return fmt.Errorf("parsing locator: %w", err)
	}
	if components.SubPath == "" {
		return errors.New("locator has no subpath defined")
	}

	fsys, err := CloneRepository(locator, funcs...)
	if err != nil {
		return fmt.Errorf("cloning repository: %w", err)
	}

	// Walk the filesystem to fetch all we need
	if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Fatal(err)
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasPrefix(path, strings.TrimPrefix(components.SubPath, "/")) {
			return nil
		}

		// We know all paths are files here, so we create the dir and copy
		src, err := fsys.Open(path)
		if err != nil {
			return fmt.Errorf("opening file from source: %w", err)
		}
		defer src.Close() //nolint:errcheck

		destDir := filepath.Join(localDir, filepath.Dir(path))
		if err := os.MkdirAll(destDir, os.FileMode(0o755)); err != nil {
			return fmt.Errorf("creating destination dir: %w", err)
		}

		dst, err := os.Create(filepath.Join(localDir, path)) //nolint: gosec
		if err != nil {
			return fmt.Errorf("opening destination file: %w", err)
		}
		defer dst.Close() //nolint:errcheck

		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("copying data stream: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// CloneRepository clones the repository defined by the locator to a path.
func CloneRepository[T ~string](locator T, funcs ...fnOpt) (fs.FS, error) {
	opts := defaultOptions
	for _, fn := range funcs {
		if err := fn(&opts); err != nil {
			return nil, err
		}
	}

	// Create the locator and parse
	l := Locator(locator)
	components, err := l.Parse()
	if err != nil {
		return nil, fmt.Errorf("parsing locator: %w", err)
	}

	if components.Tool != "git" {
		return nil, errors.New("only git locators are supported for cloning")
	}

	var reference plumbing.ReferenceName
	if components.Branch != "" {
		reference = plumbing.NewBranchReferenceName(components.Branch)
	} else if components.Tag != "" {
		reference = plumbing.NewTagReferenceName(components.Tag)
	}

	var fs billy.Filesystem
	if opts.ClonePath == "" {
		fs = memfs.New()
	} else {
		fs = osfs.New(opts.ClonePath)
	}

	// Make a shallow clone of the repo to memory
	repo, err := git.Clone(memory.NewStorage(), fs, &git.CloneOptions{
		URL: components.RepoURL(),
		// Progress:      os.Stdout,
		ReferenceName: reference,
		SingleBranch:  true,
		// RecurseSubmodules: 0,
		// ShallowSubmodules: false,
	})
	if err != nil {
		return nil, fmt.Errorf("cloning repo: %w", err)
	}

	// If a revision was specified, check it out
	if components.Commit != "" {
		wt, err := repo.Worktree()
		if err != nil {
			return nil, fmt.Errorf("getting repository worktree: %w", err)
		}

		if err = wt.Checkout(&git.CheckoutOptions{
			Hash: plumbing.NewHash(components.Commit),
		}); err != nil {
			return nil, fmt.Errorf("checking out commit: %w", err)
		}
	}

	return iofs.New(fs), nil
}
