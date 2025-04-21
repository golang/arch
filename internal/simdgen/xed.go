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

		ins, outs, err := decodeOperands(db, strings.Fields(inst.Operands))
		if err != nil {
			log.Printf("%s: [%s] %s", inst.Pos, inst.Opcode(), err)
			return
		}
		// TODO: "feature"
		fields := []string{"goarch", "asm", "in", "out", "extension"}
		values := []*unify.Value{
			unify.NewValue(unify.NewStringExact("amd64")),
			unify.NewValue(unify.NewStringExact(inst.Opcode())),
			unify.NewValue(ins),
			unify.NewValue(outs),
			unify.NewValue(unify.NewStringExact(inst.Extension)),
		}
		if strings.Contains(inst.Pattern, "ZEROING=0") {
			fields = append(fields, "zeroing")
			values = append(values, unify.NewValue(unify.NewStringExact("false")))
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

type operandCommon struct {
	action operandAction
}

// operandAction defines whether this operand is read and/or written.
//
// TODO: Should this live in [xeddata.Operand]?
type operandAction struct {
	r  bool // Read
	w  bool // Written
	cr bool // Read is conditional (implies r==true)
	cw bool // Write is conditional (implies w==true)
}

type operandMem struct {
	operandCommon
	// TODO
}

type vecShape struct {
	elemBits int // Element size in bits
	bits     int // Register width in bits (total vector bits)
}

type operandVReg struct { // Vector register
	operandCommon
	vecShape
	elemBaseType scalarBaseType
}

// operandMask is a vector mask.
//
// Regardless of the actual mask representation, the [vecShape] of this operand
// corresponds to the "bit for bit" type of mask. That is, elemBits gives the
// element width covered by each mask element, and bits/elemBits gives the total
// number of mask elements. (bits gives the total number of bits as if this were
// a bit-for-bit mask, which may be meaningless on its own.)
type operandMask struct {
	operandCommon
	vecShape
	// Bits in the mask is w/bits.
	allMasks bool
}

type operandImm struct {
	operandCommon
	bits int // Immediate size in bits
}

type operand interface {
	common() operandCommon
	toValue() (fields []string, vals []*unify.Value)
}

func strVal(s any) *unify.Value {
	return unify.NewValue(unify.NewStringExact(fmt.Sprint(s)))
}

func (o operandCommon) common() operandCommon {
	return o
}

func (o operandMem) toValue() (fields []string, vals []*unify.Value) {
	// TODO: w, base
	return []string{"class"}, []*unify.Value{strVal("memory")}
}

func (o operandVReg) toValue() (fields []string, vals []*unify.Value) {
	baseDomain, err := unify.NewStringRegex(o.elemBaseType.regex())
	if err != nil {
		panic("parsing baseRe: " + err.Error())
	}
	fields, vals = []string{"class", "bits", "base"}, []*unify.Value{
		strVal("vreg"),
		strVal(o.bits),
		unify.NewValue(baseDomain)}
	if o.elemBits != o.bits {
		fields, vals = append(fields, "elemBits"), append(vals, strVal(o.elemBits))
	}
	// otherwise it means the vector could be any shape.
	return
}

func (o operandMask) toValue() (fields []string, vals []*unify.Value) {
	return []string{"class", "elemBits", "bits"}, []*unify.Value{strVal("mask"), strVal(o.elemBits), strVal(o.bits)}
}

func (o operandMask) zeroMaskValue() (fields []string, vals []*unify.Value) {
	return []string{"class"}, []*unify.Value{strVal("mask")}
}

func (o operandImm) toValue() (fields []string, vals []*unify.Value) {
	return []string{"class", "bits"}, []*unify.Value{strVal("immediate"), strVal(o.bits)}
}

var actionEncoding = map[string]operandAction{
	"r":   {r: true},
	"cr":  {r: true, cr: true},
	"w":   {w: true},
	"cw":  {w: true, cw: true},
	"rw":  {r: true, w: true},
	"crw": {r: true, w: true, cr: true},
	"rcw": {r: true, w: true, cw: true},
}

func decodeOperand(db *xeddata.Database, operand string) (operand, error) {
	op, err := xeddata.NewOperand(db, operand)
	if err != nil {
		log.Fatalf("parsing operand %q: %v", operand, err)
	}
	if *flagDebugXED {
		fmt.Printf("  %+v\n", op)
	}

	// TODO: See xed_decoded_inst_operand_action. This might need to be more
	// complicated.
	action, ok := actionEncoding[op.Action]
	if !ok {
		return nil, fmt.Errorf("unknown action %q", op.Action)
	}
	common := operandCommon{action: action}

	lhs := op.NameLHS()
	if strings.HasPrefix(lhs, "MEM") {
		// TODO: Width, base type
		return operandMem{
			operandCommon: common,
		}, nil
	} else if strings.HasPrefix(lhs, "REG") {
		if op.Width == "mskw" {
			// The mask operand doesn't specify a width. We have to infer it.
			return operandMask{
				operandCommon: common,
			}, nil
		} else {
			regBits, ok := decodeReg(op)
			if !ok {
				return nil, fmt.Errorf("failed to decode register %q", operand)
			}
			baseType, elemBits, ok := decodeType(op)
			if !ok {
				return nil, fmt.Errorf("failed to decode register width %q", operand)
			}
			shape := vecShape{elemBits: elemBits, bits: regBits}
			return operandVReg{
				operandCommon: common,
				vecShape:      shape,
				elemBaseType:  baseType,
			}, nil
		}
	} else if strings.HasPrefix(lhs, "IMM") {
		_, bits, ok := decodeType(op)
		if !ok {
			return nil, fmt.Errorf("failed to decode register width %q", operand)
		}
		return operandImm{
			operandCommon: common,
			bits:          bits,
		}, nil
	}

	// TODO: BASE and SEG
	return nil, fmt.Errorf("unknown operand LHS %q in %q", lhs, operand)
}

func decodeOperands(db *xeddata.Database, operands []string) (ins, outs unify.Tuple, err error) {
	fail := func(err error) (unify.Tuple, unify.Tuple, error) {
		return unify.Tuple{}, unify.Tuple{}, err
	}

	// Decode all of the operands.
	var ops []operand
	for _, o := range operands {
		op, err := decodeOperand(db, o)
		if err != nil {
			return unify.Tuple{}, unify.Tuple{}, err
		}
		ops = append(ops, op)
	}

	// XED doesn't encode the size of mask operands. If there are mask operands,
	// try to infer their sizes from other operands.
	//
	// This is a heuristic and it falls apart in some cases:
	//
	// - Mask operations like KAND[BWDQ] have *nothing* in the XED to indicate
	// mask size.
	//
	// - VINSERT*, VPSLL*, VPSRA*, and VPSRL* and some others naturally have
	// mixed input sizes and the XED doesn't indicate which operands the mask
	// applies to.
	//
	// - VPDP* and VP4DP* have really complex mixed operand patterns.
	//
	// I think for these we may just have to hand-write a table of which
	// operands each mask applies to.
	inferMask := func(r, w bool) error {
		var masks []int
		var rSizes, wSizes, sizes []vecShape
		allMasks := true
		for i, op := range ops {
			action := op.common().action
			if _, ok := op.(operandMask); ok {
				if action.r && action.w {
					return fmt.Errorf("unexpected rw mask")
				}
				if action.r == r || action.w == w {
					masks = append(masks, i)
				}
			} else {
				allMasks = false
				if reg, ok := op.(operandVReg); ok {
					if action.r {
						rSizes = append(rSizes, reg.vecShape)
					}
					if action.w {
						wSizes = append(wSizes, reg.vecShape)
					}
				}
			}
		}
		if len(masks) == 0 {
			return nil
		}

		if r {
			sizes = rSizes
			if len(sizes) == 0 {
				sizes = wSizes
			}
		}
		if w {
			sizes = wSizes
			if len(sizes) == 0 {
				sizes = rSizes
			}
		}

		if len(sizes) == 0 {
			// If all operands are masks, leave the mask inferrence to the users.
			if allMasks {
				for _, i := range masks {
					m := ops[i].(operandMask)
					m.allMasks = true
					ops[i] = m
				}
				return nil
			}
			return fmt.Errorf("cannot infer mask size: no register operands")
		}
		shape, ok := singular(sizes)
		if !ok {
			return fmt.Errorf("cannot infer mask size: multiple register sizes %v", sizes)
		}
		for _, i := range masks {
			m := ops[i].(operandMask)
			m.vecShape = shape
			ops[i] = m
		}
		return nil
	}
	if err := inferMask(true, false); err != nil {
		return fail(err)
	}
	if err := inferMask(false, true); err != nil {
		return fail(err)
	}

	var inVals, outVals []*unify.Value
	for asmPos, op := range ops {
		fields, values := op.toValue()
		if opm, ok := op.(operandMask); ok {
			if opm.allMasks {
				// If all operands are masks, leave the mask inferrence to the users.
				fields, values = opm.zeroMaskValue()
			}
		}

		fields = append(fields, "asmPos")
		values = append(values, unify.NewValue(unify.NewStringExact(fmt.Sprint(asmPos))))

		action := op.common().action
		if action.r {
			inVal := unify.NewValue(unify.NewDef(fields, values))
			inVals = append(inVals, inVal)
		}
		if action.w {
			outVal := unify.NewValue(unify.NewDef(fields, values))
			outVals = append(outVals, outVal)
		}
	}

	return unify.NewTuple(inVals...), unify.NewTuple(outVals...), nil
}

func singular[T comparable](xs []T) (T, bool) {
	if len(xs) == 0 {
		return *new(T), false
	}
	for _, x := range xs[1:] {
		if x != xs[0] {
			return *new(T), false
		}
	}
	return xs[0], true
}

func decodeReg(op *xeddata.Operand) (w int, ok bool) {
	// op.Width tells us the total width, e.g.,:
	//
	//    dq => 128 bits (XMM)
	//    qq => 256 bits (YMM)
	//    mskw => K
	//    z[iuf?](8|16|32|...) => 512 bits (ZMM)
	//
	// But the encoding is really weird and it's not clear if these *always*
	// mean XMM/YMM/ZMM or if other irregular things can use these large widths.
	// Hence, we dig into the register sets themselves.

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

// scalarBaseType describes the base type of a scalar element. This is a Go
// type, but without the bit width suffix (with the exception of
// scalarBaseIntOrUint).
type scalarBaseType int

const (
	scalarBaseInt scalarBaseType = iota
	scalarBaseUint
	scalarBaseIntOrUint // Signed or unsigned is unspecified
	scalarBaseFloat
	scalarBaseComplex
	scalarBaseBFloat
	scalarBaseHFloat
)

func (s scalarBaseType) regex() string {
	switch s {
	case scalarBaseInt:
		return "int"
	case scalarBaseUint:
		return "uint"
	case scalarBaseIntOrUint:
		return "int|uint"
	case scalarBaseFloat:
		return "float"
	case scalarBaseComplex:
		return "complex"
	case scalarBaseBFloat:
		return "BFloat"
	case scalarBaseHFloat:
		return "HFloat"
	}
	panic(fmt.Sprintf("unknown scalar base type %d", s))
}

func decodeType(op *xeddata.Operand) (base scalarBaseType, bits int, ok bool) {
	// The xtype tells you the element type. i8, i16, i32, i64, f32, etc.
	//
	// TODO: Things like AVX2 VPAND have an xtype of u256 because they're
	// element-width agnostic. Do I map that to all widths, or just omit the
	// element width and let unification flesh it out? There's no u512
	// (presumably those are all masked, so elem width matters). These are all
	// Category: LOGICAL, so maybe we could use that info?

	// Handle some weird ones.
	switch op.Xtype {
	// 8-bit float formats as defined by Open Compute Project "OCP 8-bit
	// Floating Point Specification (OFP8)".
	case "bf8": // E5M2 float
		return scalarBaseBFloat, 8, true
	case "hf8": // E4M3 float
		return scalarBaseHFloat, 8, true
	case "bf16": // bfloat16 float
		return scalarBaseBFloat, 16, true
	case "2f16":
		// Complex consisting of 2 float16s. Doesn't exist in Go, but we can say
		// what it would be.
		return scalarBaseComplex, 32, true
	case "2i8", "2I8":
		// These just use the lower INT8 in each 16 bit field.
		// As far as I can tell, "2I8" is a typo.
		return scalarBaseInt, 8, true
	}

	// The rest follow a simple pattern.
	m := xtypeRe.FindStringSubmatch(op.Xtype)
	if m == nil {
		// TODO: Report unrecognized xtype
		return 0, 0, false
	}
	bits, _ = strconv.Atoi(m[2])
	switch m[1] {
	case "i", "u":
		// XED is rather inconsistent about what's signed, unsigned, or doesn't
		// matter, so merge them together and let the Go definitions narrow as
		// appropriate. Maybe there's a better way to do this.
		return scalarBaseIntOrUint, bits, true
	case "f":
		return scalarBaseFloat, bits, true
	default:
		panic("unreachable")
	}
}
