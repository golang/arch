// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xmlspec

import (
	"archive/tar"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// The unexported fields are filled by the parser. Example:
// For instruction ADD predicated
// https://developer.arm.com/documentation/111108/2025-12/SVE-Instructions/ADD--vectors--predicated---Add--predicated--?lang=en)
// - parsed data in RegDiagram
// --- Parsed Data ---
// 	fixedBin: 0x4000000
// 	mask: 0xff3fe000
// 	varBin:
// 		size: [22, 24]
// 		Pg: [10, 13]
// 		Zm: [5, 10]
// 		Zdn: [0, 5]
// 	parsed: true
// --- Parsed Data ---
//
// - parsed data in Encoding
// --- Parsed Data ---
// 	binary: 0x4000000
// 	mask: 0xff3fe000
// 	asm: ADD  <Zdn>.<T>, <Pg>/M, <Zdn>.<T>, <Zm>.<T>
// 	goOp: AZADD
// 	arm64Op: A64ADD
// 	class: C_SVE
// 	invalid: false
// 	alias: false
// 	prefix: AZ
// 	operands:
// 		operand{
// 			name: ADD
// 			typ:
// 			elems:
// 		}
// 		operand{
// 			name: <Zm>.<T>
// 			typ: AC_ARNG
// 			elems:
// 				element{
// 					encodedIn: Zm
// 					textExp: Is the name of the second source scalable vector register, encoded in the "Zm" field.
// 					symbol: <Zm>
// 				}
// 				element{
// 					encodedIn: size
// 					textExp: size   <T>
// 					00      B
// 					01      H
// 					10      S
// 					11      D
// 					symbol: <T>
// 				}
// 		}
// 		operand{
// 			name: <Zdn>.<T>
// 			typ: AC_ARNG
// 			elems:
// 				element{
// 					encodedIn: Zdn
// 					textExp: Is the name of the first source and destination scalable vector register, encoded in the "Zdn" field.
// 					symbol: <Zdn>
// 				}
// 				element{
// 					encodedIn: size
// 					textExp: size   <T>
// 					00      B
// 					01      H
// 					10      S
// 					11      D
// 					symbol: <T>
// 				}
// 		}
// 		operand{
// 			name: <Pg>/M
// 			typ: AC_PREGM
// 			elems:
// 				element{
// 					encodedIn: Pg
// 					textExp: Is the name of the governing scalable predicate register P0-P7, encoded in the "Pg" field.
// 					symbol: <Pg>
// 				}
// 		}
// 		operand{
// 			name: <Zdn>.<T>
// 			typ: AC_ARNG
// 			elems:
// 				element{
// 					encodedIn: Zdn
// 					textExp: Is the name of the first source and destination scalable vector register, encoded in the "Zdn" field.
// 					symbol: <Zdn>
// 				}
// 				element{
// 					encodedIn: size
// 					textExp: size   <T>
// 					00      B
// 					01      H
// 					10      S
// 					11      D
// 					symbol: <T>
// 				}
// 		}
// 	parsed: true
// --- Parsed Data ---

// DocVar represents a <docvar> element, storing document metadata like "isa" or "mnemonic".
type DocVar struct {
	Key   string `xml:"key,attr"`
	Value string `xml:"value,attr"`
}

// ArchVariant represents an <arch_variant> element, specifying the required architectural features.
type ArchVariant struct {
	Name    string `xml:"name,attr"`
	Feature string `xml:"feature,attr"`
}

// C represents a <c> element within a register diagram box, specifying bit content or a named field.
type C struct {
	Value   string `xml:",chardata"`
	ColSpan string `xml:"colspan,attr"`
}

// Box represents a <box> element in a register diagram, describing a specific bitfield.
type Box struct {
	HiBit    string `xml:"hibit,attr"`
	Width    string `xml:"width,attr"`
	Name     string `xml:"name,attr"`
	UseName  string `xml:"usename,attr"`
	Settings string `xml:"settings,attr"`
	PsBits   string `xml:"psbits,attr"`
	Cs       []C    `xml:"c"`
}

// bitRange represents a range of bits from lo (inclusive) to hi (exclusive)
type bitRange struct {
	lo, hi int
}

// RegDiagram represents a <regdiagram> element, detailing the bit layout of an encoding.
type RegDiagram struct {
	Boxes []Box `xml:"box"`
	// The fields below are the parsed results of the XML files.
	fixedBin uint32              // instruction encoding binary
	mask     uint32              // instruction decoding mask, it specifies the fixed bit positions of the instruction encoding
	varBin   map[string]bitRange // named bit ranges, key is the name
	parsed   bool                // whether this regdiagram has been parsed
}

// TextA represents a <text> or <a> element within an assembly template.
type TextA struct {
	Value string `xml:",chardata"`
	Link  string `xml:"link,attr"`
	Hover string `xml:"hover,attr"` // contains possible values
}

// AsmTemplate represents an <asmtemplate> element, defining the syntax of the instruction.
type AsmTemplate struct {
	// <Asmtemplate> contains two kinds of sub-elements, <text> and <a>.
	// <text> contains string literals, <a> contains a symbol and
	// two attributes: link and hover. The order of <text> and <a> matters,
	// so we save both into the following structure to preserve their order.
	TextA []TextA `xml:",any"`
}

type element struct {
	encodedIn         string // the name of the binary box this element is encoded in.
	textExp           string // text explanation extracted
	textExpWithRanges string // text explanation with named bit ranges mapping attached.
	symbol            string // asm template
	// Fields below are all parsed metadata for the symbol.
	// Useful for deduplication at instruction matching.
	// When they are default value, they should have no effect on the instruction matching.
	fixedArng        string // if non empty, this element has a fixed arrangement
	fixedLSL         string // if non empty, this element has a fixed LSL
	fixedSXTW        bool   // if true, this element has a fixed SXTW
	fixedUXTW        bool   // if true, this element has a fixed UXTW
	fixedModAmt      string // if non empty, <mod> comes with a fixed <amount>
	fixedScalarWidth int    // if non zero, this element has a fixed scalar width
	hasMod           bool   // if true, this element is a <mod>
	isP              bool   // if true, this element is a scalable predicate register
	isZ              bool   // if true, this element is a scalable vector register
}

type operand struct {
	name        string // asm template
	typ         string
	elems       []element
	constraints []string
}

type class int

const (
	C_NONE class = iota
	C_SVE
	C_SVE2
)

// Encoding represents an <encoding> element for a specific instruction variant.
type Encoding struct {
	Name        string      `xml:"name,attr"`
	Label       string      `xml:"label,attr"`
	DocVars     []DocVar    `xml:"docvars>docvar"`
	Boxes       []Box       `xml:"box"`
	AsmTemplate AsmTemplate `xml:"asmtemplate"`
	// The fields below are the parsed results of the XML files.
	binary   uint32 // more specific instruction encoding than regdiagram.binary
	mask     uint32
	asm      string // asm template
	goOp     string // opcode in Go
	arm64Op  string // arm64 opcode
	operands []operand
	class    class  // instruction class
	invalid  bool   // indicate if this is a valid encoding that need to print
	alias    bool   // whether it is an alias
	prefix   string // prefix to GoOp
	parsed   bool   // whether this encoding has been parsed
}

// Iclass represents an <iclass> element, grouping instruction encodings that share a register diagram.
type Iclass struct {
	Name        string      `xml:"name,attr"`
	OneOf       string      `xml:"oneof,attr"`
	ID          string      `xml:"id,attr"`
	NoEncodings string      `xml:"no_encodings,attr"`
	ISA         string      `xml:"isa,attr"`
	DocVars     []DocVar    `xml:"docvars>docvar"`
	ArchVariant ArchVariant `xml:"arch_variants>arch_variant"`
	RegDiagram  RegDiagram  `xml:"regdiagram"`
	Encodings   []Encoding  `xml:"encoding"`
}

// Classes represents a <classes> element, grouping instruction classes.
type Classes struct {
	Iclass []Iclass `xml:"iclass"`
}

// Symbol represents a <symbol> element, linking an explanation to an assembly sequence symbol.
type Symbol struct {
	Value string `xml:",chardata"`
	Link  string `xml:"link,attr"`
}

// Account represents an <account> element, providing a textual explanation for a symbol.
type Account struct {
	Encodedin string   `xml:"encodedin,attr"`
	DocVars   []DocVar `xml:"docvars>docvar"`
	Intro     string   `xml:"intro>para"`
}

// Entry represents an <entry> element, defining a single cell in a definition table.
type Entry struct {
	Value string `xml:",chardata"`
	Class string `xml:"class,attr"`
}

// Row represents a <row> element within a table header or body.
type Row struct {
	Entries []Entry `xml:"entry"`
}

// THead represents a <thead> element, containing the table header.
type THead struct {
	Row Row `xml:"row"`
}

// TBody represents a <tbody> element, containing the table body.
type TBody struct {
	Row []Row `xml:"row"`
}

// TGroup represents a <tgroup> element, defining the column and row groups of a table.
type TGroup struct {
	Cols  string `xml:"cols,attr"`
	THead THead  `xml:"thead"`
	TBody TBody  `xml:"tbody"`
}

// Table represents a <table> element used to explain symbol encodings.
type Table struct {
	Class  string `xml:"class,attr"`
	TGroup TGroup `xml:"tgroup"`
}

// Definition represents a <definition> element, usually containing a table to define symbol values.
type Definition struct {
	Encodedin string `xml:"encodedin,attr"`
	Intro     string `xml:"intro"`
	Table     Table  `xml:"table"`
}

// Explanation represents an <explanation> element for a symbol used in the assembly template.
type Explanation struct {
	Symbol     Symbol     `xml:"symbol"`
	Account    Account    `xml:"account"`
	Definition Definition `xml:"definition"`
}

// Explanations represents an <explanations> element, grouping symbol explanations.
type Explanations struct {
	Scope        string        `xml:"scope,attr"`
	Explanations []Explanation `xml:"explanation"`
}

// Instruction represents the root <instructionsection> element of an instruction XML specification.
type Instruction struct {
	XMLName      xml.Name     `xml:"instructionsection"`
	Title        string       `xml:"title,attr"`
	Type         string       `xml:"type,attr"`
	DocVars      []DocVar     `xml:"docvars>docvar"`
	Classes      Classes      `xml:"classes"`
	Explanations Explanations `xml:"explanations"`
	ParseError   string
	// The file that this instruction is from, used for error reporting.
	file string
}

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
