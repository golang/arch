// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xmlspec

import "encoding/xml"

// This file contains the data types for the XML specification of the ARM64 ISA.

// DocVar represents a <docvar> element, storing document metadata like "isa" or "mnemonic".
type DocVar struct {
	Key   string `xml:"key,attr"`
	Value string `xml:"value,attr"`
}

// C represents a <c> element within a register diagram box, specifying bit content or a named field.
type C struct {
	Value   string `xml:",chardata"`
	ColSpan string `xml:"colspan,attr"`
}

// Box represents a <box> element in a register diagram, describing a specific bitfield.
type Box struct {
	HiBit string `xml:"hibit,attr"`
	Name  string `xml:"name,attr"`
	Cs    []C    `xml:"c"`
}

// TextA represents a <text> or <a> element within an assembly template.
type TextA struct {
	Value string `xml:",chardata"`
	Link  string `xml:"link,attr"`
}

// AsmTemplate represents an <asmtemplate> element, defining the syntax of the instruction.
type AsmTemplate struct {
	// <Asmtemplate> contains two kinds of sub-elements, <text> and <a>.
	// <text> contains string literals, <a> contains a symbol and
	// two attributes: link and hover. The order of <text> and <a> matters,
	// so we save both into the following structure to preserve their order.
	TextA []TextA `xml:",any"`
}

// RegDiagram represents a <regdiagram> element, detailing the bit layout of an encoding.
type RegDiagram struct {
	Boxes  []Box  `xml:"box"`
	PsName string `xml:"psname,attr"`
}

// Encoding represents an <encoding> element for a specific instruction variant.
type Encoding struct {
	Name        string      `xml:"name,attr"`
	DocVars     []DocVar    `xml:"docvars>docvar"`
	Boxes       []Box       `xml:"box"`
	AsmTemplate AsmTemplate `xml:"asmtemplate"`
}

type ArchVariant struct {
	Feature string `xml:"feature,attr"`
}

// Iclass represents an <iclass> element, grouping instruction encodings that share a register diagram.
type Iclass struct {
	Name         string           `xml:"name,attr"`
	DocVars      []DocVar         `xml:"docvars>docvar"`
	ArchVariants []ArchVariant    `xml:"arch_variant"`
	RegDiagram   RegDiagramParsed `xml:"regdiagram"`
	Encodings    []EncodingParsed `xml:"encoding"`
	PsSection    []PsSection      `xml:"ps_section"`
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
	THead THead `xml:"thead"`
	TBody TBody `xml:"tbody"`
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

// Desc represents a <desc> element, containing the description of an instruction.
type Desc struct {
	Brief    Brief    `xml:"brief"`
	Authored Authored `xml:"authored"`
}

// Brief represents a <brief> element, containing a brief description.
type Brief struct {
	Para []Para `xml:"para"`
}

// Authored represents an <authored> element, containing authored paragraphs.
type Authored struct {
	Paragraphs []Para `xml:"para"`
}

// Para represents a <para> element, containing paragraph text.
type Para struct {
	Text string `xml:",innerxml"`
}

// PsSection represents a <ps_section> element, containing pseudocode sections.
type PsSection struct {
	Ps []Ps `xml:"ps"`
}

// Ps represents a <ps> element, containing pseudocode text.
type Ps struct {
	PSText []string `xml:"pstext"` // pseudocode text
}

// Instruction represents the root <instructionsection> element of an instruction XML specification.
type Instruction struct {
	XMLName      xml.Name     `xml:"instructionsection"`
	Title        string       `xml:"title,attr"`
	Desc         Desc         `xml:"desc"`
	Type         string       `xml:"type,attr"`
	DocVars      []DocVar     `xml:"docvars>docvar"`
	Classes      Classes      `xml:"classes"`
	Explanations Explanations `xml:"explanations"`
	PsSections   []PsSection  `xml:"ps_section"`
}
