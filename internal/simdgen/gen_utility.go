// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"text/template"
	"unicode"
)

func templateOf(temp, name string) *template.Template {
	t, err := template.New(name).Parse(temp)
	if err != nil {
		panic(fmt.Errorf("failed to parse template %s: %w", name, err))
	}
	return t
}

func createPath(goroot string, file string) (*os.File, error) {
	fp := filepath.Join(goroot, file)
	dir := filepath.Dir(fp)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	f, err := os.Create(fp)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s: %w", fp, err)
	}
	return f, nil
}

const (
	InvalidIn int = iota
	PureVregIn
	OneKmaskIn
	OneImmIn
	OneKmaskImmIn
	PureKmaskIn
)

const (
	InvalidOut int = iota
	NoOut
	OneVregOut
	OneKmaskOut
	OneVregOutAtIn
)

const (
	InvalidMask int = iota
	NoMask
	OneMask
	OneConstMask
	AllMasks
)

const (
	InvalidImm int = iota
	NoImm
	ConstImm
	VarImm
	ConstVarImm
)

// opShape returns the an int denoting the shape of the operation:
//
//		shapeIn:
//			InvalidIn: unknown, with err set to the error message
//			PureVregIn: pure vreg operation
//			OneKmaskIn: operation with one k mask input (TODO: verify if it's always opmask predicate)
//			OneImmIn: operation with one imm input
//			OneKmaskImmIn: operation with one k mask input and one imm input
//			PureKmaskIn: it's a K mask instruction (which can use K0)
//
//		shapeOut:
//		 	InvalidOut: unknown, with err set to the error message
//			NoOut: no outputs, this is invalid now.
//			OneVregOut: one vreg output
//			OneKmaskOut: one mask output
//			OneVregOutAtIn: one vreg output, it's at the same time the first input
//
//		maskType:
//			InvalidMask: unknown, with err set to the error message
//			NoMask: no mask
//			OneMask: with mask (K1 to K7)
//			OneConstMask: with const mask K0
//			AllMasks: it's a K mask instruction
//
//	 	immType:
//			InvalidImm: unrecognize immediate structure
//			NoImm: no immediate
//			ConstImm: const only immediate
//			VarImm: pure imm argument provided by the users
//			ConstVarImm: a combination of user arg and const
//
// opNoImm is op with its inputs excluding the const imm.
// opNoConstMask is op with its inputs excluding the const mask.
// opNoConstImmMask is op with its inputs excluding the const imm and mask.
//
// This function does not modify op.
func (op *Operation) shape() (shapeIn, shapeOut, maskType, immTyppe int, opNoImm Operation, opNoConstMask Operation, opNoImmConstMask Operation, err error) {
	if len(op.Out) > 1 {
		err = fmt.Errorf("simdgen only supports 1 output: %s", op)
		return
	}
	var outputReg int
	if len(op.Out) == 1 {
		outputReg = op.Out[0].AsmPos
		if op.Out[0].Class == "vreg" {
			shapeOut = OneVregOut
		} else if op.Out[0].Class == "mask" {
			shapeOut = OneKmaskOut
		} else {
			err = fmt.Errorf("simdgen only supports output of class vreg or mask: %s", op)
			return
		}
	} else {
		shapeOut = NoOut
		// TODO: are these only Load/Stores?
		// We manually supported two Load and Store, are those enough?
		err = fmt.Errorf("simdgen only supports 1 output: %s", op)
		return
	}
	hasImm := false
	maskCount := 0
	iConstMask := -1
	hasVreg := false
	for i, in := range op.In {
		if in.AsmPos == outputReg {
			if shapeOut != OneVregOutAtIn && in.AsmPos == 0 && in.Class == "vreg" {
				shapeOut = OneVregOutAtIn
			} else {
				err = fmt.Errorf("simdgen only support output and input sharing the same position case of \"the first input is vreg and the only output\": %s", op)
				return
			}
		}
		if in.Class == "immediate" {
			// A manual check on XED data found that AMD64 SIMD instructions at most
			// have 1 immediates. So we don't need to check this here.
			if *in.Bits != 8 {
				err = fmt.Errorf("simdgen only supports immediates of 8 bits: %s", op)
				return
			}
			hasImm = true
		} else if in.Class == "mask" {
			if in.Const != nil {
				if *in.Const == "K0" {
					if iConstMask != -1 {
						err = fmt.Errorf("simdgen only supports one const mask in inputs: %s", op)
						return
					}
					iConstMask = i
					// Const mask should be invisible in ssa and prog, so we don't treat it as a mask.
					// More specifically in prog, it's optional: when missing the assembler will default it to K0).
					// TODO: verify the above assumption is safe.
				} else {
					err = fmt.Errorf("simdgen only supports const mask K0 in inputs: %s", op)
				}
			} else {
				maskCount++
			}
		} else {
			hasVreg = true
		}
	}
	opNoImm = *op
	opNoConstMask = *op
	opNoImmConstMask = *op
	removeConstMask := func(o *Operation) {
		o.In = append(o.In[:iConstMask], o.In[iConstMask+1:]...)
	}
	if iConstMask != -1 {
		removeConstMask(&opNoConstMask)
		removeConstMask(&opNoImmConstMask)
	}
	removeImm := func(o *Operation) {
		o.In = o.In[1:]
	}
	if hasImm {
		removeImm(&opNoImm)
		removeImm(&opNoImmConstMask)
		if op.In[0].Const != nil {
			if op.In[0].ImmOffset != nil {
				immTyppe = ConstVarImm
			} else {
				immTyppe = ConstImm
			}
		} else if op.In[0].ImmOffset != nil {
			immTyppe = VarImm
		} else {
			err = fmt.Errorf("simdgen requires imm to have at least one of ImmOffset or Const set: %s", op)
			return
		}
	} else {
		immTyppe = NoImm
	}
	if maskCount == 0 {
		if iConstMask == -1 {
			maskType = NoMask
		} else {
			maskType = OneConstMask
		}
	} else {
		maskType = OneMask
	}
	checkPureMask := func() bool {
		if hasImm {
			err = fmt.Errorf("simdgen does not support immediates in pure mask operations: %s", op)
			return true
		}
		if iConstMask != -1 {
			err = fmt.Errorf("simdgen does not support const mask in pure mask operations: %s", op)
			return true
		}
		if hasVreg {
			err = fmt.Errorf("simdgen does not support more than 1 masks in non-pure mask operations: %s", op)
			return true
		}
		return false
	}
	if !hasImm && maskCount == 0 {
		shapeIn = PureVregIn
	} else if !hasImm && maskCount > 0 {
		if maskCount == 1 {
			shapeIn = OneKmaskIn
		} else {
			if checkPureMask() {
				return
			}
			shapeIn = PureKmaskIn
			maskType = AllMasks
		}
	} else if hasImm && maskCount == 0 {
		shapeIn = OneImmIn
	} else {
		if maskCount == 1 {
			shapeIn = OneKmaskImmIn
		} else {
			checkPureMask()
			return
		}
	}
	return
}

