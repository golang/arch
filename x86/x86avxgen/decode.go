// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"golang.org/x/arch/x86/xeddata"
)

// encoding is decoded XED instruction pattern.
type encoding struct {
	// opbyte is opcode byte (one that follows [E]VEX prefix).
	// It's called "opcode" in Intel manual, but we use that for
	// instruction name (iclass in XED terms).
	opbyte string

	// opdigit is ModRM.Reg field used to encode opcode extension.
	// In Intel manual, "/digit" notation is used.
	opdigit string

	// vex represents [E]VEX fields that are used in a first [E]VEX
	// opBytes element (see prefixExpr function).
	vex struct {
		P string // 66/F2/F3
		L string // 128/256/512
		M string // 0F/0F38/0F3A
		W string // W0/W1
	}

	// evexScale is a scaling factor used to calculate compact disp-8.
	evexScale string

	// evexBcstScale is like evexScale, but used during broadcasting.
	// Empty for optab entries that do not have broadcasting support.
	evexBcstScale string

	// evex describes which features of EVEX can be used by optab entry.
	// All flags are "false" for VEX-encoded insts.
	evex struct {
		// There is no "broadcast" flag because it's inferred
		// from non-empty evexBcstScale.

		SAE      bool // EVEX.b controls SAE for reg-reg insts
		Rounding bool // EVEX.b + EVEX.RC (VL) control rounding for FP insts
		Zeroing  bool // Instruction can use zeroing.
	}
}

type decoder struct {
	ctx   *context
	insts []*instruction
}

// decodeGroups fills ctx.groups with decoded instruction groups.
//
// Reads XED objects from ctx.xedPath.
func decodeGroups(ctx *context) {
	d := decoder{ctx: ctx}
	groups := make(map[string][]*instruction)
	for _, inst := range d.DecodeAll() {
		groups[inst.opcode] = append(groups[inst.opcode], inst)
	}
	for op, insts := range groups {
		ctx.groups = append(ctx.groups, &instGroup{
			opcode: op,
			list:   insts,
		})
	}
}

// DecodeAll decodes every XED instruction.
func (d *decoder) DecodeAll() []*instruction {
	err := xeddata.WalkInsts(d.ctx.xedPath, func(inst *xeddata.Inst) {
		inst.Pattern = xeddata.ExpandStates(d.ctx.db, inst.Pattern)
		pset := xeddata.NewPatternSet(inst.Pattern)

		opcode := inst.Iclass

		switch {
		case inst.HasAttribute("AMDONLY") || inst.Extension == "XOP":
			return // Only VEX and EVEX are supported
		case !pset.Is("VEX") && !pset.Is("EVEX"):
			return // Skip non-AVX instructions
		case inst.RealOpcode == "N":
			return // Skip unstable instructions
		}

		// Expand some patterns to simplify decodePattern.
		pset.Replace("FIX_ROUND_LEN128()", "VL=0")
		pset.Replace("FIX_ROUND_LEN512()", "VL=2")

		mask, args := d.decodeArgs(pset, inst)
		d.insts = append(d.insts, &instruction{
			pset:   pset,
			opcode: opcode,
			mask:   mask,
			args:   args,
			enc:    d.decodePattern(pset, inst),
		})
	})
	if err != nil {
		log.Fatalf("walk insts: %v", err)
	}
	return d.insts
}

// registerArgs maps XED argument name RHS to its decoded version.
var registerArgs = map[string]argument{
	"GPR32_R()":  {"Yrl", "reg"},
	"GPR64_R()":  {"Yrl", "reg"},
	"VGPR32_R()": {"Yrl", "reg"},
	"VGPR64_R()": {"Yrl", "reg"},
	"VGPR32_N()": {"Yrl", "regV"},
	"VGPR64_N()": {"Yrl", "regV"},
	"GPR32_B()":  {"Yrl", "reg/mem"},
	"GPR64_B()":  {"Yrl", "reg/mem"},
	"VGPR32_B()": {"Yrl", "reg/mem"},
	"VGPR64_B()": {"Yrl", "reg/mem"},

	"XMM_R()":  {"Yxr", "reg"},
	"XMM_R3()": {"YxrEvex", "reg"},
	"XMM_N()":  {"Yxr", "regV"},
	"XMM_N3()": {"YxrEvex", "regV"},
	"XMM_B()":  {"Yxr", "reg/mem"},
	"XMM_B3()": {"YxrEvex", "reg/mem"},
	"XMM_SE()": {"Yxr", "regIH"},

	"YMM_R()":  {"Yyr", "reg"},
	"YMM_R3()": {"YyrEvex", "reg"},
	"YMM_N()":  {"Yyr", "regV"},
	"YMM_N3()": {"YyrEvex", "regV"},
	"YMM_B()":  {"Yyr", "reg/mem"},
	"YMM_B3()": {"YyrEvex", "reg/mem"},
	"YMM_SE()": {"Yyr", "regIH"},

	"ZMM_R3()": {"Yzr", "reg"},
	"ZMM_N3()": {"Yzr", "regV"},
	"ZMM_B3()": {"Yzr", "reg/mem"},

	"MASK_R()": {"Yk", "reg"},
	"MASK_N()": {"Yk", "regV"},
	"MASK_B()": {"Yk", "reg/mem"},

	"MASKNOT0()": {"Yknot0", "kmask"},

	// Handled specifically in "generate".
	"MASK1()": {"MASK1()", "MASK1()"},
}

