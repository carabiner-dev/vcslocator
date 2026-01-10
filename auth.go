// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package vcslocator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// getAuthMethod returns an appropriate auth method based on the transport type
// and available credentials.
//
// It mimics git's behavior by automatically detecting and using SSH keys, SSH
// agent, or configuring http credentials from the options.
func GetAuthMethod[T ~string](locator T, funcs ...fnOpt) (transport.AuthMethod, error) {
	opts := defaultOptions
	for _, fn := range funcs {
		if err := fn(&opts); err != nil {
			return nil, err
		}
	}

	l := Locator(locator)
	components, err := l.Parse()
	if err != nil {
		return nil, err
	}

	switch components.Transport {
	case TransportSSH:
		return getSSHAuth()
	case TransportHTTPS:
		return getHTTPAuth(opts), nil
	case TransportFile:
		return nil, nil // No auth needed for local file:// repos
	default:
		return nil, nil
	}
}

// getSSHAuth returns SSH authentication, trying in order:
// 1. SSH agent
// 2. Default SSH keys (~/.ssh/id_rsa, ~/.ssh/id_ed25519, ~/.ssh/id_ecdsa)
func getSSHAuth() (transport.AuthMethod, error) {
	// Try SSH agent first (like git does)
	auth, err := ssh.NewSSHAgentAuth("git")
	if err == nil {
		return auth, nil
	}

	// Try common SSH key locations
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	sshDir := filepath.Join(homeDir, ".ssh")

	// Try keys in order of preference (same as git)
	keyFiles := []string{
		"id_ed25519", "id_ecdsa", "id_rsa", "id_dsa",
	}

	var lastErr error
	for _, keyFile := range keyFiles {
		keyPath := filepath.Join(sshDir, keyFile)

		// Check if key file exists
		if _, err := os.Stat(keyPath); err != nil {
			continue
		}

		// Try to load the key (without password first)
		auth, err := ssh.NewPublicKeysFromFile("git", keyPath, "")
		if err == nil {
			return auth, nil
		}

		lastErr = err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("no usable SSH keys found: %w", lastErr)
	}

	return nil, errors.New("no SSH authentication method available")
}

// getHTTPAuth returns HTTP an authenticator using the credentials configured
// in the options
func getHTTPAuth(opts options) transport.AuthMethod {
	if opts.HttpPassword == "" && opts.HttpUsername == "" {
		return nil
	}

	return &http.BasicAuth{
		Username: opts.HttpUsername,
		Password: opts.HttpPassword,
	}
}
