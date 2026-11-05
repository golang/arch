// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/parser"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/arch/arm64/instgen/xmlspec"
)

// The fields the tests refer to. They are placed as in the ARM XML's SVE iclasses,
// and none of these tests needs the XML.
var testFields = map[string]xmlspec.IRField{
	"size": {Lo: 22, Hi: 24},
	"sh":   {Lo: 13, Hi: 14},
	"tszh": {Lo: 22, Hi: 23},
	"tszl": {Lo: 19, Hi: 21},
	"Rm":   {Lo: 16, Hi: 21},
	"tsz":  {Lo: 16, Hi: 21},
}

func compileTest(t *testing.T, cond string) aslNode {
	t.Helper()
	aliases := map[string]aslBits{"tsize": {22, 20, 19}}
	n, err := compileCondition(cond, testFields, aliases)
	if err != nil {
		t.Fatalf("compileCondition(%q): %v", cond, err)
	}
	return n
}

// eval is the meaning of a node, written out separately from the emitter.
func eval(n aslNode, w uint32) bool {
	switch n := n.(type) {
	case aslConst:
		return bool(n)
	case aslMatch:
		return (w&n.mask == n.value) != n.ne
	case aslNot:
		return !eval(n.x, w)
	case aslAnd:
		for _, x := range n {
			if !eval(x, w) {
				return false
			}
		}
		return true
	case aslOr:
		for _, x := range n {
			if eval(x, w) {
				return true
			}
		}
		return false
	}
	panic("eval")
}

func field(w uint32, lo, hi int) uint32 { return w >> uint(lo) & (1<<uint(hi-lo) - 1) }

