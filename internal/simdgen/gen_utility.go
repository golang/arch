// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/format"
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

func formatWriteAndClose(out *bytes.Buffer, goroot string, file string) {
	b, err := format.Source(out.Bytes())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		fmt.Fprintf(os.Stderr, "%s\n", numberLines(out.Bytes()))
		fmt.Fprintf(os.Stderr, "%v\n", err)
		panic(err)
	} else {
		writeAndClose(b, goroot, file)
	}
}

func writeAndClose(b []byte, goroot string, file string) {
	ofile, err := createPath(goroot, file)
	if err != nil {
		panic(err)
	}
	ofile.Write(b)
	ofile.Close()
}

// numberLines takes a slice of bytes, and returns a string where each line
// is numbered, starting from 1.
func numberLines(data []byte) string {
	var buf bytes.Buffer
	r := bytes.NewReader(data)
	s := bufio.NewScanner(r)
	for i := 1; s.Scan(); i++ {
		fmt.Fprintf(&buf, "%d: %s\n", i, s.Text())
	}
	return buf.String()
}

type inShape uint8
type outShape uint8
type maskShape uint8
type immShape uint8

const (
	InvalidIn     inShape = iota
	PureVregIn            // vector register input only
	OneKmaskIn            // vector and kmask input
	OneImmIn              // vector and immediate input
	OneKmaskImmIn         // vector, kmask, and immediate inputs
	PureKmaskIn           // only mask inputs.
)

const (
	InvalidOut     outShape = iota
	NoOut                   // no output
	OneVregOut              // (one) vector register output
	OneGregOut              // (one) general register output
	OneKmaskOut             // mask output
	OneVregOutAtIn          // the first input is also the output
)

const (
	InvalidMask maskShape = iota
	NoMask                // no mask
	OneMask               // with mask (K1 to K7)
	AllMasks              // a K mask instruction (K0-K7)
)

const (
	InvalidImm  immShape = iota
	NoImm                // no immediate
	ConstImm             // const only immediate
	VarImm               // pure imm argument provided by the users
	ConstVarImm          // a combination of user arg and const
)

