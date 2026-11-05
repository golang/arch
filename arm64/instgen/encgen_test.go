// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEncoderLiteral(t *testing.T) {
	syms := newSymtab()
	arngB, arngH := syms.get("B"), syms.get("H")
	for _, c := range []struct {
		name string
		d    dDesc
		want string
	}{
		{"scaled signed immediate",
			dDesc{Fields: []dField{{16, 4}}, Signed: true, HasRng: true, Min: -32, Max: 28, Stride: 4, AltBit: -1, Fixed: -1},
			"encField{parts: []encPart{{16, 4}}, signed: true, hasRange: true, min: -32, max: 28, stride: 4}"},
		{"inverted shift",
			dDesc{Fields: []dField{{16, 3}}, HasRng: true, Min: 1, Max: 8, Stride: 1, Invert: 8, AltBit: -1, Fixed: -1},
			"encField{parts: []encPart{{16, 3}}, hasRange: true, min: 1, max: 8, invert: 8}"},
		{"second of a pair",
			dDesc{Role: roleReg, Fields: []dField{{0, 4}}, Regfile: rfP, Bias: 1, Stride: 1, AltBit: -1, Fixed: -1},
			"encField{parts: []encPart{{0, 4}}, hasRange: true, min: 1, max: 16, bias: 1}"},
		{"predicate as counter",
			dDesc{Role: roleReg, Fields: []dField{{0, 3}}, Regfile: rfPN, HasRng: true, Min: 8, Max: 15, Bias: 8, Stride: 1, AltBit: -1, Fixed: -1},
			"encField{parts: []encPart{{0, 3}}, hasRange: true, min: 24, max: 31, bias: 24}"},
		{"optional shift",
			dDesc{Fields: []dField{{5, 8}}, HasRng: true, Min: 0, Max: 255, Stride: 1, AltBit: 13, AltSh: 8, Fixed: -1},
			"encField{parts: []encPart{{5, 8}}, hasRange: true, min: 0, max: 255, altBit: 0x2000, altShift: 8}"},
		{"table",
			dDesc{Fields: []dField{{19, 2}}, Table: []dRow{{Mask: 3, Value: 1, Sym: arngB}, {Mask: 2, Value: 2, Sym: arngH}}, Stride: 1, AltBit: -1, Fixed: -1},
			"encField{parts: []encPart{{19, 2}}, table: []encRow{{uint32(ARNG_B), 1}, {uint32(ARNG_H), 2}}}"},
	} {
		got, err := encoderLiteral(c.d, syms)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
		} else if got != c.want {
			t.Errorf("%s:\ngot  %s\nwant %s", c.name, got, c.want)
		}
	}
}

func TestEncoderLiteralRefusesWhatDependsOnTheElementSize(t *testing.T) {
	d := dDesc{Fields: []dField{{16, 3}}, TszMode: "tszShiftR", AltBit: -1, Fixed: -1}
	if got, err := encoderLiteral(d, newSymtab()); err == nil {
		t.Errorf("encoderLiteral accepted a descriptor that depends on the element size: %s", got)
	}
}

func TestEncoderRuntimeParses(t *testing.T) {
	if _, err := parser.ParseFile(token.NewFileSet(), "runtime.go", "package arm64\n"+encoderRuntime, 0); err != nil {
		t.Fatal(err)
	}
}

// encoderStubs stands in for the parts of package arm64 that encoderRuntime and
// the helpers before it in encodingTmpl refer to. The numbers only have to keep
// the order of the real ones: raw register numbers below obj.RBaseARM64, then
// the R, Z and Z-arrangement ranges, then REG_RSP. The Z ranges start at a
// multiple of 32, as the real ones must, because stripRawZ takes the low 5 bits.
const encoderStubs = `package arm64

const (
	RBaseARM64    = 1 << 13
	REG_R31       = RBaseARM64 + 31
	REG_Z0        = RBaseARM64 + 256
	REG_Z31       = REG_Z0 + 31
	REG_ZARNG     = REG_Z31 + 1
	REG_ZARNGELEM = REG_ZARNG + 160
	REG_RSP       = (REG_ZARNGELEM + 64) | 0x1f
)
`