// A condition compiles to a test of the word that agrees with the pseudocode on
// every setting of the bits it names. The reference functions below read the
// fields the way the ARM manual does, as numbers, so this checks the mask and
// value arithmetic, the concatenation order and the slices against something that
// shares none of it.
func TestConditionMeaning(t *testing.T) {
	size := func(w uint32) uint32 { return field(w, 22, 24) }
	tsize := func(w uint32) uint32 { return field(w, 22, 23)<<2 | field(w, 19, 21) }
	tests := []struct {
		cond string
		want func(w uint32) bool
	}{
		{"size == '00'", func(w uint32) bool { return size(w) == 0 }},
		{"size == '11'", func(w uint32) bool { return size(w) == 3 }},
		{"size != '11'", func(w uint32) bool { return size(w) != 3 }},
		{"size != '10'", func(w uint32) bool { return size(w) != 2 }},
		{"size IN {'0x'}", func(w uint32) bool { return size(w)>>1 == 0 }},
		{"size IN {'1x'}", func(w uint32) bool { return size(w)>>1 == 1 }},
		{"size IN {'x1'}", func(w uint32) bool { return size(w)&1 == 1 }},
		{"size[0] == '0'", func(w uint32) bool { return size(w)&1 == 0 }},
		{"size[1] == '1'", func(w uint32) bool { return size(w)>>1 == 1 }},
		{"size[1:0] == '10'", func(w uint32) bool { return size(w) == 2 }},
		{"Rm == '11111'", func(w uint32) bool { return field(w, 16, 21) == 31 }},
		{"tsz == '00000'", func(w uint32) bool { return field(w, 16, 21) == 0 }},
		// A concatenation reads the first field as the more significant.
		{"size::sh == '001'", func(w uint32) bool { return size(w)<<1|field(w, 13, 14) == 1 }},
		{"sh::size == '001'", func(w uint32) bool { return field(w, 13, 14)<<2|size(w) == 1 }},
		{"tszh::tszl == '100'", func(w uint32) bool { return field(w, 22, 23)<<2|field(w, 19, 21) == 4 }},
		// An alias for a concatenation, as the narrowing instructions bind.
		{"tsize == '000'", func(w uint32) bool { return tsize(w) == 0 }},
		{"!(tsize IN {'001', '010', '100'})", func(w uint32) bool {
			v := tsize(w)
			return !(v == 1 || v == 2 || v == 4)
		}},
		{"size == '00' || Rm == '11111'", func(w uint32) bool { return size(w) == 0 || field(w, 16, 21) == 31 }},
		{"size != '00' && Rm != '11111'", func(w uint32) bool { return size(w) != 0 && field(w, 16, 21) != 31 }},
		{"!(size == '00' || Rm == '11111')", func(w uint32) bool { return !(size(w) == 0 || field(w, 16, 21) == 31) }},
		{"(size == '00' || size == '11') && sh == '1'", func(w uint32) bool {
			return (size(w) == 0 || size(w) == 3) && field(w, 13, 14) == 1
		}},
		// The assumptions.
		{"!IsFeatureImplemented(FEAT_SVE)", func(uint32) bool { return false }},
		{"!IsFeatureImplemented(FEAT_SVE) && !IsFeatureImplemented(FEAT_SME)", func(uint32) bool { return false }},
		{"(!IsFeatureImplemented(FEAT_SVE2) && !IsFeatureImplemented(FEAT_SME2)) || !IsFeatureImplemented(FEAT_FP8)",
			func(uint32) bool { return false }},
		{"!HaveSVE2FP8FMA()", func(uint32) bool { return false }},
		{"MaxImplementedAnyVL() < 256", func(uint32) bool { return false }},
		{"MaxImplementedAnyVL() < 512", func(uint32) bool { return false }},
		{"MaxImplementedAnyVL() < 1024", func(uint32) bool { return false }},
		{"MaxImplementedAnyVL() >= 512", func(uint32) bool { return true }},
		// The assumed vector length is 2048 bits, and every comparison is exact at it.
		{"MaxImplementedAnyVL() < 2049", func(uint32) bool { return true }},
		{"MaxImplementedAnyVL() <= 2048", func(uint32) bool { return true }},
		{"MaxImplementedAnyVL() <= 2047", func(uint32) bool { return false }},
		{"MaxImplementedAnyVL() > 2047", func(uint32) bool { return true }},
		{"MaxImplementedAnyVL() > 2048", func(uint32) bool { return false }},
		{"MaxImplementedAnyVL() >= 2049", func(uint32) bool { return false }},
		{"MaxImplementedAnyVL() == 2048", func(uint32) bool { return true }},
		{"MaxImplementedAnyVL() == 512", func(uint32) bool { return false }},
		{"MaxImplementedAnyVL() != 2048", func(uint32) bool { return false }},
		{"MaxImplementedAnyVL() != 512", func(uint32) bool { return true }},
		// A feature check beside a field test leaves the field test.
		{"!IsFeatureImplemented(FEAT_SVE) || size == '00'", func(w uint32) bool { return size(w) == 0 }},
		{"IsFeatureImplemented(FEAT_SVE) && size == '00'", func(w uint32) bool { return size(w) == 0 }},
		// A pattern of don't-cares is always true.
		{"size == 'xx'", func(uint32) bool { return true }},
		{"size != 'xx'", func(uint32) bool { return false }},
	}
	// Every setting of the bits that any of these conditions reads.
	const bits = 1<<23 | 1<<22 | 0x1f<<16 | 1<<13
	var words []uint32
	for w, ok := uint32(0), true; ok; ok = w != 0 {
		words = append(words, w)
		w = (w - bits) & bits
	}
	for _, tc := range tests {
		n := compileTest(t, tc.cond)
		for _, w := range words {
			if got, want := eval(n, w), tc.want(w); got != want {
				t.Errorf("%s at word %#08x: compiled to %q, which is %v, want %v",
					tc.cond, w, goExpr(n, 0), got, want)
				break
			}
		}
	}
}

