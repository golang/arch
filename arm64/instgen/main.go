// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// instgen is used to generate the ARM64 instruction table for encoding and decoding.
// Usage:
//
//	instgen [-i=inputDir] [-o=outputDir] [-url=url] [-version=version]
//
// Users can download the ARM64 XML instruction manual from
// https://developer.arm.com/downloads/-/exploration-tools,
// decompress it, and pass the directory containing the latest XML
// instruction files by "-i" to instgen. Example:
//
//	curl -o foo.gz https://developer.arm.com/-/cdn-downloads/permalink/Exploration-Tools-A64-ISA/ISA_A64/ISA_A64_xml_A_profile-2025-12.tar.gz
//	mkdir foo
//	(cd foo; tar xfz ../foo.gz)
//	./instgen -i foo/ISA_A64_xml_A_profile-2025-12
//
// If the user does not provide "-i" option, the program will try to do the above
// automatically, the temporary directory will be cleaned up after the program exits.
//
// The program parses and processes all the XML files, and generates four .go files:
// inst_gen.go, elem_gen.go, goops_gen.go and arm64ops_gen.go to the output directory.
// The output directory is assumed to be GOROOT, these four files will be generated to
// <outputDir>/src/cmd/internal/obj/arm64/.
//
// If -o option is not specified, no output files will be generated.
//
// Since the format of the ARM64 instruction specification document may update,
// this parser may not work for some versions of the XML document.
// Due to differences in documents between different versions, the generated instruction
// table may also be inconsistent, mainly elements. These differences need to be fixed
// when applying the output to the assembler and disassembler.
//
// The current implementation is based on version:
// https://developer.arm.com/-/cdn-downloads/permalink/Exploration-Tools-A64-ISA/ISA_A64/ISA_A64_xml_A_profile-2025-12.tar.gz
// And after decompressing the tarball, the latest version will be used, i.e. "ISA_A64_xml_A_profile-2025-12".
// Users can provide the url and version of the xml files by "-url" and "-version" options.
// But there is no guarantee that the logic of instgen will work for other versions.
package main

import (
	"flag"
	"log"
	"os"

	"golang.org/x/arch/arm64/instgen/xmlspec"
)

var input = flag.String("i", "", "the input directory of the xml files, this is an optional argument")
var output = flag.String("o", "", "the output directory of the generated files, this is an optional argument")
var genE2E = flag.Bool("e2e", false, "generate end-to-end test data")

var url = flag.String("url", xmlspec.ExpectedURL, "the url of the xml files")
var version = flag.String("version", xmlspec.ExpectedVersion, "the version of the xml files")

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()

	xmlDir := *input
	if *input == "" {
		tmpDir, err := os.MkdirTemp("", "instgen_xml_spec")
		if err != nil {
			// Failed to create a temporary directory, ask the user to provide the data then.
			flag.Usage()
			os.Exit(1)
		}
		defer os.RemoveAll(tmpDir)

		xmlDir, err = xmlspec.GetArm64XMLSpec(tmpDir, *url, *version)
		if err != nil {
			// Get XML data from remote failed, ask the user to provide the data.
			flag.Usage()
			return
		}
	}

	// Parse each xml file to insts.
	insts := xmlspec.ParseXMLFiles(xmlDir)
	xmlspec.ProcessXMLFiles(insts)
	if *output != "" {
		Generate(insts, *output, *genE2E)
	}
	errCnt := 0
	for _, inst := range insts {
		if inst == nil {
			continue
		}
		if inst.ParseError != "" {
			errCnt++
			log.Printf("error: %s", inst.ParseError)
		}
	}
	log.Printf("len(insts) = %v, error count = %v\n", len(insts), errCnt)
}