// encoderCases drive the generated runtime. Each is an encField as the generator
// writes it, a value, and the word or the refusal that the field must give. The
// expected words are worked out by hand.
const encoderCases = `package arm64

import "testing"

func TestEncodeField(t *testing.T) {
	for _, c := range []struct {
		name string
		f    encField
		v    uint32
		want uint32
		ok   bool
	}{
		{"field copy", encField{parts: []encPart{{5, 5}}}, 7, 7 << 5, true},
		{"field copy stays in its field", encField{parts: []encPart{{5, 5}}}, 33, 1 << 5, true},
		{"split field, low part first", encField{parts: []encPart{{5, 3}, {16, 2}}}, 0b10110, 0b110<<5 | 0b10<<16, true},

		{"scaled signed, most negative", scaled, ^uint32(31), 8 << 16, true},
		{"scaled signed, largest", scaled, 28, 7 << 16, true},
		{"scaled signed, zero", scaled, 0, 0, true},
		{"scaled signed, minus four", scaled, ^uint32(3), 15 << 16, true},
		{"scaled signed, too large", scaled, 32, 0, false},
		{"scaled signed, too small", scaled, ^uint32(35), 0, false},
		{"scaled signed, not a multiple", scaled, 26, 0, false},
		{"scaled signed, negative, not a multiple", scaled, ^uint32(1), 0, false},

		{"bias, bottom", biased, 8, 0, true},
		{"bias, top", biased, 15, 7, true},
		{"bias, below", biased, 7, 0, false},
		{"bias, above", biased, 16, 0, false},

		{"register plus one, bottom", plusOne, 1, 0, true},
		{"register plus one, top", plusOne, 16, 15, true},
		{"register plus one, below", plusOne, 0, 0, false},
		{"register plus one, above", plusOne, 17, 0, false},

		{"modulo wraps below", encField{parts: []encPart{{0, 5}}, modulo: 32, bias: 1}, 0, 31, true},
		{"modulo reduces", encField{parts: []encPart{{0, 5}}, modulo: 32, bias: 1}, 33, 0, true},

		{"inverted, middle", inverted, 6, 2 << 16, true},
		{"inverted, bottom of the range", inverted, 1, 7 << 16, true},
		{"inverted, top of the range", inverted, 8, 0, true},
		{"inverted, below", inverted, 0, 0, false},
		{"inverted, above", inverted, 9, 0, false},

		{"optional shift, off", alt, 9, 9 << 5, true},
		{"optional shift, on", alt, 9 << 8, 9<<5 | 0x2000, true},
		{"optional shift, not a multiple", alt, 9<<8 | 1, 0, false},
		{"optional shift, too large", alt, 256 << 8, 0, false},
		{"optional shift, signed", encField{parts: []encPart{{5, 8}}, signed: true, hasRange: true, min: -128, max: 127, altBit: 0x2000, altShift: 8}, ^uint32(255), 0xff<<5 | 0x2000, true},

		{"table, first row", table, 100, 1 << 22, true},
		{"table, second row", table, 200, 2 << 22, true},
		{"table, no row", table, 300, 0, false},
		{"flags, first", flags, 1, 0, true},
		{"flags, second", flags, 2, 1 << 14, true},
		{"flags, none", flags, 0, 0, false},
		{"flags, others", flags, 4, 0, false},

		{"Z, raw number", encField{parts: []encPart{{5, 5}}, reg: encRegZ}, 3, 3 << 5, true},
		{"Z, register", encField{parts: []encPart{{5, 5}}, reg: encRegZ}, REG_Z0 + 9, 9 << 5, true},
		{"Z, last register", encField{parts: []encPart{{5, 5}}, reg: encRegZ}, REG_Z31, 31 << 5, true},
		{"Z, arrangement", encField{parts: []encPart{{5, 5}}, reg: encRegZ}, REG_ZARNG + 2, 2 << 5, true},
		{"Z, not a Z register", encField{parts: []encPart{{5, 5}}, reg: encRegZ}, REG_R31, 0, false},
		{"Z, past the arrangements", encField{parts: []encPart{{5, 5}}, reg: encRegZ}, REG_ZARNGELEM, 0, false},

		{"R, raw number", encField{parts: []encPart{{16, 5}}, reg: encRegR}, 5, 5 << 16, true},
		{"R, last register", encField{parts: []encPart{{16, 5}}, reg: encRegR}, REG_R31, 31 << 16, true},
		{"R, not an R register", encField{parts: []encPart{{16, 5}}, reg: encRegR}, REG_Z0, 0, false},
		{"R or SP, register", encField{parts: []encPart{{16, 5}}, reg: encRegR, sp: true}, RBaseARM64 + 3, 3 << 16, true},
		{"R or SP, zero register", encField{parts: []encPart{{16, 5}}, reg: encRegR, sp: true}, REG_R31, 0, false},
		{"R or SP, stack pointer", encField{parts: []encPart{{16, 5}}, reg: encRegR, sp: true}, REG_RSP, 31 << 16, true},
		{"R or ZR, absent", encField{parts: []encPart{{16, 5}}, reg: encRegR, zr: true}, 0, 31 << 16, true},
		{"R or ZR, present", encField{parts: []encPart{{16, 5}}, reg: encRegR, zr: true}, RBaseARM64 + 7, 7 << 16, true},

		{"V", encField{parts: []encPart{{0, 5}}, reg: encRegV}, RBaseARM64 + 37, 5, true},

		{"predicate counter, bottom", encField{parts: []encPart{{0, 3}}, hasRange: true, min: 24, max: 31, bias: 24}, 24, 0, true},
		{"predicate counter, top", encField{parts: []encPart{{0, 3}}, hasRange: true, min: 24, max: 31, bias: 24}, 31, 7, true},
		{"predicate counter, below", encField{parts: []encPart{{0, 3}}, hasRange: true, min: 24, max: 31, bias: 24}, 23, 0, false},
		{"predicate counter, above", encField{parts: []encPart{{0, 3}}, hasRange: true, min: 24, max: 31, bias: 24}, 32, 0, false},
	} {
		f := c.f
		got, ok := encodeField(&f, c.v)
		if ok != c.ok || got != c.want {
			t.Errorf("%s: encodeField(%#x) = %#x, %v, want %#x, %v", c.name, c.v, got, ok, c.want, c.ok)
		}
	}
}

var (
	scaled   = encField{parts: []encPart{{16, 4}}, signed: true, hasRange: true, min: -32, max: 28, stride: 4}
	biased   = encField{parts: []encPart{{0, 3}}, hasRange: true, min: 8, max: 15, bias: 8}
	plusOne  = encField{parts: []encPart{{0, 4}}, hasRange: true, min: 1, max: 16, bias: 1}
	inverted = encField{parts: []encPart{{16, 3}}, hasRange: true, min: 1, max: 8, invert: 8}
	alt      = encField{parts: []encPart{{5, 8}}, hasRange: true, min: 0, max: 255, altBit: 0x2000, altShift: 8}
	table    = encField{parts: []encPart{{22, 2}}, table: []encRow{{100, 1}, {200, 2}}}
	flags    = encField{parts: []encPart{{14, 1}}, table: []encRow{{1, 0}, {2, 1}}, flags: true}
)
`

