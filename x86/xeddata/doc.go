// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package xeddata provides utilities to work with XED datafiles.
//
// Main features:
//   - Fundamental XED enumerations (CPU modes, operand sizes, ...)
//   - XED objects and their components
//   - XED datafiles reader (see below)
//   - Utility functions like ExpandStates
//
// The amount of file formats that is understood is a minimal
// set required to generate x86.csv from XED tables:
//   - states - simple macro substitutions used in patterns
//   - widths - mappings from width names to their size
//   - element-types - XED xtype information
//   - objects - XED objects that constitute "the tables"
//
// Collectively, those files are called "datafiles".
//
// Terminology is borrowed from XED itself,
// where appropriate, x86csv names are provided
// as an alternative.
//
// Suppose $XED is the path of a checkout of the
// https://github.com/intelxed/xed repo.
//
// "$XED/foo/bar.txt" notation is used to specify a path to "foo/bar.txt"
// file under local XED source repository folder.
//
// The default usage scheme:
//  1. Open "XED database" to load required metadata.
//  2. Read XED file with objects definitions.
//  3. Operate on XED objects.
//
// See example_test.go for complete examples.
// See testdata/xed_objects.txt for examples of "XED objects".
//
// # Obtain XED datafiles
//
// It is required to build Intel XED before attempting to use
// its datafiles, as this package expects the "all" versions that
// are a concatenated final versions of datafiles.
// To build it, follow the instruction on https://github.com/intelxed/xed.
//
// Once built, the "all" versions of data files are in "$XED/obj/dgen/".
// If "$XED/obj/dgen/" does not contain relevant files,
// then either this documentation is stale or your XED is not built.
// Pass $XED/obj/dgen (or a copy of it) as the "xedPath" to [NewDatabase]
// or to x86avxgen -xedPath.
//
// Intel XED https://github.com/intelxed/xed provides all documentation
// that can be required to understand datafiles.
// The "$XED/misc/engineering-notes.txt" is particularly useful.
// For convenience, the most important notes are spread across package comments.
//
// Tested with XED 088c48a2efa447872945168272bcd7005a7ddd91.
package xeddata