// opShape returns the several integers describing the shape of the operation,
// and modified versions of the op:
//
// opNoImm is op with its inputs excluding the const imm.
// opNoConstImmMask is op with its inputs excluding the const imm and mask.
//
// This function does not modify op.
func (op *Operation) shape() (shapeIn inShape, shapeOut outShape, maskType maskShape, immType immShape,
	opNoImm Operation, opNoImmConstMask Operation) {
	if len(op.Out) > 1 {
		panic(fmt.Errorf("simdgen only supports 1 output: %s", op))
	}
	var outputReg int
	if len(op.Out) == 1 {
		outputReg = op.Out[0].AsmPos
		if op.Out[0].Class == "vreg" {
			shapeOut = OneVregOut
		} else if op.Out[0].Class == "greg" {
			shapeOut = OneGregOut
		} else if op.Out[0].Class == "mask" {
			shapeOut = OneKmaskOut
		} else {
			panic(fmt.Errorf("simdgen only supports output of class vreg or mask: %s", op))
		}
	} else {
		shapeOut = NoOut
		// TODO: are these only Load/Stores?
		// We manually supported two Load and Store, are those enough?
		panic(fmt.Errorf("simdgen only supports 1 output: %s", op))
	}
	hasImm := false
	maskCount := 0
	hasVreg := false
	for _, in := range op.In {
		if in.AsmPos == outputReg {
			if shapeOut != OneVregOutAtIn && in.AsmPos == 0 && in.Class == "vreg" {
				shapeOut = OneVregOutAtIn
			} else {
				panic(fmt.Errorf("simdgen only support output and input sharing the same position case of \"the first input is vreg and the only output\": %s", op))
			}
		}
		if in.Class == "immediate" {
			// A manual check on XED data found that AMD64 SIMD instructions at most
			// have 1 immediates. So we don't need to check this here.
			if *in.Bits != 8 {
				panic(fmt.Errorf("simdgen only supports immediates of 8 bits: %s", op))
			}
			hasImm = true
		} else if in.Class == "mask" {
			maskCount++
		} else {
			hasVreg = true
		}
	}
	opNoImm = *op
	opNoImmConstMask = *op

	removeImm := func(o *Operation) {
		o.In = o.In[1:]
	}
	if hasImm {
		removeImm(&opNoImm)
		removeImm(&opNoImmConstMask)
		if op.In[0].Const != nil {
			if op.In[0].ImmOffset != nil {
				immType = ConstVarImm
			} else {
				immType = ConstImm
			}
		} else if op.In[0].ImmOffset != nil {
			immType = VarImm
		} else {
			panic(fmt.Errorf("simdgen requires imm to have at least one of ImmOffset or Const set: %s", op))
		}
	} else {
		immType = NoImm
	}
	if maskCount == 0 {
		maskType = NoMask
	} else {
		maskType = OneMask
	}
	checkPureMask := func() bool {
		if hasImm {
			panic(fmt.Errorf("simdgen does not support immediates in pure mask operations: %s", op))
		}
		if hasVreg {
			panic(fmt.Errorf("simdgen does not support more than 1 masks in non-pure mask operations: %s", op))
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
	_, _, _, _, _, gOp := op.shape()
	var regInfo string
	var vRegInCnt, gRegInCnt, kMaskInCnt, vRegOutCnt, gRegOutCnt, kMaskOutCnt int
	for _, in := range gOp.In {
		if in.Class == "vreg" {
			vRegInCnt++
		} else if in.Class == "greg" {
			gRegInCnt++
		} else if in.Class == "mask" {
			kMaskInCnt++
		}
	}
	for _, out := range gOp.Out {
		// If class overwrite is happening, that's not really a mask but a vreg.
		if out.Class == "vreg" || out.OverwriteClass != nil {
			vRegOutCnt++
		} else if out.Class == "greg" {
			gRegOutCnt++
		} else if out.Class == "mask" {
			kMaskOutCnt++
		}
	}
	var inRegs, inMasks, outRegs, outMasks string

	rmAbbrev := func(s string, i int) string {
		if i == 0 {
			return ""
		}
		if i == 1 {
			return s
		}
		return fmt.Sprintf("%s%d", s, i)

	}

	inRegs = rmAbbrev("v", vRegInCnt)
	inRegs += rmAbbrev("gp", gRegInCnt)
	inMasks = rmAbbrev("k", kMaskInCnt)

	outRegs = rmAbbrev("v", vRegOutCnt)
	outRegs += rmAbbrev("gp", gRegOutCnt)
	outMasks = rmAbbrev("k", kMaskOutCnt)

	if kMaskInCnt == 0 && kMaskOutCnt == 0 && gRegInCnt == 0 && gRegOutCnt == 0 {
		// For pure v we can abbreviate it as v%d%d.
		regInfo = fmt.Sprintf("v%d%d", vRegInCnt, vRegOutCnt)
	} else if kMaskInCnt == 0 && kMaskOutCnt == 0 {
		regInfo = fmt.Sprintf("%s%s", inRegs, outRegs)
	} else {
		regInfo = fmt.Sprintf("%s%s%s%s", inRegs, inMasks, outRegs, outMasks)
	}
	return regInfo, nil
}

// sortOperand sorts op.In by putting immediates first, then vreg, and mask the last.
// TODO: verify that this is a safe assumption of the prog structure.
// from my observation looks like in asm, imms are always the first,
// masks are always the last, with vreg in between.
func (op *Operation) sortOperand() {
	priority := map[string]int{"immediate": 0, "vreg": 1, "greg": 1, "mask": 2}
	sort.SliceStable(op.In, func(i, j int) bool {
		pi := priority[op.In[i].Class]
		pj := priority[op.In[j].Class]
		if pi != pj {
			return pi < pj
		}
		return op.In[i].AsmPos < op.In[j].AsmPos
	})
}

// goNormalType returns the Go type name for the result of an Op that
// does not return a vector, i.e., that returns a result in a general
// register.  Currently there's only one family of Ops in Go's simd library
// that does this (GetElem), and so this is specialized to work for that,
// but the problem (mismatch betwen hardware register width and Go type
// width) seems likely to recur if there are any other cases.
func (op Operation) goNormalType() string {
	if op.Go == "GetElem" {
		// GetElem returns an element of the vector into a general register
		// but as far as the hardware is concerned, that result is either 32
		// or 64 bits wide, no matter what the vector element width is.
		// This is not "wrong" but it is not the right answer for Go source code.
		// To get the Go type right, combine the base type ("int", "uint", "float"),
		// with the input vector element width in bits (8,16,32,64).

		at := 0 // proper value of at depends on whether immediate was stripped or not
		if op.In[at].Class == "immediate" {
			at++
		}
		return fmt.Sprintf("%s%d", *op.Out[0].Base, *op.In[at].ElemBits)
	}
	panic(fmt.Errorf("Implement goNormalType for %v", op))
}

// SSAType returns the string for the type reference in SSA generation,
// for example in the intrinsics generating template.
func (op Operation) SSAType() string {
	if op.Out[0].Class == "greg" {
		return fmt.Sprintf("types.Types[types.T%s]", strings.ToUpper(op.goNormalType()))
	}
	return fmt.Sprintf("types.TypeVec%d", *op.Out[0].Bits)
}

// GoType returns the Go type returned by this operation (relative to the simd package),
// for example "int32" or "Int8x16".  This is used in a template.
func (op Operation) GoType() string {
	if op.Out[0].Class == "greg" {
		return op.goNormalType()
	}
	return *op.Out[0].Go
}

// ImmName returns the name to use for an operation's immediate operand.
// This can be overriden in the yaml with "name" on an operand,
// otherwise, for now, it is "imm" but
// TODO come up with a better default immediate parameter name.
func (op Operation) ImmName() string {
	return op.Op0Name("imm")
}

func (o Operand) OpName(s string) string {
	if n := o.Name; n != nil {
		return *n
	}
	return s
}

func (o Operand) OpNameAndType(s string) string {
	return o.OpName(s) + " " + *o.Go
}

// Op0Name returns the name to use for the 0 operand,
// if any is present, otherwise the parameter is used.
func (op Operation) Op0Name(s string) string {
	return op.In[0].OpName(s)
}

// Op1Name returns the name to use for the 1 operand,
// if any is present, otherwise the parameter is used.
func (op Operation) Op1Name(s string) string {
	return op.In[1].OpName(s)
}

// Op2Name returns the name to use for the 2 operand,
// if any is present, otherwise the parameter is used.
func (op Operation) Op2Name(s string) string {
	return op.In[2].OpName(s)
}

// Op3Name returns the name to use for the 3 operand,
// if any is present, otherwise the parameter is used.
func (op Operation) Op3Name(s string) string {
	return op.In[3].OpName(s)
}

// Op0NameAndType returns the name and type to use for
// the 0 operand, if a name is provided, otherwise
// the parameter value is used as the default.
func (op Operation) Op0NameAndType(s string) string {
	return op.In[0].OpNameAndType(s)
}

// Op1NameAndType returns the name and type to use for
// the 1 operand, if a name is provided, otherwise
// the parameter value is used as the default.
func (op Operation) Op1NameAndType(s string) string {
	return op.In[1].OpNameAndType(s)
}

// Op2NameAndType returns the name and type to use for
// the 2 operand, if a name is provided, otherwise
// the parameter value is used as the default.
func (op Operation) Op2NameAndType(s string) string {
	return op.In[2].OpNameAndType(s)
}

// Op3NameAndType returns the name and type to use for
// the 3 operand, if a name is provided, otherwise
// the parameter value is used as the default.
func (op Operation) Op3NameAndType(s string) string {
	return op.In[3].OpNameAndType(s)
}

// Op4NameAndType returns the name and type to use for
// the 4 operand, if a name is provided, otherwise
// the parameter value is used as the default.
func (op Operation) Op4NameAndType(s string) string {
	return op.In[4].OpNameAndType(s)
}

var immClasses []string = []string{"BAD0Imm", "BAD1Imm", "op1Imm8", "op2Imm8", "op3Imm8", "op4Imm8"}
var classes []string = []string{"BAD0", "op1", "op2", "op3", "op4"}

// classifyOp returns a classification string, modified operation, and perhaps error based
// on the stub and intrinsic shape for the operation.
// The classification string is in the regular expression set "op[1234](Imm8)?(_<order>)?"
// where the "<order>" suffix is optionally attached to the Operation in its input yaml.
// The classification string is used to select a template or a clause of a template
// for intrinsics declaration and the ssagen intrinisics glue code in the compiler.
func classifyOp(op Operation) (string, Operation, error) {
	_, _, _, immType, _, gOp := op.shape()

	var class string

	if immType == VarImm || immType == ConstVarImm {
		switch l := len(op.In); l {
		case 1:
			return "", op, fmt.Errorf("simdgen does not recognize this operation of only immediate input: %s", op)
		case 2, 3, 4, 5:
			class = immClasses[l]
		default:
			return "", op, fmt.Errorf("simdgen does not recognize this operation of input length %d: %s", len(op.In), op)
		}
		if order := op.OperandOrder; order != nil {
			class += "_" + *order
		}
		return class, op, nil
	} else {
		switch l := len(gOp.In); l {
		case 1, 2, 3, 4:
			class = classes[l]
		default:
			return "", op, fmt.Errorf("simdgen does not recognize this operation of input length %d: %s", len(op.In), op)
		}
		if order := op.OperandOrder; order != nil {
			class += "_" + *order
		}
		return class, gOp, nil
	}
}

func checkVecAsScalar(op Operation) (idx int, err error) {
	idx = -1
	sSize := 0
	for i, o := range op.In {
		if o.TreatLikeAScalarOfSize != nil {
			if idx == -1 {
				idx = i
				sSize = *o.TreatLikeAScalarOfSize
			} else {
				err = fmt.Errorf("simdgen only supports one TreatLikeAScalarOfSize in the arg list: %s", op)
				return
			}
		}
	}
	if idx >= 0 {
		if idx != 1 {
			err = fmt.Errorf("simdgen only supports TreatLikeAScalarOfSize at the 2nd arg of the arg list: %s", op)
			return
		}
		if sSize != 8 && sSize != 16 && sSize != 32 && sSize != 64 {
			err = fmt.Errorf("simdgen does not recognize this uint size: %d, %s", sSize, op)
			return
		}
	}
	return
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
		shapeIn, _, _, _, _, _ := op.shape()

		if shapeIn == OneKmaskIn || shapeIn == OneKmaskImmIn {
			op2 := op
			// The ops should be sorted when calling this function, the mask is in the end, drop the mask
			op2.In = slices.Clone(op.In)[:len(op.In)-1]
			if !strings.HasSuffix(op2.Go, "Masked") {
				return nil, fmt.Errorf("simdgen only recognizes masked operations with name ending with 'Masked': %s", op)
			}
			maskedOpName := op2.Go
			op2.Go = strings.TrimSuffix(op2.Go, "Masked")
			if op2.Documentation != nil {
				*op2.Documentation = strings.ReplaceAll(*op2.Documentation, maskedOpName, op2.Go)
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
		_, _, _, _, _, gOp := op.shape()

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
		_, _, _, immType, _, _ := op.shape()

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
	overwrite := func(op []Operand, idx int, o Operation) error {
		if op[idx].OverwriteClass != nil {
			if op[idx].OverwriteBase == nil {
				panic(fmt.Errorf("simdgen: [OverwriteClass] must be set together with [OverwriteBase]: %s", op[idx]))
			}
			oBase := *op[idx].OverwriteBase
			oClass := *op[idx].OverwriteClass
			if oClass != "mask" {
				panic(fmt.Errorf("simdgen: [Class] overwrite only supports overwritting to mask: %s", op[idx]))
			}
			if oBase != "int" {
				panic(fmt.Errorf("simdgen: [Class] overwrite must set [OverwriteBase] to int: %s", op[idx]))
			}
			if op[idx].Class != "vreg" {
				panic(fmt.Errorf("simdgen: [Class] overwrite must be overwriting [Class] from vreg: %s", op[idx]))
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
			if op[idx].ElemBits == nil {
				panic(fmt.Errorf("ElemBits is nil at operand %d of %v", idx, o))
			}
			*op[idx].ElemBits = *op[idx].OverwriteElementBits
			*op[idx].Lanes = *op[idx].Bits / *op[idx].ElemBits
			*op[idx].Go = fmt.Sprintf("%s%dx%d", capitalizeFirst(*op[idx].Base), *op[idx].ElemBits, *op[idx].Lanes)
		}
		return nil
	}
	for i, o := range ops {
		hasClassOverwrite = false
		for j := range ops[i].In {
			if err := overwrite(ops[i].In, j, o); err != nil {
				return err
			}
			if hasClassOverwrite {
				return fmt.Errorf("simdgen does not support [OverwriteClass] in inputs: %s", ops[i])
			}
		}
		for j := range ops[i].Out {
			if err := overwrite(ops[i].Out, j, o); err != nil {
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

// reportXEDInconsistency reports potential XED inconsistencies.
// We can add more fields to [Operation] to enable more checks and implement it here.
// Supported checks:
// [NameAndSizeCheck]: NAME[BWDQ] should set the elemBits accordingly.
// This check is useful to find inconsistencies, then we can add overwrite fields to
// those defs to correct them manually.
func reportXEDInconsistency(ops []Operation) error {
	for _, o := range ops {
		if o.NameAndSizeCheck != nil {
			suffixSizeMap := map[byte]int{'B': 8, 'W': 16, 'D': 32, 'Q': 64}
			checkOperand := func(opr Operand) error {
				if opr.ElemBits == nil {
					return fmt.Errorf("simdgen expects elemBits to be set when performing NameAndSizeCheck")
				}
				if v, ok := suffixSizeMap[o.Asm[len(o.Asm)-1]]; !ok {
					return fmt.Errorf("simdgen expects asm to end with [BWDQ] when performing NameAndSizeCheck")
				} else {
					if v != *opr.ElemBits {
						return fmt.Errorf("simdgen finds NameAndSizeCheck inconsistency in def: %s", o)
					}
				}
				return nil
			}
			for _, in := range o.In {
				if in.Class != "vreg" && in.Class != "mask" {
					continue
				}
				if in.TreatLikeAScalarOfSize != nil {
					// This is an irregular operand, don't check it.
					continue
				}
				if err := checkOperand(in); err != nil {
					return err
				}
			}
			for _, out := range o.Out {
				if err := checkOperand(out); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (o Operation) String() string {
	var sb strings.Builder
	var nils string

	optStr := func(field string, ps *string) {
		if ps != nil {
			fmt.Fprintf(&sb, "  %s: %s\n", field, *ps)
		} else {
			nils += " " + field
		}
	}

	// two spaces then field: value
	str := func(field string, value string) {
		fmt.Fprintf(&sb, "  %s: %s\n", field, value)
	}

	sb.WriteString("Operation {\n")
	str("Go", o.Go)
	str("GoArch", o.GoArch)
	str("Asm", o.Asm)
	str("Commutative", o.Commutative)
	str("Extension", o.Extension)
	optStr("ConstImm", o.ConstImm)
	optStr("Masked", o.Masked)
	optStr("Zeroing", o.Zeroing)
	optStr("OperandOrder", o.OperandOrder)

	sb.WriteString("  In: [\n")
	for _, op := range o.In {
		fmt.Fprintf(&sb, "    %s,\n", op.String())
	}
	sb.WriteString("  ]\n")

	sb.WriteString("  Out: [\n")
	for _, op := range o.Out {
		fmt.Fprintf(&sb, "    %s,\n", op.String())
	}
	sb.WriteString("  ]\n")

	optStr("Documentation", o.Documentation)
	if len(nils) != 0 {
		sb.WriteString("  nils = " + nils[1:] + "\n")
	}

	sb.WriteString("}\n")
	return sb.String()
}

// String returns a string representation of the Operand.
func (op Operand) String() string {
	var sb strings.Builder
	var nils string

	optStr := func(field string, ps *string) {
		if ps != nil {
			fmt.Fprintf(&sb, "    %s: %s\n", field, *ps)
		} else {
			nils += " " + field
		}
	}

	optNum := func(field string, pi *int) {
		if pi != nil {
			fmt.Fprintf(&sb, "    %s: %d\n", field, *pi)
		} else {
			nils += " " + field
		}
	}

	// four spaces then field: value
	str := func(field string, value string) {
		fmt.Fprintf(&sb, "    %s: %s\n", field, value)
	}
	num := func(field string, value int) {
		fmt.Fprintf(&sb, "    %s: %d\n", field, value)
	}
	sb.WriteString("Operand {\n")
	str("Class", op.Class)
	optStr("Go", op.Go)
	num("AsmPos", op.AsmPos)
	optStr("Base", op.Base)
	optNum("ElemBits", op.ElemBits)
	optNum("Bits", op.Bits)
	optStr("Const", op.Const)
	optStr("ImmOffset", op.ImmOffset)
	optNum("Lanes", op.Lanes)
	optStr("Name", op.Name)
	optNum("TreatLikeAScalarOfSize", op.TreatLikeAScalarOfSize)
	optStr("OverwriteClass", op.OverwriteClass)
	optStr("OverwriteBase", op.OverwriteBase)
	optNum("OverwriteElementBits", op.OverwriteElementBits)

	if len(nils) != 0 {
		sb.WriteString("    nils = " + nils[1:] + "\n")
	}

	sb.WriteString("  }\n")
	return sb.String()
}
