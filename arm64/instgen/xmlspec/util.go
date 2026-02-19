// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xmlspec

// This file contains utility functions.

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// GetArm64XMLSpec downloads the ARM64 XML spec from the given URL to a temporary directory.
// It returns the path to directory containing all instruction XML files.
// If anything goes wrong, it will return an error.
func GetArm64XMLSpec(tmpDir string, url string, version string) (string, error) {
	if err := downloadArm64XMLSpec(tmpDir, url); err != nil {
		return "", fmt.Errorf("downloadArm64XMLSpec failed: %v", err)
	}

	// The tarball extracts to a directory like "ISA_A64_xml_A_profile-2025-12".
	// We need to find it.
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return "", fmt.Errorf("os.ReadDir failed: %v", err)
	}

	var xmlDir string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), version) {
			xmlDir = filepath.Join(tmpDir, e.Name())
			break
		}
	}

	if xmlDir == "" {
		return "", fmt.Errorf("could not find extracted XML directory in %s", tmpDir)
	}
	return xmlDir, nil
}

// downloadArm64XMLSpec downloads the ARM64 XML spec from the given URL to the given directory.
func downloadArm64XMLSpec(dir string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("fetching ARM64 XML spec from %s failed: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching ARM64 XML spec from %s returned status: %s", url, resp.Status)
	}

	if err := extractTarGz(resp.Body, dir); err != nil {
		return err
	}
	return nil
}

// extractTarGz extracts the tar.gz file to the given directory.
func extractTarGz(r io.Reader, dir string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	// Iterate over the entries in the tarball.
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dir, header.Name)

		switch header.Typeflag {
		// directories
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		// regular files
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}

const ExpectedURL = "https://developer.arm.com/-/cdn-downloads/permalink/Exploration-Tools-A64-ISA/ISA_A64/ISA_A64_xml_A_profile-2025-12.tar.gz"
const ExpectedVersion = "ISA_A64_xml_A_profile-2025-12"