func TestGoExpr(t *testing.T) {
	tests := []struct{ cond, want string }{
		{"size == '00'", "w&0xc00000 == 0"},
		{"size != '11'", "w&0xc00000 != 0xc00000"},
		{"size IN {'0x'}", "w&0x800000 == 0"},
		{"size[0] == '0'", "w&0x400000 == 0"},
		{"size::sh == '001'", "w&0xc02000 == 0x2000"},
		{"!(tsize IN {'001', '010', '100'})",
			"!(w&0x580000 == 0x80000 || w&0x580000 == 0x100000 || w&0x580000 == 0x400000)"},
		// Negating a test flips it rather than wrapping it.
		{"!(size == '00')", "w&0xc00000 != 0"},
		{"!!(size == '00')", "w&0xc00000 == 0"},
		// An || inside an && is parenthesized, and the other way round is not.
		{"(size == '00' || size == '11') && sh == '1'", "(w&0xc00000 == 0 || w&0xc00000 == 0xc00000) && w&0x2000 == 0x2000"},
		{"size == '00' || (Rm == '11111' && sh == '1')", "w&0xc00000 == 0 || w&0x1f0000 == 0x1f0000 && w&0x2000 == 0x2000"},
		{"!(size == '00' && sh == '1')", "!(w&0xc00000 == 0 && w&0x2000 == 0x2000)"},
		{"!(size == '00' || sh == '1')", "!(w&0xc00000 == 0 || w&0x2000 == 0x2000)"},
		{"!IsFeatureImplemented(FEAT_SVE) || size == '00'", "w&0xc00000 == 0"},
		{"size == 'xx'", "true"},
	}
	for _, tc := range tests {
		n := compileTest(t, tc.cond)
		got := goExpr(n, 0)
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.cond, got, tc.want)
		}
		if _, err := parser.ParseExpr(got); err != nil {
			t.Errorf("%s: %q is not a Go expression: %v", tc.cond, got, err)
		}
	}
}

func TestConditionErrors(t *testing.T) {
	tests := []struct{ cond, why string }{
		{"nosuch == '00'", "unknown field"},
		{"size == '000'", "pattern"},
		{"size == '0'", "pattern"},
		{"size[3] == '0'", "slice"},
		{"size[0:1] == '00'", "slice"},
		{"UInt(size) == 0", "unknown function"},
		{"IsFeatureImplemented(size)", "IsFeatureImplemented"},
		{"MaxImplementedAnyVL() < size", "MaxImplementedAnyVL"},
		{"MaxImplementedAnyVL() foo 3", "compared with"},
		{"MaxImplementedAnyVL() < 99999999999999999999", "compared with"},
		{"MaxImplementedAnyVL()", "compared with"},
		{"size", "expected"},
		{"size IS {'00'}", "expected"},
		{"size[2:0] == '00'", "slice"},
		{"size[x:0] == '00'", "slice"},
		{"size[1:y] == '00'", "slice"},
		{"size == '00' size == '11'", "after the condition"},
		{"size == '00' &&", "expected a condition"},
		{"(size == '00'", `expected ")"`},
		{"size IN {'00'", `expected "}"`},
		{"size IN '00'", `expected "{"`},
		{"size < 3", "expected ==, != or IN"},
		{"size == 0", "expected a bit pattern"},
		{"size ?", "unexpected"},
		{"", "expected a condition"},
	}
	for _, tc := range tests {
		_, err := compileCondition(tc.cond, testFields, nil)
		if err == nil {
			t.Errorf("%q compiled; want an error about %q", tc.cond, tc.why)
		} else if !strings.Contains(err.Error(), tc.why) {
			t.Errorf("%q: error %q does not mention %q", tc.cond, err, tc.why)
		}
	}
}

const testDecode = `
if !IsFeatureImplemented(FEAT_SVE) && !IsFeatureImplemented(FEAT_SME) then
    EndOfDecode(Decode_UNDEF);
end;
if size == '00' then EndOfDecode(Decode_UNDEF); end;
let tsize : bits(3) = tszh::tszl;
let esize : integer{} = 8 << UInt(size);
let csize : integer{} = if esize < 64 then 32 else 64;
if !(tsize IN {'001', '010', '100'}) then
    EndOfDecode(Decode_UNDEF);
end;
if MaxImplementedAnyVL() < 256 then UNDEFINED; end;
let n : integer = UInt(Rm);
`

