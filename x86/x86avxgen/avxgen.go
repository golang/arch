// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"strings"

	"golang.org/x/arch/x86/x86csv"
)

// An encoding is the parsed x86csv.Inst Encoding.
type encoding struct {
	vex     string // Dot separated VEX prefix. e.g. "VEX.NDD.256.66.0F.WIG"
	opbyte  string // Single opcode encoding byte (example: "38")
	opdigit string // "/digit" byte that extends the opcode (example: "7" for /7)
}

// parseEncoding parses x86csv.Inst Encoding.
func parseEncoding(encString string) encoding {
	f := strings.Fields(encString)
	enc := encoding{
		vex:    f[0],
		opbyte: f[1],
	}

	// Parse rest parts.
	// Currently interested only in "/digit" byte,
	// but that may change later.
	for _, p := range f[2:] {
		switch p {
		case "/r", "/is4":
			// Currently not handled.

		case "/0", "/1", "/2", "/3", "/4", "/5", "/6", "/7":
			enc.opdigit = p[len("/"):]
		}
	}

	return enc
}

// ytabID is a name of "x86/asm6.go" ytab table object.
//
// ytabMap contains all IDs that can be referenced
// from generated Optabs.
type ytabID string

// optab holds data that is required to emit x86 optab entry.
//
// That is, it is not "the optab" itself, but a set
// of parameters required to expand a template.
//
// Terminology differences:
// x86csv   | asm6.go
// ------------------
// opcode   | as
// encoding | op
// ------------------
// We use asm6.go terminology only in description of this structure,
// as it describes asm6.go object.
type optab struct {
	// Prefix is fixed to "Pvex" right now.
	// This may change when EVEX-encoded instructions
	// generation is supported.

	as     string   // AXXX constant name without leading "A" (example: ADD for AADD)
	ytabID ytabID   // ytab table name (example: yvex_y2)
	op     []string // Encoding parts
}

// doGroups groups instructions in insts by Go name and then calls
// f for each different name, passing the name and the instructions
// using that name. The calls are made ordered by first appearance
// of name in insts, and the list of instructions for a given name
// are in the same order as in insts.
func doGroups(insts []*x86csv.Inst, f func(string, []*x86csv.Inst)) {
	var opcodes []string
	groups := make(map[string][]*x86csv.Inst)
	for _, inst := range insts {
		op := inst.GoOpcode()
		if groups[op] == nil {
			opcodes = append(opcodes, op)
		}
		groups[op] = append(groups[op], inst)
	}
	for _, op := range opcodes {
		f(op, groups[op])
	}
}

// argsNormalizer is used to transform Intel manual style args (operands)
// to shorter form. Compact form is used in compound keys (see ytabMap).
//
// asm6.go (x86 asm backend) does not care about:
// - memory operand sizes. There are distinct instructions for different sizes.
// - register indexes. "xmm1" or "xmm" - does not matter.
var argsNormalizer = strings.NewReplacer(
	", ", ",",
	" ", "",

	"imm8", "i8",

	"m8", "m",
	"m16", "m",
	"m32", "m",
	"m64", "m",
	"m128", "m",
	"m256", "m",

	"r32", "r",
	"r64", "r",

	"xmm1", "x",
	"xmm2", "x",
	"xmm3", "x",
	"xmm", "x",

	"ymm1", "y",
	"ymm2", "y",
	"ymm3", "y",
	"ymm", "y",
)

// ytabKey computes a key describing the operand forms from insts for ytabMap.
// This lets us find instructions with the same groups of forms and
// have them share a ytab entry.
func ytabKey(op string, insts []*x86csv.Inst) string {
	var all []string
	for _, inst := range insts {
		form := argsNormalizer.Replace(inst.Go[len(op):])
		all = append(all, form)
	}
	return strings.Join(all, ";")
}

// vexExpr returns the Go expression describing the VEX prefix.
//
// Examples:
//   "VEX.NDS.256.0F.WIG" => "vexNDS|vex256|vex0F|vexWIG"
//   "VEX.256.0F.WIG"     => "vexNOVSR|vex256|vex0F|vexWIG"
func vexExpr(vex string) string {
	expr := strings.Replace(vex, ".", "|vex", -1)[len("VEX|"):]
	for _, p := range [...]string{"vexNDS", "vexNDD", "vexDDS"} {
		if strings.HasPrefix(expr, p) {
			return expr
		}
	}
	return "vexNOVSR|" + expr
}