// TestEncoderRuntime compiles the runtime that is emitted into encoding_gen.go
// with the helpers it calls, and runs encoderCases on it. The runtime is text
// until the assembler is built, so without this nothing in this module would
// notice a mistake in it.
func TestEncoderRuntime(t *testing.T) {
	goTool := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goTool); err != nil {
		var err error
		if goTool, err = exec.LookPath("go"); err != nil {
			t.Skip("no go command to compile the runtime with")
		}
	}
	// The helpers are the text of the template that precedes the runtime.
	// Package obj is where RBaseARM64 lives in the assembler's tree.
	i := strings.Index(encodingTmpl, "// stripRawZ")
	j := strings.Index(encodingTmpl, "{{.Runtime}}")
	if i < 0 || j < i {
		t.Fatal("the helpers are not where this test looks for them in encodingTmpl")
	}
	helpers := strings.ReplaceAll(encodingTmpl[i:j], "obj.", "")

	dir := t.TempDir()
	for name, text := range map[string]string{
		"go.mod":          "module encoderruntime\n\ngo 1.21\n",
		"stubs.go":        encoderStubs,
		"runtime.go":      "package arm64\n" + helpers + encoderRuntime,
		"runtime_test.go": encoderCases,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(goTool, "test", "-count=1", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("the runtime fails its cases: %v\n%s", err, out)
	}
}
