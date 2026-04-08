// Demo program showing how to use vcslocator.Download to clone and extract
// files from a VCS locator string. Pass a locator as the first argument
// (e.g. "git+https://github.com/org/repo@tag#path/") and the referenced
// content will be downloaded to /tmp/test.
package main

import (
	"fmt"
	"os"

	vcslocator "github.com/carabiner-dev/vcslocator"
)

func main() {
	//nolint:gocritic // example code kept for reference
	// f, _ := os.Create("/tmp/test.file")
	if len(os.Args) < 2 {
		fmt.Println("no vcs locator sepcified")
		os.Exit(1)
	}

	if err := vcslocator.Download(os.Args[1], "/tmp/test"); err != nil {
		fmt.Println("Error: " + err.Error())
	}

	//nolint:gocritic // example code kept for reference
	// if err := vcslocator.DownloadFile(os.Args[1], f); err != nil {
	// 	fmt.Println("Error: " + err.Error())
	// 	os.Remove(f.Name())
	// }
}
