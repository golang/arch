// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"strings"

	"golang.org/x/arch/x86/xeddata"
)

// argument is a describes single instruction operand properties.
type argument struct {
	// ytype is argument class as returned by asm6 "oclass" function.
	ytype string

	// zkind is a partial Z-case matcher.
	// Determines which Z-case handles the encoding of instruction.
	zkind string
}

// instruction is decoded XED instruction.
// Used to produce ytabs and optabs in later phases.
type instruction struct {
	// opcode is instruction symbolic name.
	opcode string

	pset xeddata.PatternSet
	enc  *encoding

	// mask is EVEX K-register argument; points to args element.
	// Used to emit Yk0+Yknot0 table entries.
	// Nil for VEX-encoded insts.
	mask *argument
	args []*argument

	// zform is a pattern that determines which encoder Z-case is used.
	// We store zform instead of zcase directly because it's further
	// expanded during optabs generation.
	zform string
}

// String returns short inst printed representation.
func (inst *instruction) String() string { return inst.opcode }

// YtypeListString joins each argument Y-type and returns the result.
func (inst *instruction) YtypeListString() string {
	var parts []string
	for _, arg := range inst.args {
		parts = append(parts, arg.ytype)
	}
	return strings.Join(parts, " ")
}

// ArgIndexByZkind returns first argument matching given zkind or -1.
func (inst *instruction) ArgIndexByZkind(zkind string) int {
	for i, arg := range inst.args {
		if arg.zkind == zkind {
			return i
		}
	}
	return -1
}