func (d *decoder) decodeArgs(pset xeddata.PatternSet, inst *xeddata.Inst) (mask *argument, args []*argument) {
	for i, f := range strings.Fields(inst.Operands) {
		xarg, err := xeddata.NewOperand(d.ctx.db, f)
		if err != nil {
			log.Fatalf("%s: args[%d]: %v", inst, i, err)
		}

		switch {
		case xarg.Action == "":
			continue // Skip meta args like EMX_BROADCAST_1TO32_8
		case !xarg.IsVisible():
			continue
		}

		arg := &argument{}
		args = append(args, arg)

		switch xarg.NameLHS() {
		case "IMM0":
			if xarg.Width != "b" {
				log.Fatalf("%s: args[%d]: expected width=b, found %s", inst, i, xarg.Width)
			}
			if pset["IMM0SIGNED=1"] {
				arg.ytype = "Yi8"
			} else {
				arg.ytype = "Yu8"
			}
			arg.zkind = "imm8"

		case "REG0", "REG1", "REG2", "REG3":
			rhs := xarg.NameRHS()
			if rhs == "MASK1()" {
				mask = arg
			}
			*arg = registerArgs[rhs]
			if arg.ytype == "" {
				log.Fatalf("%s: args[%d]: unexpected %s reg", inst, i, rhs)
			}
			if xarg.Attributes["MULTISOURCE4"] {
				arg.ytype += "Multi4"
			}

		case "MEM0":
			arg.ytype = pset.MatchOrDefault("Ym",
				"VMODRM_XMM()", "Yxvm",
				"VMODRM_YMM()", "Yyvm",
				"UISA_VMODRM_XMM()", "YxvmEvex",
				"UISA_VMODRM_YMM()", "YyvmEvex",
				"UISA_VMODRM_ZMM()", "Yzvm",
			)
			arg.zkind = "reg/mem"

		default:
			log.Fatalf("%s: args[%d]: unexpected %s", inst, i, xarg.NameRHS())
		}
	}

	// Reverse args.
	for i := len(args)/2 - 1; i >= 0; i-- {
		j := len(args) - 1 - i
		args[i], args[j] = args[j], args[i]
	}

	return mask, args
}

func (d *decoder) decodePattern(pset xeddata.PatternSet, inst *xeddata.Inst) *encoding {
	var enc encoding

	enc.opdigit = d.findOpdigit(pset)
	enc.opbyte = d.findOpbyte(pset, inst)

	if strings.Contains(inst.Attributes, "DISP8_") {
		enc.evexScale = d.findEVEXScale(pset)
		enc.evexBcstScale = d.findEVEXBcstScale(pset, inst)
	}

	enc.vex.P = pset.Match(
		"VEX_PREFIX=1", "66",
		"VEX_PREFIX=2", "F2",
		"VEX_PREFIX=3", "F3")
	enc.vex.M = pset.Match(
		"MAP=1", "0F",
		"MAP=2", "0F38",
		"MAP=3", "0F3A")
	enc.vex.L = pset.MatchOrDefault("128",
		"VL=0", "128",
		"VL=1", "256",
		"VL=2", "512")
	enc.vex.W = pset.MatchOrDefault("W0",
		"REXW=0", "W0",
		"REXW=1", "W1")

	if pset.Is("EVEX") {
		enc.evex.SAE = strings.Contains(inst.Operands, "TXT=SAESTR")
		enc.evex.Rounding = strings.Contains(inst.Operands, "TXT=ROUNDC")
		enc.evex.Zeroing = strings.Contains(inst.Operands, "TXT=ZEROSTR")
	}

	// Prefix each non-empty part with vex or evex.
	parts := [...]*string{
		&enc.evexScale, &enc.evexBcstScale,
		&enc.vex.P, &enc.vex.M, &enc.vex.L, &enc.vex.W,
	}
	for _, p := range parts {
		if *p == "" {
			continue
		}
		if pset.Is("EVEX") {
			*p = "evex" + *p
		} else {
			*p = "vex" + *p
		}
	}

	return &enc
}