// regShape returns a string representation of the register shape.
func (op *Operation) regShape() (string, error) {
	_, _, _, _, _, _, gOp, _ := op.shape()
	var regInfo string
	var vRegInCnt, kMaskInCnt, vRegOutCnt, kMaskOutCnt int
	for _, in := range gOp.In {
		if in.Class == "vreg" {
			vRegInCnt++
		} else if in.Class == "mask" {
			kMaskInCnt++
		}
	}
	for _, out := range gOp.Out {
		// If class overwrite is happening, that's not really a mask but a vreg.
		if out.Class == "vreg" || out.OverwriteClass != nil {
			vRegOutCnt++
		} else if out.Class == "mask" {
			kMaskOutCnt++
		}
	}
	var vRegInS, kMaskInS, vRegOutS, kMaskOutS string
	if vRegInCnt > 0 {
		vRegInS = fmt.Sprintf("fp%d", vRegInCnt)
	}
	if kMaskInCnt > 0 {
		kMaskInS = fmt.Sprintf("k%d", kMaskInCnt)
	}
	if vRegOutCnt > 0 {
		vRegOutS = fmt.Sprintf("fp%d", vRegOutCnt)
	}
	if kMaskOutCnt > 0 {
		kMaskOutS = fmt.Sprintf("k%d", kMaskOutCnt)
	}
	if kMaskInCnt == 0 && kMaskOutCnt == 0 {
		// For pure fp we can abbreviate it as fp%d%d.
		regInfo = fmt.Sprintf("fp%d%d", vRegInCnt, vRegOutCnt)
	} else {
		regInfo = fmt.Sprintf("%s%s%s%s", vRegInS, kMaskInS, vRegOutS, kMaskOutS)
	}
	return regInfo, nil
}

