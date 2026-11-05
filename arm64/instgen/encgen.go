// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The encoder side of the operand descriptors. An entry of encodingImpls whose
// body is empty has no hand-written encoder: its encode function is generated,
// as a call of encodeField on a descriptor derived from the ARM prose by
// descriptors.go. The decoder generator derives the same descriptors for the
// same prose, which is what makes the two directions one description.

package main

import (
	"fmt"
	"strings"

	"golang.org/x/arch/arm64/instgen/xmlspec"
)

// encoderRuntime is emitted into encoding_gen.go. It is the whole of the
// interpretation of a descriptor in the encode direction. Package arm64 cannot
// import arm64asm, where the decode direction has its own runtime, so this is
// emitted into the assembler's tree and not shared as code.
const encoderRuntime = `
// encPart is a run of width bits at bit lo of the instruction word.
type encPart struct{ lo, width uint }

// encRow maps an operand value to the value of the field.
type encRow struct{ sym, field uint32 }

// encReg says what the value of a register field is checked and reduced with.
type encReg uint8

const (
	encRegNone encReg = iota // not a register, or a predicate register, which arrives as its number
	encRegZ                  // a scalable vector register
	encRegR                  // a general-purpose register
	encRegV                  // a SIMD&FP register
)

// An encField says how the value of one operand element is stored in an
// instruction word. It is derived from the description in the ARM XML, and the
// disassembler's tables are derived from the same description.
type encField struct {
	parts []encPart // where the field value goes, least significant first

	reg encReg
	sp  bool // register 31 is the stack pointer, and ZR is not accepted
	zr  bool // an absent register, v == 0, stands for register 31

	signed   bool // v is a two's complement immediate
	hasRange bool
	min, max int64 // the values of v that are accepted, if hasRange

	// The field holds invert - (v - bias) / stride, or, if invert is 0,
	// (v - bias) / stride, taken modulo modulo if that is not 0.
	bias, stride, modulo, invert int64

	// altBit is set, and v is shifted right by altShift first, if v is only
	// in range after that: an optional "LSL #8".
	altBit   uint32
	altShift uint

	table []encRow // the values of v that are accepted, if there is one
	flags bool     // the entries of table are bits that v may have set
}

func (f *encField) scatter(n uint32) uint32 {
	var out uint32
	for _, p := range f.parts {
		out |= (n & (1<<p.width - 1)) << p.lo
		n >>= p.width
	}
	return out
}

// value returns the field value for v.
func (f *encField) value(v int64) (int64, bool) {
	if f.hasRange && (v < f.min || v > f.max) {
		return 0, false
	}
	n := v - f.bias
	if f.stride > 1 {
		if n%f.stride != 0 {
			return 0, false
		}
		n /= f.stride
	}
	if f.modulo > 0 {
		n = (n%f.modulo + f.modulo) % f.modulo
	}
	if f.invert != 0 {
		n = f.invert - n
	}
	return n, true
}

// encodeField returns the bits of the instruction word for the operand value v,
// or false if the field cannot hold v.
func encodeField(f *encField, v uint32) (uint32, bool) {
	switch f.reg {
	case encRegZ:
		if !stripRawZ(&v) {
			return 0, false
		}
	case encRegR:
		if f.zr && v == 0 {
			return f.scatter(31), true
		}
		if !checkIsR(v) {
			return 0, false
		}
		if f.sp {
			if v == REG_R31 {
				return 0, false
			}
			if v == REG_RSP {
				return f.scatter(31), true
			}
		}
		v &= 31
	case encRegV:
		v &= 31
	}
	for _, r := range f.table {
		if f.flags && v&r.sym != 0 || !f.flags && v == r.sym {
			return f.scatter(r.field), true
		}
	}
	if f.table != nil {
		return 0, false
	}
	sv := int64(v)
	if f.signed {
		sv = int64(int32(v))
	}
	if n, ok := f.value(sv); ok {
		return f.scatter(uint32(n)), true
	}
	if f.altShift != 0 && sv&(1<<f.altShift-1) == 0 {
		if n, ok := f.value(sv >> f.altShift); ok {
			return f.scatter(uint32(n)) | f.altBit, true
		}
	}
	return 0, false
}
`

// encoderSyms maps the spellings that a value table names to the values the
// assembler passes to an encode function: the arrangement constants, and, for
// an index modifier, a bit for each of UXTW and SXTW, because the operand
// matcher passes the modifier as a set of flags.
var encoderSyms = map[string]string{
	"B": "uint32(ARNG_B)", "H": "uint32(ARNG_H)", "S": "uint32(ARNG_S)",
	"D": "uint32(ARNG_D)", "Q": "uint32(ARNG_Q)",
	"UXTW": "1", "SXTW": "2",
}

