// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"testing"

	"golang.org/x/arch/arm64/instgen/xmlspec"
)

// The descriptions below are the ARM prose that the encoder generator keys on,
// as they appear in encodings.go. These tests need neither the XML
// specification nor the GNU toolchain.

const (
	proseZd = "Is the name of the destination scalable vector register, encoded in the \"Zd\" field.\nbit range mappings:\nZd: [0:5)\n"
	prose8  = "For the \"8-bit\" variant: is the immediate shift amount, in the range 1 to 8, encoded in the \"imm3\" field.\nbit range mappings:\nimm3: [16:19)\n"
)

func TestDeriveRegister(t *testing.T) {
	d, err := derive(xmlspec.IRElement{Symbol: "<Zd>", TextExpWithRanges: proseZd, IsZ: true}, newSymtab(), "AC_ZREG")
	if err != nil {
		t.Fatal(err)
	}
	if d.Role != roleReg || d.Regfile != rfZ || len(d.Fields) != 1 || d.Fields[0] != (dField{0, 5}) {
		t.Errorf("derive(Zd) = %+v, want a Z register in bits [0,5)", d)
	}
}

// The 8-bit variant of the narrowing shifts holds esize - shift, which the prose
// does not say.
func TestDeriveInvertedShift(t *testing.T) {
	d, err := derive(xmlspec.IRElement{Symbol: "<const>", TextExpWithRanges: prose8}, newSymtab(), "AC_IMM")
	if err != nil {
		t.Fatal(err)
	}
	if d.Invert != 8 || !d.HasRng || d.Min != 1 || d.Max != 8 {
		t.Errorf("derive(imm3) = %+v, want invert 8 over 1..8", d)
	}
	if len(d.Fields) != 1 || d.Fields[0] != (dField{16, 3}) {
		t.Errorf("derive(imm3) fields = %+v, want bits [16,19)", d.Fields)
	}
}

// The second destination of a pair of predicates is the first plus one, and
// the prose names only the first in the field.
func TestDeriveSecondDestination(t *testing.T) {
	const prose = "Is the name of the second destination scalable predicate register, encoded in the \"Pd\" field.\nbit range mappings:\nPd: [0:4)\n"
	d, err := derive(xmlspec.IRElement{Symbol: "<Pd2>", TextExpWithRanges: prose, IsP: true}, newSymtab(), "AC_PREG")
	if err != nil {
		t.Fatal(err)
	}
	if d.Role != roleReg || d.Regfile != rfP || d.Bias != 1 {
		t.Errorf("derive(Pd2) = %+v, want a P register with bias 1", d)
	}
}

func TestDeriveDefaultsToXZR(t *testing.T) {
	const prose = "Is the optional 64-bit name of the general-purpose offset register, defaulting to XZR, encoded in the \"Rm\" field.\nbit range mappings:\nRm: [16:21)\n"
	d, err := derive(xmlspec.IRElement{Symbol: "<Xm>", TextExpWithRanges: prose, FixedScalarWidth: 64}, newSymtab(), "AC_MEM")
	if err != nil {
		t.Fatal(err)
	}
	if !d.ZR || d.Regfile != rfR64 {
		t.Errorf("derive(Xm) = %+v, want a 64-bit register defaulting to ZR", d)
	}
}

// A header of a value table names a field of the encoding, or a bit or bits of
// one. A name or a slice that the encoding does not have is refused, so that the
// table is not read against the wrong bits.
func TestHeaderField(t *testing.T) {
	rs := map[string]dField{"size": {22, 2}, "Rm": {16, 5}}
	for _, tt := range []struct {
		header string
		want   dField
		ok     bool
	}{
		{"size", dField{22, 2}, true},
		{"size[0]", dField{22, 1}, true},
		{"size[1]", dField{23, 1}, true},
		{"size[1:0]", dField{22, 2}, true},
		{"size[1:1]", dField{23, 1}, true},
		{"Rm[3:1]", dField{17, 3}, true},
		{"Rm[4]", dField{20, 1}, true},
		{"size[2]", dField{}, false},
		{"size[2:0]", dField{}, false},
		{"size[0:1]", dField{}, false},
		{"Rm[5]", dField{}, false},
		{"nosuch[0]", dField{}, false},
		{"nosuch", dField{}, false},
		{"size[x]", dField{}, false},
		{"size[]", dField{}, false},
	} {
		got, ok := headerField(tt.header, rs)
		if got != tt.want || ok != tt.ok {
			t.Errorf("headerField(%q) = %v, %v, want %v, %v", tt.header, got, ok, tt.want, tt.ok)
		}
	}
}
