// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xmlspec

import (
	"flag"
	"regexp"
	"strings"
	"testing"
)

var remoteData = flag.Bool("remote", false, "use remote data")

func getData(t *testing.T) []*Instruction {
	if *remoteData {
		xmlDir, err := GetArm64XMLSpec(t.TempDir(), ExpectedURL, ExpectedVersion)
		if err != nil {
			t.Fatalf("GetArm64XMLSpec failed: %v", err)
		}
		return ParseXMLFiles(xmlDir)
	}
	return ParseXMLFiles("testdata")
}

func TestParseXMLFiles(t *testing.T) {
	insts := getData(t)
	// Merely check the parser runs.
	t.Logf("Number of instructions: %d\n", len(insts))
}

func TestProcessXMLFiles(t *testing.T) {
	insts := getData(t)
	ProcessXMLFiles(insts)
	specialInsts := map[string]bool{
		"HISTSEG -- A64":          true,
		"REVB, REVH, REVW -- A64": true,
		"SXTB, SXTH, SXTW -- A64": true,
		"URECPE -- A64":           true,
		"URSQRTE -- A64":          true,
		"UXTB, UXTH, UXTW -- A64": true,
	}
outer:
	for _, inst := range insts {
		if inst == nil {
			continue
		}
		if specialInsts[inst.Title] {
			// These instructions's size is not encoded in the assembler symbols,
			// but specified in the mnemonic and decoding ASL.
			// The parser should have special logic to handle them, skip them for now here.
			continue
		}
		isAlias := false
		for _, doc := range inst.DocVars {
			if doc.Key == "alias_mnemonic" {
				isAlias = true
				break
			}
		}
		if isAlias {
			// Alias instructions are not fully specified in their own XML files,
			// skip them.
			continue
		}
		// Check RegDiagram and Encodings all parsed
		for _, iclass := range inst.Classes.Iclass {
			if !iclass.RegDiagram.parsed {
				continue
			}
			for _, encoding := range iclass.Encodings {
				if !encoding.parsed {
					continue outer
				}
			}
			// Check for all encodings, every named box should be encoded
			// by some encoding elements
			encodedBoxes := make(map[string]bool)
			for _, encoding := range iclass.Encodings {
				if encoding.alias {
					// Alias encodings are not fully specified in their own section,
					// skip them.
					continue
				}
				for _, operand := range encoding.operands {
					for _, elem := range operand.elems {
						encodedIn := elem.encodedIn
						encodedBoxes[encodedIn] = true
						if strings.Contains(encodedIn, ":") {
							// Some weird instructions like smlalb_z_zzzi
							// encodes an immediate in 2 or 3 disjoin fields
							// appended together. We need to record all of them.
							// They are represented as "(name1 :: name2)" or
							// "(name1 :: name2 :: name3)".
							// We need to extract these names.
							re := regexp.MustCompile(`\((.*?) :: (.*?)( :: (.*?))?\)`)
							matches := re.FindStringSubmatch(encodedIn)
							if len(matches) == 3 {
								encodedBoxes[matches[1]] = true
								encodedBoxes[matches[2]] = true
							} else if len(matches) == 5 {
								encodedBoxes[matches[1]] = true
								encodedBoxes[matches[2]] = true
								encodedBoxes[matches[4]] = true
							}
						}
						if strings.Contains(encodedIn, "[") {
							// Some weird instructions like histcnt_z_p_zz
							// encodes only part of a field:
							// size[0] (meanwhile size is of size 2).
							// We need to record the full field name.
							re := regexp.MustCompile(`(.*)\[.*?\]`)
							matches := re.FindStringSubmatch(encodedIn)
							if len(matches) == 2 {
								encodedBoxes[matches[1]] = true
							}
						}
					}
				}
			}
			for name := range iclass.RegDiagram.varBin {
				if !encodedBoxes[name] {
					t.Errorf("Box %s not encoded in %s", name, inst.file)
				}
			}
		}
	}
}
