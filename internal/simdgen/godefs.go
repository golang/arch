// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/arch/internal/unify"
)

type Operation struct {
	Go string // Go method name

	GoArch       string  // GOARCH for this definition
	Asm          string  // Assembly mnemonic
	OperandOrder *string // optional Operand order for better Go declarations

	In            []Operand // Arguments
	Out           []Operand // Results
	Commutative   string    // Commutativity
	Extension     string    // Extension
	Zeroing       *string   // Zeroing is a flag for asm prefix "Z", if non-nil it will always be "false"
	Documentation *string   // Documentation will be appended to the stubs comments.
	// ConstMask is a hack to reduce the size of defs the user writes for const-immediate
	// If present, it will be copied to [In[0].Const].
	ConstImm *string
	// Masked indicates that this is a masked operation, this field has to be set for masked operations
	// otherwise simdgen won't recognize it in [splitMask].
	Masked *string
	// NameAndSizeCheck is used to check [BWDQ] maps to (8|16|32|64) elemBits.
	NameAndSizeCheck *string
}

func (o *Operation) VectorWidth() int {
	out := o.Out[0]
	if out.Class == "vreg" {
		return *out.Bits
	} else if out.Class == "greg" || out.Class == "mask" {
		for i := range o.In {
			if o.In[i].Class == "vreg" {
				return *o.In[i].Bits
			}
		}
	}
	panic(fmt.Errorf("Figure out what the vector width is for %v and implement it", *o))
}

func compareStringPointers(x, y *string) int {
	if x != nil && y != nil {
		return compareNatural(*x, *y)
	}
	if x == nil && y == nil {
		return 0
	}
	if x == nil {
		return -1
	}
	return 1
}

func compareIntPointers(x, y *int) int {
	if x != nil && y != nil {
		return *x - *y
	}
	if x == nil && y == nil {
		return 0
	}
	if x == nil {
		return -1
	}
	return 1
}

func compareOperations(x, y Operation) int {
	if c := compareNatural(x.Go, y.Go); c != 0 {
		return c
	}
	xIn, yIn := x.In, y.In

	if len(xIn) > len(yIn) && xIn[len(xIn)-1].Class == "mask" {
		xIn = xIn[:len(xIn)-1]
	} else if len(xIn) < len(yIn) && yIn[len(yIn)-1].Class == "mask" {
		yIn = yIn[:len(yIn)-1]
	}

	if len(xIn) < len(yIn) {
		return -1
	}
	if len(xIn) > len(yIn) {
		return 1
	}
	if len(x.Out) < len(y.Out) {
		return -1
	}
	if len(x.Out) > len(y.Out) {
		return 1
	}
	for i := range xIn {
		ox, oy := &xIn[i], &yIn[i]
		if c := compareOperands(ox, oy); c != 0 {
			return c
		}
	}
	return 0
}

func compareOperands(x, y *Operand) int {
	if c := compareNatural(x.Class, y.Class); c != 0 {
		return c
	}
	if x.Class == "immediate" {
		return compareStringPointers(x.ImmOffset, y.ImmOffset)
	} else {
		if c := compareStringPointers(x.Base, y.Base); c != 0 {
			return c
		}
		if c := compareIntPointers(x.ElemBits, y.ElemBits); c != 0 {
			return c
		}
		if c := compareIntPointers(x.Bits, y.Bits); c != 0 {
			return c
		}
		return 0
	}
}

type Operand struct {
	Class string // One of "mask", "immediate", "vreg", "greg", and "mem"

	Go     *string // Go type of this operand
	AsmPos int     // Position of this operand in the assembly instruction

	Base     *string // Base Go type ("int", "uint", "float")
	ElemBits *int    // Element bit width
	Bits     *int    // Total vector bit width

	Const *string // Optional constant value for immediates.
	// Optional immediate arg offsets. If this field is non-nil,
	// This operand will be an immediate operand:
	// The compiler will right-shift the user-passed value by ImmOffset and set it as the AuxInt
	// field of the operation.
	ImmOffset *string
	Name      *string // optional name in the Go intrinsic declaration
	Lanes     *int    // *Lanes equals Bits/ElemBits except for scalars, when *Lanes == 1
	// TreatLikeAScalarOfSize means only the lower $TreatLikeAScalarOfSize bits of the vector
	// is used, so at the API level we can make it just a scalar value of this size; Then we
	// can overwrite it to a vector of the right size during intrinsics stage.
	TreatLikeAScalarOfSize *int
	// If non-nil, it means the [Class] field is overwritten here, right now this is used to
	// overwrite the results of AVX2 compares to masks.
	OverwriteClass *string
	// If non-nil, it means the [Base] field is overwritten here. This field exist solely
	// because Intel's XED data is inconsistent. e.g. VANDNP[SD] marks its operand int.
	OverwriteBase *string
	// If non-nil, it means the [ElementBits] field is overwritten. This field exist solely
	// because Intel's XED data is inconsistent. e.g. AVX512 VPMADDUBSW marks its operand
	// elemBits 16, which should be 8.
	OverwriteElementBits *int
}

