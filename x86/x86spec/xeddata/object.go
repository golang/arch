// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xeddata

import (
	"encoding/json"
	"strings"
)

// An Object is a single "dec/enc-instruction" XED object from datafiles.
//
// Field names and their comments are borrowed from Intel XED
// engineering notes (see "$XED/misc/engineering-notes.txt").
//
// Field values are always trimmed (i.e. no leading/trailing whitespace).
//
// Missing optional members are expressed with an empty string.
//
// Object contains multiple Inst elements that represent concrete
// instruction with encoding pattern and operands description.
type Object struct {
	// Iclass is instruction class name (opcode).
	// Iclass alone is not enough to uniquely identify machine instructions.
	// Example: "PSRLW".
	Iclass string

	// Disasm is substituted name when a simple conversion
	// from iclass is inappropriate.
	// Never combined with DisasmIntel or DisasmATTSV.
	// Example: "syscall".
	//
	// Optional.
	Disasm string

	// DisasmIntel is like Disasm, but with Intel syntax.
	// If present, usually comes with DisasmATTSV.
	// Example: "jmp far".
	//
	// Optional.
	DisasmIntel string

	// DisasmATTSV is like Disasm, but with AT&T/SysV syntax.
	// If present, usually comes with DisasmIntel.
	// Example: "ljmp".
	//
	// Optional.
	DisasmATTSV string

	// Attributes describes name set for bits in the binary attributes field.
	// Example: "NOP X87_CONTROL NOTSX".
	//
	// Optional. If not present, zero attribute set is implied.
	Attributes string

	// Uname is unique name used for deleting / replacing instructions.
	//
	// Optional. Provided for completeness, mostly useful for XED internal usage.
	Uname string

	// CPL is instruction current privilege level restriction.
	// Can have value of "0" or "3".
	CPL string

	// Category is an ad-hoc categorization of instructions.
	// Example: "SEMAPHORE".
	Category string

	// Extension is an ad-hoc grouping of instructions.
	// If no ISASet is specified, this is used instead.
	// Example: "3DNOW"
	Extension string

	// Exceptions is an exception set name.
	// Example: "SSE_TYPE_7".
	//
	// Optional. Empty exception category generally means that
	// instruction generates no exceptions.
	Exceptions string

	// ISASet is a name for the group of instructions that
	// introduced this feature.
	// Example: "I286PROTECTED".
	//
	// Older objects only defined Extension field.
	// Newer objects may contain both Extension and ISASet fields.
	// For some objects Extension==ISASet.
	// Both fields are required to do precise CPUID-like decisions.
	//
	// Optional.
	ISASet string

	// Flags describes read/written flag bit values.
	// Example: "MUST [ of-u sf-u af-u pf-u cf-mod ]".
	//
	// Optional. If not present, no flags are neither read nor written.
	Flags string

	// A hopefully useful comment.
	//
	// Optional.
	Comment string

	// The object revision.
	//
	// Optional.
	Version string

	// RealOpcode marks unstable (not in SDM yet) instructions with "N".
	// Normally, always "Y" or not present at all.
	//
	// Optional.
	RealOpcode string

	// Insts are concrete instruction templates that are derived from containing Object.
	// Inst contains fields PATTERN, OPERANDS, IFORM in enc/dec instruction.
	Insts []*Inst
}

// Inst represents a single instruction template.
//
// Some templates contain expandable (macro) pattern and operands
// which tells that there are more than one real instructions
// that are expressed by the template.
type Inst struct {
	// Object that contains properties that are shared with multiple
	// Inst objects.
	*Object

	// Index is the position inside XED object.
	// Object.Insts[Index] returns this inst.
	Index int

	// Pattern is the sequence of bits and nonterminals used to
	// decode/encode an instruction.
	// Example: "0x0F 0x28 no_refining_prefix MOD[0b11] MOD=3 REG[rrr] RM[nnn]".
	Pattern string

	// Operands are instruction arguments, typicall registers,
	// memory operands and pseudo-resources. Separated by space.
	// Example: "MEM0:rcw:b REG0=GPR8_R():r REG1=XED_REG_AL:rcw:SUPP".
	Operands string

	// Iform is a name for the pattern that starts with the
	// iclass and bakes in the operands. If omitted, XED
	// tries to generate one. We often add custom suffixes
	// to these to disambiguate certain combinations.
	// Example: "MOVAPS_XMMps_XMMps_0F28".
	//
	// Optional.
	Iform string
}

// Opcode returns instruction name or empty string,
// if appropriate Object fields are not initialized.
func (o *Object) Opcode() string {
	switch {
	case o.Iclass != "":
		return o.Iclass
	case o.Disasm != "":
		return o.Disasm
	case o.DisasmIntel != "":
		return o.DisasmIntel
	case o.DisasmATTSV != "":
		return o.DisasmATTSV
	case o.Uname != "":
		return o.Uname
	}
	return ""
}

// HasAttribute checks that o has attribute with specified name.
// Note that check is done at "word" level, substring names will not match.
func (o *Object) HasAttribute(name string) bool {
	return containsWord(o.Attributes, name)
}

// String returns pretty-printed inst representation.
//
// Outputs valid JSON string. This property is
// not guaranteed to be preserved.
func (inst *Inst) String() string {
	// Do not use direct inst marshalling to achieve
	// flat object printed representation.
	// Map is avoided to ensure consistent props order.
	type flatObject struct {
		Iclass      string
		Disasm      string `json:",omitempty"`
		DisasmIntel string `json:",omitempty"`
		DisasmATTSV string `json:",omitempty"`
		Attributes  string `json:",omitempty"`
		Uname       string `json:",omitempty"`
		CPL         string
		Category    string
		Extension   string
		Exceptions  string `json:",omitempty"`
		ISASet      string `json:",omitempty"`
		Flags       string `json:",omitempty"`
		Comment     string `json:",omitempty"`
		Version     string `json:",omitempty"`
		RealOpcode  string `json:",omitempty"`
		Pattern     string
		Operands    string
		Iform       string `json:",omitempty"`
	}

	flat := flatObject{
		Iclass:      inst.Iclass,
		Disasm:      inst.Disasm,
		DisasmIntel: inst.DisasmIntel,
		DisasmATTSV: inst.DisasmATTSV,
		Attributes:  inst.Attributes,
		Uname:       inst.Uname,
		CPL:         inst.CPL,
		Category:    inst.Category,
		Extension:   inst.Extension,
		Exceptions:  inst.Exceptions,
		ISASet:      inst.ISASet,
		Flags:       inst.Flags,
		Comment:     inst.Comment,
		Version:     inst.Version,
		RealOpcode:  inst.RealOpcode,
		Pattern:     inst.Pattern,
		Operands:    inst.Operands,
		Iform:       inst.Iform,
	}

	b, err := json.MarshalIndent(flat, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(b)
}

// ExpandStates returns a copy of s where all state macros
// are expanded.
// This requires db "states" to be loaded.
func ExpandStates(db *Database, s string) string {
	substs := db.states
	parts := strings.Fields(s)
	for i := range parts {
		if repl := substs[parts[i]]; repl != "" {
			parts[i] = repl
		}
	}
	return strings.Join(parts, " ")
}

// containsWord searches for whole word match in s.
func containsWord(s, word string) bool {
	i := strings.Index(s, word)
	if i == -1 {
		return false
	}
	leftOK := i == 0 ||
		(s[i-1] == ' ')
	rigthOK := i+len(word) == len(s) ||
		(s[i+len(word)] == ' ')
	return leftOK && rigthOK
}
