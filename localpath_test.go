// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package vcslocator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFromPath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		path string
		want Locator
	}{
		{"posix-absolute", "/tmp/repo", Locator("file:///tmp/repo")},
		{"posix-nested", "/home/user/projects/repo", Locator("file:///home/user/projects/repo")},
		{"posix-relative", "repo", Locator("file:///repo")},
		{"windows-backslash", `C:\Users\x\repo`, Locator("file:///C:/Users/x/repo")},
		{"windows-forwardslash", "C:/Users/x/repo", Locator("file:///C:/Users/x/repo")},
		{"windows-lowercase-drive", `d:\repo`, Locator("file:///d:/repo")},
		{"windows-mixed-slashes", `C:\Users/x\repo`, Locator("file:///C:/Users/x/repo")},
		{"empty", "", Locator("file://")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, NewFromPath(tc.path))
		})
	}
}

// TestNewFromPathRoundTrip ensures NewFromPath and Locator.LocalPath are
// inverses: feeding LocalPath's output back through NewFromPath (and vice
// versa) produces a stable value. This guards the platform-specific slash
// handling since both operations must agree on the Windows drive-letter
// convention.
func TestNewFromPathRoundTrip(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		path string
	}{
		{"posix", "/tmp/repo"},
		{"windows-drive", "C:/Users/x/repo"},
		{"windows-lowercase", "d:/repo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			loc := NewFromPath(tc.path)
			got, err := loc.LocalPath()
			require.NoError(t, err)
			require.Equal(t, tc.path, got)
		})
	}
}

func TestLocatorLocalPath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		locator Locator
		want    string
		mustErr bool
	}{
		// file:// URLs must return a usable native filesystem path on every
		// platform — go-git, os.Open, etc. treat the leading "/C:/..." form
		// as drive-relative on Windows, so it must be stripped.
		{"posix-absolute", Locator("file:///tmp/repo"), "/tmp/repo", false},
		{"posix-with-ref", Locator("file:///tmp/repo@main"), "/tmp/repo", false},
		{"posix-with-subpath", Locator("file:///tmp/repo#sub"), "/tmp/repo", false},
		{"windows-drive", Locator("file:///C:/Users/x/repo"), "C:/Users/x/repo", false},
		{"windows-drive-lowercase", Locator("file:///d:/repo"), "d:/repo", false},
		{"windows-drive-with-ref", Locator("file:///C:/repo@abc123"), "C:/repo", false},
		{"windows-drive-with-subpath", Locator("file:///C:/repo#sub/dir"), "C:/repo", false},

		// Non-file transports: LocalPath is not meaningful, return "".
		{"https", Locator("https://github.com/example/test"), "", false},
		{"ssh", Locator("ssh://git@github.com/example/test"), "", false},
		{"slug", Locator("example/test"), "", false},

		// Parse errors propagate.
		{"empty", Locator(""), "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.locator.LocalPath()
			if tc.mustErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
