// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xmlspec

// This file contains the parsed data type.

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

// InstructionParsed is the parsed Instruction, with additional fields for parsing status.
type InstructionParsed struct {
	Instruction
	ParseError string
	// The file that this instruction is from, used for error reporting.
	file string
}

// bitRange represents a range of bits from lo (inclusive) to hi (exclusive)
type bitRange struct {
	lo, hi int
}

// EncodingParsed is the parsed Encoding.
type EncodingParsed struct {
	Encoding
	Binary   uint32    // more specific instruction encoding than regdiagram.fixedBin
	GoOp     string    // opcode in Go
	Operands []Operand // The operands of the instruction
	Asm      string    // asm template
	Alias    bool      // whether it is an alias
	Parsed   bool      // whether this encoding has been parsed
	mask     uint32
	arm64Op  string // arm64 opcode
	class    class  // instruction class
	invalid  bool   // indicate if this is a valid encoding that need to print
	prefix   string // prefix to GoOp
}

// RegDiagramParsed is the parsed RegDiagram.
type RegDiagramParsed struct {
	RegDiagram
	Parsed   bool                // whether this regdiagram has been parsed
	fixedBin uint32              // instruction encoding binary
	mask     uint32              // instruction decoding mask, it specifies the fixed bit positions of the instruction encoding
	varBin   map[string]bitRange // named bit ranges, key is the name
}

// Operand is the parsed operand of the instruction.
type Operand struct {
	Name        string // asm template
	Typ         string // The operand type
	Elems       []Element
	constraints []string
}

// Element represents a parsed element of an operand.
type Element struct {
	TextExpWithRanges string // text explanation with named bit ranges mapping attached.
	encodedIn         string // the name of the binary box this element is encoded in.
	textExp           string // text explanation extracted
	symbol            string // asm template
	// Fields below are useful for deduplication at instruction matching.
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

type class int

const (
	C_NONE class = iota
	C_SVE
	C_SVE2
)
