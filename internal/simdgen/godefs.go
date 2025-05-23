// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"

	"golang.org/x/arch/internal/unify"
)

type Operation struct {
	Go string // Go method name

	GoArch string // GOARCH for this definition
	Asm    string // Assembly mnemonic

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
}

type Operand struct {
	Class string // One of "mask", "immediate", "vreg" and "mem"

	Go     *string // Go type of this operand
	AsmPos int     // Position of this operand in the assembly instruction

	Base     *string // Base Go type ("int", "uint", "float")
	ElemBits *int    // Element bit width
	Bits     *int    // Total vector bit width

	Const *string // Optional constant value
	Lanes *int    // Lanes should equal Bits/ElemBits
	// If non-nil, it means the [Class] field is overwritten here, right now this is used to
	// overwrite the results of AVX2 compares to masks.
	OverwriteClass *string
	// If non-nil, it means the [Base] field is overwritten here. This field exist solely
	// because Intel's XED data is inconsistent. e.g. VANDNP[SD] marks its operand int.
	OverwriteBase *string
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
	// The parsed XED data might contain duplicates, like
	// 512 bits VPADDP.
	deduped := dedup(ops)
	log.Printf("dedup len: %d\n", len(ops))
	var err error
	if err = overwrite(deduped); err != nil {
		return err
	}
	log.Printf("dedup len: %d\n", len(deduped))
	if !*FlagNoSplitMask {
		if deduped, err = splitMask(deduped); err != nil {
			return err
		}
	}
	log.Printf("dedup len: %d\n", len(deduped))
	if !*FlagNoDedup {
		if deduped, err = dedupGodef(deduped); err != nil {
			return err
		}
	}
	log.Printf("dedup len: %d\n", len(deduped))
	if !*FlagNoConstImmPorting {
		if err = copyConstImm(deduped); err != nil {
			return err
		}
	}
	log.Printf("dedup len: %d\n", len(deduped))
	typeMap := parseSIMDTypes(deduped)
	if err = writeSIMDTypes(path, typeMap); err != nil {
		return err
	}
	if err = writeSIMDStubs(path, deduped, typeMap); err != nil {
		return err
	}
	if err = writeSIMDIntrinsics(path, deduped, typeMap); err != nil {
		return err
	}
	if err = writeSIMDGenericOps(path, deduped); err != nil {
		return err
	}
	if err = writeSIMDMachineOps(path, deduped); err != nil {
		return err
	}
	if err = writeSIMDRules(path, deduped); err != nil {
		return err
	}
	if err = writeSIMDSSA(path, deduped); err != nil {
		return err
	}
	return nil
}