func (d *decoder) findOpdigit(pset xeddata.PatternSet) string {
	reg := pset.Index(
		"REG[0b000]",
		"REG[0b001]",
		"REG[0b010]",
		"REG[0b011]",
		"REG[0b100]",
		"REG[0b101]",
		"REG[0b110]",
		"REG[0b111]",
	)
	// Fixed ModRM.Reg field means that it is used for opcode extension.
	if reg != -1 {
		return fmt.Sprintf("0%d", reg)
	}
	return ""
}

// opbyteRE matches uint8 hex literal.
var opbyteRE = regexp.MustCompile(`0x[0-9A-F]{2}`)

func (d *decoder) findOpbyte(pset xeddata.PatternSet, inst *xeddata.Inst) string {
	opbyte := ""
	for k := range pset {
		if opbyteRE.MatchString(k) {
			if opbyte == "" {
				opbyte = k
			} else {
				log.Fatalf("%s: multiple opbytes", inst)
			}
		}
	}
	return opbyte
}

func (d *decoder) findEVEXScale(pset xeddata.PatternSet) string {
	switch {
	case pset["NELEM_FULL()"], pset["NELEM_FULLMEM()"]:
		return pset.Match(
			"VL=0", "N16",
			"VL=1", "N32",
			"VL=2", "N64")
	case pset["NELEM_MOVDDUP()"]:
		return pset.Match(
			"VL=0", "N8",
			"VL=1", "N32",
			"VL=2", "N64")
	case pset["NELEM_HALF()"], pset["NELEM_HALFMEM()"]:
		return pset.Match(
			"VL=0", "N8",
			"VL=1", "N16",
			"VL=2", "N32")
	case pset["NELEM_QUARTERMEM()"]:
		return pset.Match(
			"VL=0", "N4",
			"VL=1", "N8",
			"VL=2", "N16")
	case pset["NELEM_EIGHTHMEM()"]:
		return pset.Match(
			"VL=0", "N2",
			"VL=1", "N4",
			"VL=2", "N8")
	case pset["NELEM_TUPLE2()"]:
		return pset.Match(
			"ESIZE_32_BITS()", "N8",
			"ESIZE_64_BITS()", "N16")
	case pset["NELEM_TUPLE4()"]:
		return pset.Match(
			"ESIZE_32_BITS()", "N16",
			"ESIZE_64_BITS()", "N32")
	case pset["NELEM_TUPLE8()"]:
		return "N32"
	case pset["NELEM_MEM128()"], pset["NELEM_TUPLE1_4X()"]:
		return "N16"
	}

	// Explicit list is required to make it possible to
	// detect unhandled nonterminals for the caller.
	scalars := [...]string{
		"NELEM_SCALAR()",
		"NELEM_GSCAT()",
		"NELEM_GPR_READER()",
		"NELEM_GPR_READER_BYTE()",
		"NELEM_GPR_READER_WORD()",
		"NELEM_GPR_WRITER_STORE()",
		"NELEM_GPR_WRITER_STORE_BYTE()",
		"NELEM_GPR_WRITER_STORE_WORD()",
		"NELEM_GPR_WRITER_LDOP_D()",
		"NELEM_GPR_WRITER_LDOP_Q()",
		"NELEM_TUPLE1()",
		"NELEM_TUPLE1_BYTE()",
		"NELEM_TUPLE1_WORD()",
	}
	for _, scalar := range scalars {
		if pset[scalar] {
			return pset.Match(
				"ESIZE_8_BITS()", "N1",
				"ESIZE_16_BITS()", "N2",
				"ESIZE_32_BITS()", "N4",
				"ESIZE_64_BITS()", "N8")
		}
	}

	return ""
}

func (d *decoder) findEVEXBcstScale(pset xeddata.PatternSet, inst *xeddata.Inst) string {
	// Only FULL and HALF tuples are affected by the broadcasting.
	switch {
	case pset["NELEM_FULL()"]:
		return pset.Match(
			"ESIZE_32_BITS()", "BcstN4",
			"ESIZE_64_BITS()", "BcstN8")
	case pset["NELEM_HALF()"]:
		return "BcstN4"
	default:
		if inst.HasAttribute("BROADCAST_ENABLED") {
			log.Fatalf("%s: unexpected tuple for bcst", inst)
		}
		return ""
	}
}