func TestRejectConditions(t *testing.T) {
	got, err := rejectConditions(testDecode)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"!IsFeatureImplemented(FEAT_SVE) && !IsFeatureImplemented(FEAT_SME)",
		"size == '00'",
		"!(tsize IN {'001', '010', '100'})",
		"MaxImplementedAnyVL() < 256",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rejectConditions = %q, want %q", got, want)
	}
	// Prose around the statements, and an expression "if", do not count.
	for _, s := range []string{"let x = if a then b else c;", ""} {
		if got, err := rejectConditions(s); err != nil || len(got) != 0 {
			t.Errorf("rejectConditions(%q) = %q, %v, want none", s, got, err)
		}
	}
}

// A reject that the pattern for "if cond then ... end;" cannot attribute to
// its condition is an error. Dropping it would make the decoder accept words
// that the architecture rejects.
func TestRejectConditionsUnsupported(t *testing.T) {
	for _, s := range []string{
		"if a == '1' then x = 1; else EndOfDecode(Decode_UNDEF); end;",
		"if a == '1' then x = 1; elsif b == '1' then UNDEFINED; end;",
		"if a == '1' then if b == '1' then UNDEFINED; end; end;",
		"EndOfDecode(Decode_UNDEF);",
	} {
		if got, err := rejectConditions(s); err == nil {
			t.Errorf("rejectConditions(%q) = %q, want an error", s, got)
		}
	}
}

func TestAliases(t *testing.T) {
	// A name bound to bits followed by anything else is not an alias.
	if got := aslAliases("let junk : bits(3) = tszh::tszl foo;\nlet bad : bits(3) = tszh::nosuch;\n", testFields); len(got) != 0 {
		t.Errorf("aslAliases accepted %v", got)
	}
	got := aslAliases(testDecode, testFields)
	if want := (aslBits{22, 20, 19}); !reflect.DeepEqual(got["tsize"], want) {
		t.Errorf("tsize = %v, want %v", got["tsize"], want)
	}
	// Only a name bound to bits of the word is an alias.
	if _, ok := got["esize"]; ok {
		t.Error("esize, an integer, is an alias")
	}
	// An alias may be built from an earlier one, or sliced.
	got = aslAliases("let a : bits(3) = tszh::tszl;\nlet b : bits(2) = a[1:0];\nlet c : bits(5) = a::sh;", testFields)
	if want := (aslBits{20, 19}); !reflect.DeepEqual(got["b"], want) {
		t.Errorf("b = %v, want %v", got["b"], want)
	}
	if want := (aslBits{22, 20, 19, 13}); !reflect.DeepEqual(got["c"], want) {
		t.Errorf("c = %v, want %v", got["c"], want)
	}
}

func TestIclassGuards(t *testing.T) {
	ic := xmlspec.IRIclass{Name: "SVE", Fields: testFields, Decode: testDecode}
	gs, st, err := iclassGuards(ic)
	if err != nil {
		t.Fatal(err)
	}
	// The two feature and vector-length checks are satisfied; the others constrain.
	if st != (aslStats{Conditions: 4, Emitted: 2, Satisfied: 2}) {
		t.Errorf("stats = %+v, want 4 conditions, 2 emitted, 2 satisfied", st)
	}
	if len(gs) != 2 || gs[0].ASL != "size == '00'" || gs[1].ASL != "!(tsize IN {'001', '010', '100'})" {
		t.Errorf("guards = %+v, want the size and tsize conditions in order", gs)
	}
	if got, want := guardExpr(gs), "w&0xc00000 == 0 || !(w&0x580000 == 0x80000 || w&0x580000 == 0x100000 || w&0x580000 == 0x400000)"; got != want {
		t.Errorf("guardExpr = %q, want %q", got, want)
	}

	// An iclass with no pseudocode has no constraints, and one with a condition
	// that cannot be compiled is an error, not a missing guard.
	if gs, _, err := iclassGuards(xmlspec.IRIclass{Fields: testFields}); gs != nil || err != nil {
		t.Errorf("no decode: got %v, %v", gs, err)
	}
	ic.Name = "BAD"
	ic.Decode = "if UInt(size) == 0 then EndOfDecode(Decode_UNDEF); end;"
	if _, _, err := iclassGuards(ic); err == nil || !strings.Contains(err.Error(), "BAD") || !strings.Contains(err.Error(), "UInt") {
		t.Errorf("an uncompilable condition: err = %v, want one naming the iclass and the condition", err)
	}
}