// isDigit returns true if the byte is an ASCII digit.
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// compareNatural performs a "natural sort" comparison of two strings.
// It compares non-digit sections lexicographically and digit sections
// numerically.  In the case of string-unequal "equal" strings like
// "a01b" and "a1b", strings.Compare breaks the tie.
//
// It returns:
//
//	-1 if s1 < s2
//	 0 if s1 == s2
//	+1 if s1 > s2
func compareNatural(s1, s2 string) int {
	i, j := 0, 0
	len1, len2 := len(s1), len(s2)

	for i < len1 && j < len2 {
		// Find a non-digit segment or a number segment in both strings.
		if isDigit(s1[i]) && isDigit(s2[j]) {
			// Number segment comparison.
			numStart1 := i
			for i < len1 && isDigit(s1[i]) {
				i++
			}
			num1, _ := strconv.Atoi(s1[numStart1:i])

			numStart2 := j
			for j < len2 && isDigit(s2[j]) {
				j++
			}
			num2, _ := strconv.Atoi(s2[numStart2:j])

			if num1 < num2 {
				return -1
			}
			if num1 > num2 {
				return 1
			}
			// If numbers are equal, continue to the next segment.
		} else {
			// Non-digit comparison.
			if s1[i] < s2[j] {
				return -1
			}
			if s1[i] > s2[j] {
				return 1
			}
			i++
			j++
		}
	}

	// deal with a01b vs a1b; there needs to be an order.
	return strings.Compare(s1, s2)
}

func writeGoDefs(path string, cl unify.Closure) error {
	// TODO: Merge operations with the same signature but multiple
	// implementations (e.g., SSE vs AVX)
	var ops []Operation
	for def := range cl.All() {
		var op Operation
		if !def.Exact() {
			continue
		}
		if err := def.Decode(&op); err != nil {
			log.Println(err.Error())
			log.Println(def)
			continue
		}
		// TODO: verify that this is safe.
		op.sortOperand()
		ops = append(ops, op)
	}
	slices.SortFunc(ops, compareOperations)
	// The parsed XED data might contain duplicates, like
	// 512 bits VPADDP.
	deduped := dedup(ops)

	if *Verbose {
		log.Printf("dedup len: %d\n", len(ops))
	}
	var err error
	if err = overwrite(deduped); err != nil {
		return err
	}
	if *Verbose {
		log.Printf("dedup len: %d\n", len(deduped))
	}
	if !*FlagNoSplitMask {
		if deduped, err = splitMask(deduped); err != nil {
			return err
		}
	}
	if *Verbose {
		log.Printf("dedup len: %d\n", len(deduped))
	}
	if !*FlagNoDedup {
		if deduped, err = dedupGodef(deduped); err != nil {
			return err
		}
	}
	if *Verbose {
		log.Printf("dedup len: %d\n", len(deduped))
	}
	if !*FlagNoConstImmPorting {
		if err = copyConstImm(deduped); err != nil {
			return err
		}
	}
	if *Verbose {
		log.Printf("dedup len: %d\n", len(deduped))
	}
	typeMap := parseSIMDTypes(deduped)

	formatWriteAndClose(writeSIMDTypes(typeMap), path, "src/"+simdPackage+"/types_amd64.go")
	formatWriteAndClose(writeSIMDStubs(deduped, typeMap), path, "src/"+simdPackage+"/ops_amd64.go")
	formatWriteAndClose(writeSIMDTestsWrapper(deduped), path, "src/"+simdPackage+"/simd_wrapped_test.go")
	formatWriteAndClose(writeSIMDIntrinsics(deduped, typeMap), path, "src/cmd/compile/internal/ssagen/simdintrinsics.go")
	formatWriteAndClose(writeSIMDGenericOps(deduped), path, "src/cmd/compile/internal/ssa/_gen/simdgenericOps.go")
	formatWriteAndClose(writeSIMDMachineOps(deduped), path, "src/cmd/compile/internal/ssa/_gen/simdAMD64ops.go")
	formatWriteAndClose(writeSIMDSSA(deduped), path, "src/cmd/compile/internal/amd64/simdssa.go")
	writeAndClose(writeSIMDRules(deduped).Bytes(), path, "src/cmd/compile/internal/ssa/_gen/simdAMD64.rules")

	return nil
}