// sortOperand sorts op.In by putting immediates first, then vreg, and mask the last.
// TODO: verify that this is a safe assumption of the prog strcture.
// from my observation looks like in asm, imms are always the first, masks are always the last, with
// vreg in betwee...
func (op *Operation) sortOperand() {
	priority := map[string]int{"immediate": 2, "vreg": 1, "mask": 0}
	sort.SliceStable(op.In, func(i, j int) bool {
		pi := priority[op.In[i].Class]
		pj := priority[op.In[j].Class]
		if pi != pj {
			return pi > pj
		}
		return op.In[i].AsmPos < op.In[j].AsmPos
	})
}

// classifyOp returns a classification string, modified operation, and perhaps error based
// on the stub and intrinsic shape for the operation.
// The classification string is in the regular expression set "op[1234](Imm8)?"
func classifyOp(op Operation) (string, Operation, error) {
	_, shapeOut, _, immType, _, opNoConstMask, gOp, err := op.shape()
	if err != nil {
		return "", op, err
	}
	// Put the go ssa type in GoArch field, simd intrinsics need it.
	if shapeOut == OneVregOut || shapeOut == OneKmaskOut || shapeOut == OneVregOutAtIn {
		opNoConstMask.GoArch = fmt.Sprintf("types.TypeVec%d", *opNoConstMask.Out[0].Bits)
		gOp.GoArch = fmt.Sprintf("types.TypeVec%d", *gOp.Out[0].Bits)
	}
	if immType == VarImm || immType == ConstVarImm {
		switch len(opNoConstMask.In) {
		case 1:
			return "", op, fmt.Errorf("simdgen does not recognize this operation of only immediate input: %s", op)
		case 2:
			return "op1Imm8", opNoConstMask, nil
		case 3:
			return "op2Imm8", opNoConstMask, nil
		case 4:
			return "op3Imm8", opNoConstMask, nil
		case 5:
			return "op4Imm8", opNoConstMask, nil
		default:
			return "", op, fmt.Errorf("simdgen does not recognize this operation of input length %d: %s", len(opNoConstMask.In), op)
		}
	} else {
		switch len(gOp.In) {
		case 1:
			return "op1", gOp, nil
		case 2:
			return "op2", gOp, nil
		case 3:
			return "op3", gOp, nil
		case 4:
			return "op4", gOp, nil
		default:
			return "", op, fmt.Errorf("simdgen does not recognize this operation of input length %d: %s", len(opNoConstMask.In), op)
		}
	}
}

// dedup is deduping operations in the full structure level.
func dedup(ops []Operation) (deduped []Operation) {
	for _, op := range ops {
		seen := false
		for _, dop := range deduped {
			if reflect.DeepEqual(op, dop) {
				seen = true
				break
			}
		}
		if !seen {
			deduped = append(deduped, op)
		}
	}
	return
}

// splitMask splits operations with a single mask vreg input to be masked and unmasked(const: K0).
// It also remove the "Masked" keyword from the name.
func splitMask(ops []Operation) ([]Operation, error) {
	splited := []Operation{}
	for _, op := range ops {
		splited = append(splited, op)
		if op.Masked == nil || *op.Masked != "true" {
			continue
		}
		shapeIn, _, _, _, _, _, _, err := op.shape()
		if err != nil {
			return nil, err
		}
		if shapeIn == OneKmaskIn || shapeIn == OneKmaskImmIn {
			op2 := op
			op2.In = slices.Clone(op.In)
			constMask := "K0"
			// The ops should be sorted when calling this function, the mask is in the end.
			op2.In[len(op2.In)-1].Const = &constMask
			if !strings.HasPrefix(op2.Go, "Masked") {
				return nil, fmt.Errorf("simdgen only recognizes masked operations with name starting with 'Masked': %s", op)
			}
			op2.Go = strings.ReplaceAll(op2.Go, "Masked", "")
			if op2.Documentation != nil {
				*op2.Documentation = strings.ReplaceAll(*op2.Documentation, "Masked", "")
			}
			splited = append(splited, op2)
		} else {
			return nil, fmt.Errorf("simdgen only recognizes masked operations with exactly one mask input: %s", op)
		}
	}
	return splited, nil
}