// descsOf records the descriptor of every element of enc that has no
// hand-written encoder, and reports a description that two elements derive
// differently.
func descsOf(enc xmlspec.IREncoding, syms *symtab, descs map[string]dDesc) error {
	for _, op := range enc.Operands {
		for _, el := range op.Elems {
			key := strings.Join(strings.Fields(el.TextExpWithRanges), " ")
			impl, ok := encodingImpls[key]
			if !ok || impl.body != "" {
				continue
			}
			d, err := derive(el, syms, op.Typ)
			if err != nil {
				return fmt.Errorf("%s: %v", impl.name, err)
			}
			// The role is the decoder's business: it depends on the class of the
			// operand that the element appears in, and the encoder does not use it.
			d.Role = roleSkip
			if old, ok := descs[key]; ok && fmt.Sprint(old) != fmt.Sprint(d) {
				return fmt.Errorf("%s: derived two different descriptors:\n%+v\n%+v", impl.name, old, d)
			}
			descs[key] = d
		}
	}
	return nil
}

// encoderLiteral returns the Go composite literal of the encField for d.
func encoderLiteral(d dDesc, syms *symtab) (string, error) {
	if d.TszMode != "" || d.Bitmask || d.FP8 || d.Fixed >= 0 || len(d.Fields) == 0 {
		return "", fmt.Errorf("the descriptor %+v depends on more than its own field", d)
	}
	var parts []string
	add := func(f string, a ...any) { parts = append(parts, fmt.Sprintf(f, a...)) }

	var ps []string
	for _, f := range d.Fields {
		ps = append(ps, fmt.Sprintf("{%d, %d}", f.Lo, f.Width))
	}
	add("parts: []encPart{%s}", strings.Join(ps, ", "))

	// The assembler numbers the predicate-as-counter registers from 16, and
	// passes them on as such.
	if d.Regfile == rfPN {
		if !d.HasRng {
			d.HasRng, d.Min, d.Max = true, 0, 15
		}
		d.Min, d.Max, d.Bias = d.Min+16, d.Max+16, d.Bias+16
	}
	// A register that is not simply the field's own value is still limited to
	// what the field can hold: the field of "Pd + 1" holds 1 to 16, not any
	// number, and a larger one must not be written over the neighbouring bits.
	if !d.HasRng && len(d.Table) == 0 && d.Modulo == 0 && (d.Bias != 0 || d.Stride > 1) {
		d.HasRng, d.Min, d.Max = true, d.Bias, d.Bias+d.Stride*(int64(1)<<uint(d.width())-1)
	}
	switch d.Regfile {
	case rfZ:
		add("reg: encRegZ")
	case rfR32, rfR64, rfRauto:
		add("reg: encRegR")
	case rfV:
		add("reg: encRegV")
	}
	if d.SP {
		add("sp: true")
	}
	if d.ZR {
		add("zr: true")
	}
	if d.Signed {
		add("signed: true")
	}
	if d.HasRng {
		add("hasRange: true")
		add("min: %d", d.Min)
		add("max: %d", d.Max)
	}
	if d.Bias != 0 {
		add("bias: %d", d.Bias)
	}
	if d.Stride > 1 {
		add("stride: %d", d.Stride)
	}
	if d.Modulo != 0 {
		add("modulo: %d", d.Modulo)
	}
	if d.Invert != 0 {
		add("invert: %d", d.Invert)
	}
	if d.AltBit >= 0 {
		add("altBit: %#x", uint32(1)<<uint(d.AltBit))
		add("altShift: %d", d.AltSh)
	}
	if len(d.Table) > 0 {
		var rows []string
		flags := false
		for _, r := range d.Table {
			sym := syms.all[r.Sym]
			expr, ok := encoderSyms[sym]
			if !ok {
				return "", fmt.Errorf("the table names %q, which the assembler does not pass to an encoder", sym)
			}
			rows = append(rows, fmt.Sprintf("{%s, %d}", expr, r.Value))
			flags = flags || sym == "UXTW" || sym == "SXTW"
		}
		add("table: []encRow{%s}", strings.Join(rows, ", "))
		if flags {
			add("flags: true")
		}
	}
	return "encField{" + strings.Join(parts, ", ") + "}", nil
}
