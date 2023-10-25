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

// RegDiagram represents a <regdiagram> element, detailing the bit layout of an encoding.
type RegDiagram struct {
	Boxes []Box `xml:"box"`
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

// Encoding represents an <encoding> element for a specific instruction variant.
type Encoding struct {
	Name        string      `xml:"name,attr"`
	Label       string      `xml:"label,attr"`
	DocVars     []DocVar    `xml:"docvars>docvar"`
	Boxes       []Box       `xml:"box"`
	Asmtemplate AsmTemplate `xml:"asmtemplate"`
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
}

func (i Instruction) Print() {
	enc := xml.NewEncoder(os.Stdout)
	enc.Indent("  ", "    ")
	if err := enc.Encode(i); err != nil {
		fmt.Printf("Encode error in print(): %v\n", err)
	}
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