// dedupGodef is deduping operations in [Op.Go]+[*Op.In[0].Go] level.
// By deduping, it means picking the least advanced architecture that satisfy the requirement:
// AVX512 will be least preferred.
// If FlagNoDedup is set, it will report the duplicates to the console.
func dedupGodef(ops []Operation) ([]Operation, error) {
	seen := map[string][]Operation{}
	for _, op := range ops {
		_, _, _, _, _, _, gOp, err := op.shape()
		if err != nil {
			return nil, err
		}
		genericNames := gOp.Go + *gOp.In[0].Go
		seen[genericNames] = append(seen[genericNames], op)
	}
	if *FlagReportDup {
		for gName, dup := range seen {
			if len(dup) > 1 {
				log.Printf("Duplicate for %s:\n", gName)
				for _, op := range dup {
					log.Printf("%s\n", op)
				}
			}
		}
		return ops, nil
	}
	isAVX512 := func(op Operation) bool {
		return strings.Contains(op.Extension, "AVX512")
	}
	deduped := []Operation{}
	for _, dup := range seen {
		if len(dup) > 1 {
			sort.Slice(dup, func(i, j int) bool {
				// Put non-AVX512 candidates at the beginning
				if !isAVX512(dup[i]) && isAVX512(dup[j]) {
					return true
				}
				// TODO: make the sorting logic finer-grained.
				return false
			})
		}
		deduped = append(deduped, dup[0])
	}
	slices.SortFunc(deduped, compareOperations)
	return deduped, nil
}

// Copy op.ConstImm to op.In[0].Const
// This is a hack to reduce the size of defs we need for const imm operations.
func copyConstImm(ops []Operation) error {
	for _, op := range ops {
		if op.ConstImm == nil {
			continue
		}
		_, _, _, immType, _, _, _, err := op.shape()
		if err != nil {
			return err
		}
		if immType == ConstImm || immType == ConstVarImm {
			op.In[0].Const = op.ConstImm
		}
		// Otherwise, just not port it - e.g. {VPCMP[BWDQ] imm=0} and {VPCMPEQ[BWDQ]} are
		// the same operations "Equal", [dedupgodef] should be able to distinguish them.
	}
	return nil
}

