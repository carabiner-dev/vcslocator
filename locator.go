// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

// Package vcslocator offers functions and tools to parse SPDX VCS locator strings
// and access data referenced by them.
package vcslocator

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"regexp"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/iofs"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
)

const (
	sha1Pattern      = "^[a-f0-9]{40}$"
	sha1ShortPattern = "^[a-f0-9]{7}$"

	// Supported transport strings
	TransportSSH   = "ssh"
	TransportHTTPS = "https"
	TransportFile  = "file"

	ToolGit = "git"
)

var sha1Regex, sha1ShortRegex *regexp.Regexp

// Locator is a type that wraps a VCS locator string to add functionality to it.
type Locator string

const slugRegexPattern = `^[-A-Za-z0-9_]+/[-A-Za-z0-9_]+$`

var slugRegex *regexp.Regexp

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

	if l == "" {
		return nil, errors.New("locator is an empty string")
	}

	var transportIsFile bool
	if strings.HasPrefix(string(l), TransportFile+"://") {
		transportIsFile = true
	}

	// Parse the url, pretriming the file schema if it's there
	u, err := url.Parse(strings.TrimPrefix(string(l), TransportFile+"://"))
	if err != nil {
		return nil, err
	}

	// Here, we detect if we are dealing with a github repo slug:
	if slugRegex == nil {
		slugRegex = regexp.MustCompile(slugRegexPattern)
	}
	// .. we ONLY treat is a such if there is no hostname, no scheme and....
	if u.Hostname() == "" && u.Scheme == "" && u.Path != "" {
		path, ref, _ := strings.Cut(u.Path, "@")
		// ... we have a path that matches the slug regex (org/repo)
		if slugRegex.MatchString(path) {
			tag, branch, commitSha := parseRefString(ref, opts)
			return &Components{
				Tool:      "git",
				Transport: "https",
				Hostname:  "github.com",
				RepoPath:  path,
				RefString: ref,
				Tag:       tag,
				Branch:    branch,
				Commit:    commitSha,
				SubPath:   u.Fragment,
			}, nil
		}
	}

	// Cut the ref from the path
	path, ref, _ := strings.Cut(u.Path, "@")

	tool, transport, si := strings.Cut(u.Scheme, "+")
	// Synth the file schema to capture all into the path early
	if transportIsFile {
		transport = TransportFile
		tool = "git"
		si = true
	}

	if !si {
		transport = tool
		if transport != TransportHTTPS && transport != TransportSSH && transport != TransportFile {
			return nil, fmt.Errorf("only locators with a https, ssh or file transport are supported")
		}
		tool = ""
	}

	tag, branch, commitSha := parseRefString(ref, opts)
	hostname := u.Hostname()

	// If there is a hostname in a file URI, prepend it to the path
	if transport == TransportFile && hostname != "" {
		if path == "" {
			path = u.Hostname()
		} else {
			path = u.Hostname() + "/" + strings.TrimPrefix(path, "/")
		}
		hostname = ""
	}

	if path == "" && transport == TransportFile {
		return nil, fmt.Errorf("unable to parse path from file:// locator")
	}

	return &Components{
		Tool:      tool,
		Transport: transport,
		Hostname:  hostname,
		RepoPath:  path,
		RefString: ref,
		Tag:       tag,
		Branch:    branch,
		Commit:    commitSha,
		SubPath:   u.Fragment,
	}, nil
}

// parseRefString parses a reference string and tries to determine if its a
// branch, a tag or a commit.
//
//	// TODO(puerco): Ensure this follows `man gitrevisions` > SPECIFYING REVISIONS
func parseRefString(ref string, opts options) (tag, branch, commitSha string) {
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
	case commitSha == "" && !opts.RefIsBranch && !strings.HasPrefix(ref, "refs/"):
		tag = ref
	}

	return tag, branch, commitSha
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

	// Branches and tags are safe to fetch when cloning. This is not the case
	// of notes, for example so we only pass a reference to clone if we're
	// dealing with a brach or tag.
	var reference plumbing.ReferenceName
	switch {
	case components.Branch != "":
		reference = plumbing.NewBranchReferenceName(components.Branch)
	case components.Tag != "":
		reference = plumbing.NewTagReferenceName(components.Tag)
	}

	var fsobj billy.Filesystem
	if opts.ClonePath == "" {
		fsobj = memfs.New()
	} else {
		fsobj = osfs.New(opts.ClonePath)
	}

	// Handle cloning from repos with file: transport
	repourl := components.RepoURL()
	if components.Transport == "file" {
		repourl = components.RepoPath
	}

	var auth transport.AuthMethod
	if opts.ReadCredentials {
		auth, err = GetAuthMethod(l)
		if err != nil {
			return nil, fmt.Errorf("getting git auth method: %w", err)
		}
	}

	// Make a shallow clone of the repo to memory
	repo, err := git.Clone(memory.NewStorage(), fsobj, &git.CloneOptions{
		URL:  repourl,
		Auth: auth,
		// Progress:      os.Stdout,
		ReferenceName: reference,
		SingleBranch:  true,
		// RecurseSubmodules: 0,
		// ShallowSubmodules: false,
	})
	if err != nil {
		return nil, fmt.Errorf("cloning repo: %w", err)
	}

	commitHash := components.Commit
	// Here we handle commits and other references (not tags or branches)
	if reference == "" && components.Commit == "" {
		// But also ensuring we are note refetching a previous commit
		if components.RefString != "" && components.RefString != components.Commit {
			// Since this ref was not fetched at clone time, we do a fetch here
			// to make sure it is available. This is especially important for
			// git notes that are never transferred by default and cannot be
			// fetched at clone time, I thing because of a bug that somewhere
			// changes the ref string from refs/notes/commits to refs/heads/notes/commits
			//
			if err := repo.Fetch(&git.FetchOptions{
				RefSpecs: []config.RefSpec{
					config.RefSpec(fmt.Sprintf("%s:%s", components.RefString, components.RefString)),
				},
			}); err != nil {
				return nil, fmt.Errorf("late fetching ref %q: %w", components.RefString, err)
			}

			// Resolve the reference, it should not fail as we fetched it already
			ref, err := repo.Reference(plumbing.ReferenceName(components.RefString), true)
			if err != nil {
				return nil, fmt.Errorf("resolving reference %q: %w", components.RefString, err)
			}

			// Resolve the reference to a commit hash
			hach, err := repo.ResolveRevision(plumbing.Revision(ref.Name().String()))
			if err != nil {
				return nil, fmt.Errorf("resolving latest revision on %q to commit: %w", ref.Name().String(), err)
			}
			commitHash = hach.String()
		}
	}

	// If a revision was specified, check it out
	if commitHash != "" {
		wt, err := repo.Worktree()
		if err != nil {
			return nil, fmt.Errorf("getting repository worktree: %w", err)
		}

		if err = wt.Checkout(&git.CheckoutOptions{
			Hash: plumbing.NewHash(commitHash),
		}); err != nil {
			return nil, fmt.Errorf("checking out commit %s: %w", commitHash, err)
		}
	}

	return iofs.New(fsobj), nil
}
