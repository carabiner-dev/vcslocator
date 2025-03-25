# vcslocator

A library to work with SPDX VCS locator that parses and downloads data references
by locator URIs.

## Intro

This library lets go programs work with SPDX VCS locators. These are specially
crafted URI that point to content hosted in Version Control Systems. 

For reference see table 19 in the [SPDX standard documentation](https://spdx.github.io/spdx-spec/v2.3/package-information/#77-package-download-location-field).

## Features:

At the moment the focus of this module is parsing the locator strings and
downloading data from repositories as referenced by the URIs.

#### Parsing

The library includes a parser that returns the components of the VCS locator:

```golang

// Create a new locator:
l = vcslocator.Locator("git+https://github.com/example/test@v1#filename.txt")

// Parse the locator:
components, err = l.Parse()
if err != nil {
    fmt.Fprintf(os.Stderr, "Error copying file data: %s\n", err.Error())
    os.Exit(1)
}

fmt.Printf("%+v\n",  components)

/*
{
    Tool      "git"
	Transport "https"
	Hostname  "github.com"
	RepoPath  "/example/test"
	RefString "v1"
	Commit    "
	Tag       "v1"
	Branch    ""
	SubPath   "filename.txt"
}
*/

```

#### Download and Copy

The library also supports copying and downloading the data referenced by the
VCS locator:

```golang

// This VCS locator points to the readme file of Kubernetes at the latest commit:
var filelocator = "git+https://https://github.com/kubernetes/kubernetes#README.md"

// Copy the README data to STDOUT:
if err := vcslocator.CopyFile(filelocator, Stdout); err != nil {
    fmt.Fprintf(os.Stderr, "Error copying file data: %s\n", err.Error())
    os.Exit(1)
}

// This VCS locator points to the go/predicates directory in the in-toto/attestation
// repository in GitHub at commit 159ab7123302f32caeae1b8ac99a2e465ae733f8:
var dirlocator = "git+https://github.com/in-toto/attestation@159ab7123302f32caeae1b8ac99a2e465ae733f8#go/predicates"

// Mirror the contents of the directory to mydir:
if err := vcslocator.Download(dirlocator, "mydir/"); err != nil {
    fmt.Fprintf(os.Stderr, "Error downloading data: %s\n", err.Error())
    os.Exit(1)
}
```

## Install

To install simply `go get` the module into your project:

```bash
go get github.com/carabiner-dev/vcs-locator
```

## Copyright

This moduke is released under the Apache 2.0 license and copyright by Carabiner
Systems, Inc. Feel free to open issues, send pull requests to improve the module
os simply let us know how you are using it. We love feedback!
