// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xmlspec

// The parsed intermediate form keeps several facts that an output target needs
// in unexported fields. IREncoding is a flat, read-only view of one encoding
// that exposes them, so that a generator in package main can read them without
// the fields being exported.

// IRElement is the part of an Element that a generator reads.
type IRElement struct {
	// Symbol is the name of the assembler symbol the element belongs to.
	Symbol string
	// TextExpWithRanges is the element's description, extended with the bit
	// ranges of the fields it is encoded in.
	TextExpWithRanges string
	// FixedScalarWidth is the width in bits of a scalar register whose width
	// the architecture fixes, or 0.
	FixedScalarWidth int
	// IsP and IsZ say that the element names a predicate or a vector register.
	IsP, IsZ bool
}

// IROperand is an Operand with its elements flattened.
type IROperand struct {
	Name  string
	Typ   string
	Elems []IRElement
}

// IREncoding is a read-only view of one EncodingParsed.
type IREncoding struct {
	// Asm is the assembler template, and GoOp the Go opcode.
	Asm  string
	GoOp string

	// A word w is an instance of the encoding when w&Mask == Binary, except
	// that an encoding produced by SplitInstByRegWidth (SplitHi != 0) accepts
	// any of SplitValues in bits [SplitLo, SplitHi) instead of the single
	// value Binary carries there.
	Binary, Mask     uint32
	SplitLo, SplitHi int
	SplitValues      []uint32

	// Operands are in the order instgen stores them, which is Go assembler
	// order.
	Operands []IROperand
}

// IRFor returns the view of enc.
func IRFor(enc EncodingParsed) IREncoding {
	e := IREncoding{
		Asm:         enc.Asm,
		GoOp:        enc.GoOp,
		Binary:      enc.Binary,
		Mask:        enc.mask,
		SplitLo:     enc.splitLo,
		SplitHi:     enc.splitHi,
		SplitValues: enc.splitValues,
	}
	for _, op := range enc.Operands {
		o := IROperand{Name: op.Name, Typ: op.Typ}
		for _, el := range op.Elems {
			o.Elems = append(o.Elems, IRElement{
				Symbol:            el.symbol,
				TextExpWithRanges: el.TextExpWithRanges,
				FixedScalarWidth:  el.fixedScalarWidth,
				IsP:               el.isP,
				IsZ:               el.isZ,
			})
		}
		e.Operands = append(e.Operands, o)
	}
	return e
}
