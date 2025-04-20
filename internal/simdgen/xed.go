// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/arch/internal/unify"
	"golang.org/x/arch/x86/xeddata"
	"gopkg.in/yaml.v3"
)

// TODO: Doc. Returns Values with Def domains.
func loadXED(xedPath string) []*unify.Value {
	// TODO: Obviously a bunch more to do here.

	db, err := xeddata.NewDatabase(xedPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	var defs []*unify.Value
	err = xeddata.WalkInsts(xedPath, func(inst *xeddata.Inst) {
		inst.Pattern = xeddata.ExpandStates(db, inst.Pattern)

		switch {
		case inst.RealOpcode == "N":
			return // Skip unstable instructions
		case !(strings.HasPrefix(inst.Extension, "SSE") || strings.HasPrefix(inst.Extension, "AVX")):
			// We're only intested in SSE and AVX instuctions.
			return // Skip non-AVX or SSE instructions
		}

		if *flagDebugXED {
			fmt.Printf("%s:\n%+v\n", inst.Pos, inst)
		}

		ins, outs := decodeOperands(db, strings.Fields(inst.Operands))
		// TODO: "feature"
		fields := []string{"goarch", "asm", "in", "out"}
		values := []*unify.Value{
			unify.NewValue(unify.NewStringExact("amd64")),
			unify.NewValue(unify.NewStringExact(inst.Opcode())),
			unify.NewValue(ins),
			unify.NewValue(outs),
		}
		pos := unify.Pos{Path: inst.Pos.Path, Line: inst.Pos.Line}
		defs = append(defs, unify.NewValuePos(unify.NewDef(fields, values), pos))
		if *flagDebugXED {
			y, _ := yaml.Marshal(defs[len(defs)-1])
			fmt.Printf("==>\n%s\n", y)
		}
	})
	if err != nil {
		log.Fatalf("walk insts: %v", err)
	}
	return defs
}

func decodeOperands(db *xeddata.Database, operands []string) (ins, outs unify.Tuple) {
	var inVals, outVals []*unify.Value
	for asmPos, o := range operands {
		op, err := xeddata.NewOperand(db, o)
		if err != nil {
			log.Fatalf("parsing operand %q: %v", o, err)
		}
		if *flagDebugXED {
			fmt.Printf("  %+v\n", op)
		}

		// TODO: We should have a fixed set of fields once this gets more cleaned up.
		var fields []string
		var values []*unify.Value
		add := func(f string, v *unify.Value) {
			fields = append(fields, f)
			values = append(values, v)
		}

		add("asmPos", unify.NewValue(unify.NewStringExact(fmt.Sprint(asmPos))))

		var r, w bool
		switch op.Action {
		case "r":
			r = true
		case "w":
			w = true
		case "rw":
			r, w = true, true
		default:
			continue
		}

		lhs := op.NameLHS()
		if strings.HasPrefix(lhs, "MEM") {
			add("mem", unify.NewValue(unify.NewStringExact("true")))
			add("w", unify.NewValue(unify.NewStringExact("TODO")))
			add("base", unify.NewValue(unify.NewStringExact("TODO")))
		} else if strings.HasPrefix(lhs, "REG") {
			if op.Width == "mskw" {
				add("mask", unify.NewValue(unify.NewStringExact("true")))
				add("w", unify.NewValue(unify.NewStringExact("TODO")))
				add("base", unify.NewValue(unify.NewStringExact("TODO")))
			} else {
				width, ok := decodeReg(op)
				if !ok {
					return
				}
				baseRe, bits, ok := decodeBits(op)
				if !ok {
					return
				}
				baseDomain, err := unify.NewStringRegex(baseRe)
				if err != nil {
					panic("parsing baseRe: " + err.Error())
				}
				add("bits", unify.NewValue(unify.NewStringExact(fmt.Sprint(bits))))
				add("w", unify.NewValue(unify.NewStringExact(fmt.Sprint(width))))
				add("base", unify.NewValue(baseDomain))
			}
		} else {
			// TODO: Immediates
			add("UNKNOWN", unify.NewValue(unify.NewStringExact(o)))
		}
		// dq => 128 bits (XMM)
		// qq => 256 bits (YMM)
		// mskw => K
		// z[iuf?](8|16|32|...) => 512 bits (ZMM)
		//
		// Are these always XMM/YMM/ZMM or can other irregular things
		// with large widths use these same codes?
		//
		// The only zi* is zi32. I don't understand the difference between
		// zi32 and zu32 or why there are a bunch of zu* but only one zi.
		//
		// The xtype tells you the element type. i8, i16, i32, i64, etc.
		//
		// Things like AVX2 VPAND have an xtype of u256.
		// I think we have to map that to all widths.
		// There's no u512 (presumably those are all masked, so elem width matters).
		// These are all Category: LOGICAL. Maybe we use that info?

		if r {
			inVal := unify.NewValue(unify.NewDef(fields, values))
			inVals = append(inVals, inVal)
		}
		if w {
			outVal := unify.NewValue(unify.NewDef(fields, values))
			outVals = append(outVals, outVal)
		}
	}

	return unify.NewTuple(inVals...), unify.NewTuple(outVals...)
}

func decodeReg(op *xeddata.Operand) (w int, ok bool) {
	if !strings.HasPrefix(op.NameLHS(), "REG") {
		return 0, false
	}
	// TODO: We shouldn't be relying on the macro naming conventions. We should
	// use all-dec-patterns.txt, but xeddata doesn't support that table right now.
	rhs := op.NameRHS()
	if !strings.HasSuffix(rhs, "()") {
		return 0, false
	}
	switch {
	case strings.HasPrefix(rhs, "XMM_"):
		return 128, true
	case strings.HasPrefix(rhs, "YMM_"):
		return 256, true
	case strings.HasPrefix(rhs, "ZMM_"):
		return 512, true
	}
	return 0, false
}

var xtypeRe = regexp.MustCompile(`^([iuf])([0-9]+)$`)

func decodeBits(op *xeddata.Operand) (baseRe string, bits int, ok bool) {
	// Handle some weird ones.
	switch op.Xtype {
	// 8-bit float formats as defined by Open Compute Project "OCP 8-bit
	// Floating Point Specification (OFP8)".
	case "bf8", // E5M2 float
		"hf8": // E4M3 float
		return "", 0, false // TODO
	case "bf16": // bfloat16 float
		return "", 0, false // TODO
	case "2f16":
		// Complex consisting of 2 float16s. Doesn't exist in Go, but we can say
		// what it would be.
		return "complex", 32, true
	case "2i8", "2I8":
		// These just use the lower INT8 in each 16 bit field.
		// As far as I can tell, "2I8" is a typo.
		return "int", 8, true
	}

	// The rest follow a simple pattern.
	m := xtypeRe.FindStringSubmatch(op.Xtype)
	if m == nil {
		// TODO: Report unrecognized xtype
		return "", 0, false
	}
	bits, _ = strconv.Atoi(m[2])
	switch m[1] {
	case "i", "u":
		// XED is rather inconsistent about what's signed, unsigned, or doesn't
		// matter, so merge them together and let the Go definitions narrow as
		// appropriate. Maybe there's a better way to do this.
		baseRe = "int|uint"
	case "f":
		baseRe = "float"
	}
	return baseRe, bits, true
}
