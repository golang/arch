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

func getData(t *testing.T) []*InstructionParsed {
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
			if !iclass.RegDiagram.Parsed {
				continue
			}
			for _, encoding := range iclass.Encodings {
				if !encoding.Parsed {
					continue outer
				}
			}
			// Check for all encodings, every named box should be encoded
			// by some encoding elements
			encodedBoxes := make(map[string]bool)
			for _, encoding := range iclass.Encodings {
				if encoding.Alias {
					// Alias encodings are not fully specified in their own section,
					// skip them.
					continue
				}
				for _, operand := range encoding.Operands {
					for _, elem := range operand.Elems {
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

// TestEncodingMaskCoversBinary checks that every bit set in an encoding's
// Binary is in its mask, so that word&mask == Binary can hold.
func TestEncodingMaskCoversBinary(t *testing.T) {
	insts := getData(t)
	ProcessXMLFiles(insts)
	for _, inst := range insts {
		if inst == nil {
			continue
		}
		for _, iclass := range inst.Classes.Iclass {
			for _, enc := range iclass.Encodings {
				if !enc.Parsed {
					continue
				}
				if extra := enc.Binary &^ enc.mask; extra != 0 {
					t.Errorf("%s: %s: Binary %#08x has fixed bits %#08x outside mask %#08x", inst.file, enc.GoOp, enc.Binary, extra, enc.mask)
				}
			}
		}
	}
}

// TestSizeFromPseudocode checks the size field that extractBinary fixes, as the
// decode pseudocode says, where the diagram leaves it variable: URECPE and
// URSQRTE by title, the UDIV family by its pseudocode. The mask must hold every
// bit that is fixed, 0 or 1: URECPE needs size = 0b10, so a mask of bit 23 alone
// would also match 0b11.
func TestSizeFromPseudocode(t *testing.T) {
	for _, tt := range []struct {
		title, ps   string
		mask, value uint32 // bits 23:22
	}{
		{"URECPE -- A64", "if size != '10' then EndOfDecode(); end;", 0b11, 0b10},
		{"URSQRTE -- A64", "if size != '10' then EndOfDecode(); end;", 0b11, 0b10},
		{"UDIV -- A64", "if size IN {'0x'} then EndOfDecode(); end;", 0b10, 0b10},
	} {
		inst := &InstructionParsed{}
		inst.Title = tt.title
		inst.DocVars = []DocVar{{Key: "instr-class", Value: "sve"}}
		inst.Classes.Iclass = []Iclass{{
			RegDiagram: RegDiagramParsed{RegDiagram: RegDiagram{Boxes: []Box{
				{HiBit: "23", Name: "size", Cs: []C{{ColSpan: "2"}}},
			}}},
			PsSection: []PsSection{{Ps: []Ps{{PSText: []string{tt.ps}}}}},
		}}
		inst.extractBinary()
		rd := &inst.Classes.Iclass[0].RegDiagram
		if mask, value := rd.mask>>22&3, rd.fixedBin>>22&3; mask != tt.mask || value != tt.value {
			t.Errorf("%s: size has mask %#b and value %#b, want %#b and %#b", tt.title, mask, value, tt.mask, tt.value)
		}
	}
}