func capitalizeFirst(s string) string {
	if s == "" {
		return ""
	}
	// Convert the string to a slice of runes to handle multi-byte characters correctly.
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// overwrite corrects some errors due to:
//   - The XED data is wrong
//   - Go's SIMD API requirement, for example AVX2 compares should also produce masks.
//     This rewrite has strict constraints, please see the error message.
//     These constraints are also explointed in [writeSIMDRules], [writeSIMDMachineOps]
//     and [writeSIMDSSA], please be careful when updating these constraints.
func overwrite(ops []Operation) error {
	hasClassOverwrite := false
	overwrite := func(op []Operand, idx int) error {
		if op[idx].OverwriteClass != nil {
			if op[idx].OverwriteBase == nil {
				return fmt.Errorf("simdgen: [OverwriteClass] must be set together with [OverwriteBase]: %s", op[idx])
			}
			oBase := *op[idx].OverwriteBase
			oClass := *op[idx].OverwriteClass
			if oClass != "mask" {
				return fmt.Errorf("simdgen: [Class] overwrite only supports overwritting to mask: %s", op[idx])
			}
			if oBase != "int" {
				return fmt.Errorf("simdgen: [Class] overwrite must set [OverwriteBase] to int: %s", op[idx])
			}
			if op[idx].Class != "vreg" {
				return fmt.Errorf("simdgen: [Class] overwrite must be overwriting [Class] from vreg: %s", op[idx])
			}
			hasClassOverwrite = true
			*op[idx].Base = oBase
			op[idx].Class = oClass
			*op[idx].Go = fmt.Sprintf("Mask%dx%d", *op[idx].ElemBits, *op[idx].Lanes)
		} else if op[idx].OverwriteBase != nil {
			oBase := *op[idx].OverwriteBase
			*op[idx].Go = strings.ReplaceAll(*op[idx].Go, capitalizeFirst(*op[idx].Base), capitalizeFirst(oBase))
			*op[idx].Base = oBase
		}
		if op[idx].OverwriteElementBits != nil {
			*op[idx].ElemBits = *op[idx].OverwriteElementBits
			*op[idx].Go = fmt.Sprintf("%s%dx%d", capitalizeFirst(*op[idx].Base), *op[idx].ElemBits, *op[idx].Bits / *op[idx].ElemBits)
		}
		return nil
	}
	for i := range ops {
		hasClassOverwrite = false
		for j := range ops[i].In {
			if err := overwrite(ops[i].In, j); err != nil {
				return err
			}
			if hasClassOverwrite {
				return fmt.Errorf("simdgen does not support [OverwriteClass] in inputs: %s", ops[i])
			}
		}
		for j := range ops[i].Out {
			if err := overwrite(ops[i].Out, j); err != nil {
				return err
			}
		}
		if hasClassOverwrite {
			for _, in := range ops[i].In {
				if in.Class == "mask" {
					return fmt.Errorf("simdgen only supports [OverwriteClass] for operations without mask inputs")
				}
			}
		}
	}
	return nil
}

func (o Operation) String() string {
	var sb strings.Builder
	sb.WriteString("Operation {\n")
	sb.WriteString(fmt.Sprintf("  Go: %s\n", o.Go))
	sb.WriteString(fmt.Sprintf("  GoArch: %s\n", o.GoArch))
	sb.WriteString(fmt.Sprintf("  Asm: %s\n", o.Asm))

	sb.WriteString("  In: [\n")
	for _, op := range o.In {
		sb.WriteString(fmt.Sprintf("    %s,\n", op.String()))
	}
	sb.WriteString("  ]\n")

	sb.WriteString("  Out: [\n")
	for _, op := range o.Out {
		sb.WriteString(fmt.Sprintf("    %s,\n", op.String()))
	}
	sb.WriteString("  ]\n")

	sb.WriteString(fmt.Sprintf("  Commutative: %s\n", o.Commutative))
	sb.WriteString(fmt.Sprintf("  Extension: %s\n", o.Extension))

	if o.Zeroing != nil {
		sb.WriteString(fmt.Sprintf("  Zeroing: %s\n", *o.Zeroing))
	} else {
		sb.WriteString("  Zeroing: <nil>\n")
	}

	if o.Documentation != nil {
		sb.WriteString(fmt.Sprintf("  Documentation: %s\n", *o.Documentation))
	} else {
		sb.WriteString("  Documentation: <nil>\n")
	}

	if o.ConstImm != nil {
		sb.WriteString(fmt.Sprintf("  ConstImm: %s\n", *o.ConstImm))
	} else {
		sb.WriteString("  ConstImm: <nil>\n")
	}

	if o.Masked != nil {
		sb.WriteString(fmt.Sprintf("  Masked: %s\n", *o.Masked))
	} else {
		sb.WriteString("  Masked: <nil>\n")
	}

	sb.WriteString("}\n")
	return sb.String()
}

// String returns a string representation of the Operand.
func (op Operand) String() string {
	var sb strings.Builder
	sb.WriteString("Operand {\n")
	sb.WriteString(fmt.Sprintf("    Class: %s\n", op.Class))

	if op.Go != nil {
		sb.WriteString(fmt.Sprintf("    Go: %s\n", *op.Go))
	} else {
		sb.WriteString("    Go: <nil>\n")
	}

	sb.WriteString(fmt.Sprintf("    AsmPos: %d\n", op.AsmPos))

	if op.Base != nil {
		sb.WriteString(fmt.Sprintf("    Base: %s\n", *op.Base))
	} else {
		sb.WriteString("    Base: <nil>\n")
	}

	if op.ElemBits != nil {
		sb.WriteString(fmt.Sprintf("    ElemBits: %d\n", *op.ElemBits))
	} else {
		sb.WriteString("    ElemBits: <nil>\n")
	}

	if op.Bits != nil {
		sb.WriteString(fmt.Sprintf("    Bits: %d\n", *op.Bits))
	} else {
		sb.WriteString("    Bits: <nil>\n")
	}

	if op.Const != nil {
		sb.WriteString(fmt.Sprintf("    Const: %s\n", *op.Const))
	} else {
		sb.WriteString("    Const: <nil>\n")
	}

	if op.Lanes != nil {
		sb.WriteString(fmt.Sprintf("    Lanes: %d\n", *op.Lanes))
	} else {
		sb.WriteString("    Lanes: <nil>\n")
	}

	if op.OverwriteClass != nil {
		sb.WriteString(fmt.Sprintf("    OverwriteClass: %s\n", *op.OverwriteClass))
	} else {
		sb.WriteString("    OverwriteClass: <nil>\n")
	}

	if op.OverwriteBase != nil {
		sb.WriteString(fmt.Sprintf("    OverwriteBase: %s\n", *op.OverwriteBase))
	} else {
		sb.WriteString("    OverwriteBase: <nil>\n")
	}

	if op.OverwriteElementBits != nil {
		sb.WriteString(fmt.Sprintf("    OverwriteElementBits: %d\n", *op.OverwriteElementBits))
	} else {
		sb.WriteString("    OverwriteElementBits: <nil>\n")
	}

	sb.WriteString("  }\n")
	return sb.String()
}