// The same pseudocode names a different place in the word in each iclass, so
// forms may share a case only if their tests of the word are the same too.
func TestGroupGuards(t *testing.T) {
	g := func(asl, cond string, fields map[string]xmlspec.IRField) []aslGuard {
		n, err := compileCondition(cond, fields, nil)
		if err != nil {
			t.Fatal(err)
		}
		return []aslGuard{{asl, n}}
	}
	high := map[string]xmlspec.IRField{"size": {Lo: 22, Hi: 24}}
	low := map[string]xmlspec.IRField{"size": {Lo: 17, Hi: 19}}
	perForm := [][]aslGuard{
		g("size == '00'", "size == '00'", high), // 0
		nil,                                     // 1
		g("size == '00'", "size == '00'", low),  // 2
		g("size == '00'", "size == '00'", high), // 3
		g("size == '11'", "size == '11'", high), // 4
		g("size == '00'", "size == '00'", low),  // 5
	}
	got := groupGuards(perForm)
	var forms [][]int
	for _, c := range got {
		forms = append(forms, c.Forms)
	}
	if want := [][]int{{0, 3}, {2, 5}, {4}}; !reflect.DeepEqual(forms, want) {
		t.Errorf("groups = %v, want %v", forms, want)
	}
	if e0, e2 := guardExpr(got[0].Guards), guardExpr(got[1].Guards); e0 == e2 {
		t.Errorf("size at bit 22 and at bit 17 gave the same test %q", e0)
	}
}

func TestRenderGuards(t *testing.T) {
	g := &gen{
		syms: newSymtab(),
		dIdx: map[string]int{}, aIdx: map[string]int{}, lIdx: map[string]int{},
		oIdx: map[string]int{}, reasons: map[string]int{},
	}
	g.cur = &iclassState{ir: xmlspec.IRIclass{Name: "SVE", Fields: testFields, Decode: testDecode}}
	narrow, wide := movprfx(), movprfx()
	narrow.GoOp, narrow.Asm = "AZMOVPRFX2", "MOVPRFX2  <Zd>, <Zn>"
	wide.Mask, wide.Binary = 0xfffff800, 0x0420b800
	g.form(narrow)
	g.cur = &iclassState{ir: xmlspec.IRIclass{Name: "SVE", Fields: testFields}} // no pseudocode
	g.form(wide)
	if g.skippedForms != 0 {
		t.Fatalf("form skipped: %v", g.reasons)
	}
	src, err := g.render()
	if err != nil {
		t.Fatalf("generated file does not parse: %v", err)
	}
	text := string(src)
	// The narrow mask is the more specific, so it sorts first and is the only
	// form that has a constraint.
	for _, want := range []string{
		"\t// MOVPRFX2  <Zd>, <Zn>\n\t{mask: 0xfffffc00, value: 0x0420bc00, op: 0, args: 0, reject: 1},\n",
		"func init() { sveReject = sveRejected }",
		"func sveRejected(class int, w uint32) bool {",
		"\tcase 1:\n\t\t// size == '00'\n\t\t// !(tsize IN {'001', '010', '100'})\n\t\treturn w&0xc00000 == 0 || !(w&0x580000 == 0x80000 || ",
		"1 of the 2 forms",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated file lacks %q:\n%s", want, text)
		}
	}
	// The feature and vector-length checks are decided, not emitted.
	for _, bad := range []string{"IsFeatureImplemented", "MaxImplementedAnyVL", "case 2"} {
		if strings.Contains(text, bad) {
			t.Errorf("generated file contains %q:\n%s", bad, text)
		}
	}
}
