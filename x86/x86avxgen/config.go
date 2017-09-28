// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "path/filepath"

const (
	progName = "x86avxgen"
	specFile = "x86.v0.2.csv" // Default spec filename

	// Paths are relative to GOROOT.
	pathVexOptabs = "src/cmd/internal/obj/x86/vex_optabs.go"
	pathAenum     = "src/cmd/internal/obj/x86/aenum.go"
	pathAnames    = "src/cmd/internal/obj/x86/anames.go"
	pathTests     = "src/cmd/asm/internal/asm/testdata/amd64enc.s"
)

var (
	filenameVexOptabs = filepath.Base(pathVexOptabs)
	filenameAenum     = filepath.Base(pathAenum)
	filenameAnames    = filepath.Base(pathAnames)
	filenameTests     = filepath.Base(pathTests)
)
