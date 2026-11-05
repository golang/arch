// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The decoder output target. The -decoder flag emits the tables that arm64asm's
// SVE decoder reads, from the same parsed intermediate form that the encoder
// generator walks, the way ppc64map emits a decoder and an encoder from one
// specification.
//
// The tables are data-driven operand descriptors in the style of ppc64asm's
// argField. Nothing here is written per instruction: every per-instruction fact
// comes out of the parsed form, and everything that is per operand class or per
// descriptor knob lives in the hand-written runtime that reads the tables
// (arm64asm/sverun.go). The descriptors themselves are derived in
// descriptors.go, which the encoder generator uses too.
//
// The walk is Generate()'s, so that the emitted set is exactly the set that
// becomes inst_gen.go. The counts are logged so that a divergence shows.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"log"
	"os"
	"sort"
	"strings"

	"golang.org/x/arch/arm64/instgen/xmlspec"
)

var regfileName = map[dRegfile]string{
	rfNone: "rfNone", rfZ: "rfZ", rfP: "rfP", rfPN: "rfPN",
	rfR32: "rfR32", rfR64: "rfR64", rfRauto: "rfRauto", rfV: "rfV",
}

// dArg is one operand: a rendering kind plus the descriptors behind it.
type dArg struct {
	Kind  string // the runtime's rendering case, from AClass plus the register files
	Descs []int  // indexes into the deduped descriptor table
	Name  string // the ARM operand spelling, for the generated comment
}

// dForm is one instruction form.
type dForm struct {
	Mask, Value uint32
	Op          int // index into the op table
	ArgList     int // index into the deduped arg-list table
	GPR         int // 32 or 64 when the form pins its <R> width, else 0
	Asm         string
	Guards      []aslGuard // the constraints that reject a word the row matches
}

// ---------------------------------------------------------------------------
// Operand kinds. The kind is the runtime's rendering case; it comes from the
// encoder's own AClass plus the register files the descriptors name, both of
// which are IR facts.
// ---------------------------------------------------------------------------

func argKind(cls string, descs []dDesc) (string, error) {
	nreg, vecBase, vecIndex := 0, false, false
	for _, d := range descs {
		if d.Role == roleReg {
			if nreg == 0 {
				vecBase = d.Regfile == rfZ
			} else {
				vecIndex = d.Regfile == rfZ
			}
			nreg++
		}
	}
	switch cls {
	case "AC_ARNG":
		return "kArng", nil
	case "AC_ZREG", "AC_PREG", "AC_VREG", "AC_SPZGREG":
		return "kReg", nil
	case "AC_PREGZM":
		return "kPredQual", nil
	case "AC_IMM":
		return "kImm", nil
	case "AC_ARNGIDX":
		return "kArngIdx", nil
	case "AC_ZREGIDX", "AC_PREGIDX":
		return "kRegIdx", nil
	case "AC_REGLIST1", "AC_REGLIST2", "AC_REGLIST3", "AC_REGLIST4":
		return "kList", nil
	case "AC_REGLIST_RANGE":
		return "kListRange", nil
	case "AC_SPECIAL":
		return "kSpecial", nil
	case "AC_MEMOFFMULVL":
		return "kMemMulVL", nil
	case "AC_MEMOFF":
		if vecBase {
			return "kMemVecBase", nil
		}
		return "kMemOff", nil
	case "AC_MEMEXT":
		if vecBase || vecIndex {
			return "kMemVecIndex", nil
		}
		return "kMemExt", nil
	}
	return "", fmt.Errorf("unknown operand class %s", cls)
}

// ---------------------------------------------------------------------------
// Emission
// ---------------------------------------------------------------------------

type gen struct {
	syms  *symtab
	descs []dDesc
	dIdx  map[string]int
	args  []dArg
	aIdx  map[string]int
	lists [][]int
	lIdx  map[string]int
	ops   []opRec
	oIdx  map[string]int
	forms []dForm

	// sample names the tables sveSampleOpARM and so on rather than sveOpARM, so
	// that they can sit beside the real ones in an arm64asm test.
	sample bool

	// cur is the iclass whose encodings are being walked, which is where the
	// reject constraints of their forms come from. It is nil outside a walk.
	cur *iclassState
	asl aslStats

	skippedForms int
	reasons      map[string]int
	noRow        int // encodings left out because an operand has a class that the encoder does not handle either
	elems        int
}

type opRec struct{ arm, goName string }

// iclassState is an iclass and, once a form of it needs them, its reject
// constraints. An iclass has many encodings, and they all share one.
type iclassState struct {
	ir     xmlspec.IRIclass
	guards []aslGuard
	done   bool
}

// guards returns the reject constraints of the current iclass.
func (g *gen) guards() []aslGuard {
	if g.cur == nil {
		return nil
	}
	if !g.cur.done {
		gs, st, err := iclassGuards(g.cur.ir)
		if err != nil {
			log.Fatalf("decoder: %v", err)
		}
		g.cur.guards, g.cur.done = gs, true
		g.asl.Conditions += st.Conditions
		g.asl.Emitted += st.Emitted
		g.asl.Satisfied += st.Satisfied
	}
	return g.cur.guards
}

func (g *gen) descIdx(d dDesc) int {
	k := fmt.Sprintf("%v", d)
	if i, ok := g.dIdx[k]; ok {
		return i
	}
	i := len(g.descs)
	g.dIdx[k] = i
	g.descs = append(g.descs, d)
	return i
}

func (g *gen) argIdx(a dArg) int {
	k := a.Kind + fmt.Sprintf("%v", a.Descs)
	if i, ok := g.aIdx[k]; ok {
		return i
	}
	i := len(g.args)
	g.aIdx[k] = i
	g.args = append(g.args, a)
	return i
}

func (g *gen) listIdx(l []int) int {
	k := fmt.Sprintf("%v", l)
	if i, ok := g.lIdx[k]; ok {
		return i
	}
	i := len(g.lists)
	g.lIdx[k] = i
	g.lists = append(g.lists, l)
	return i
}

func (g *gen) opIdx(arm, goName string) int {
	k := arm + "/" + goName
	if i, ok := g.oIdx[k]; ok {
		return i
	}
	i := len(g.ops)
	g.oIdx[k] = i
	g.ops = append(g.ops, opRec{arm, goName})
	return i
}

func (g *gen) fail(why string) {
	g.skippedForms++
	g.reasons[why]++
}

// err reports the forms that could not be described. That is an error: a form
// that is dropped is an instruction that the decoder does not know, and nothing
// at run time says so.
func (g *gen) err() error {
	if g.skippedForms == 0 {
		return nil
	}
	var ks []string
	for k := range g.reasons {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	msg := fmt.Sprintf("%d forms cannot be described:", g.skippedForms)
	for _, k := range ks {
		msg += fmt.Sprintf("\n\t%d: %s", g.reasons[k], k)
	}
	return errors.New(msg)
}

// GenerateDecoder emits the decoder tables. The walk is Generate()'s, so the
// emitted set matches inst_gen.go's rows.
func GenerateDecoder(insts []*xmlspec.InstructionParsed, out string) {
	generateDecoder(insts, out, false)
}

// generateDecoder is GenerateDecoder, writing the sample tables' names if sample
// is set.
func generateDecoder(insts []*xmlspec.InstructionParsed, out string, sample bool) {
	g := &gen{
		sample: sample,
		syms:   newSymtab(),
		dIdx:   map[string]int{}, aIdx: map[string]int{}, lIdx: map[string]int{},
		oIdx: map[string]int{}, reasons: map[string]int{},
	}
	kept := 0

	for _, inst := range insts {
		if inst == nil {
			continue
		}
		isAlias := false
		for _, doc := range inst.DocVars {
			if doc.Key == "alias_mnemonic" {
				isAlias = true
			}
		}
		if isAlias {
			continue
		}
		for _, iclass := range inst.Classes.Iclass {
			if !iclass.RegDiagram.Parsed {
				continue
			}
			g.cur = &iclassState{ir: xmlspec.IRForIclass(iclass)}
			for _, encodingRaw := range iclass.Encodings {
				if !encodingRaw.Parsed || encodingRaw.Alias {
					continue
				}
				for _, enc := range xmlspec.SplitInstByRegWidth(encodingRaw) {
					if len(enc.Operands) > 1 {
						skip := false
						for _, op := range enc.Operands[1:] {
							if _, ok := operandTypeOrders[op.Typ]; !ok {
								skip = true
							}
						}
						if skip {
							g.noRow++
							continue
						}
					}
					kept++
					g.form(xmlspec.IRFor(enc))
				}
			}
		}
	}

	g.cur = nil

	log.Printf("decoder: walked %d encodings, emitted %d forms (%d skipped), %d operand elements; %d encodings have no row",
		kept, len(g.forms), g.skippedForms, g.elems, g.noRow)
	if len(g.reasons) > 0 {
		var ks []string
		for k := range g.reasons {
			ks = append(ks, k)
		}
		sort.Slice(ks, func(i, j int) bool {
			if g.reasons[ks[i]] != g.reasons[ks[j]] {
				return g.reasons[ks[i]] > g.reasons[ks[j]]
			}
			return ks[i] < ks[j]
		})
		for _, k := range ks {
			log.Printf("  %5d forms: %s", g.reasons[k], k)
		}
	}
	log.Printf("decoder: %d reject conditions in the decode pseudocode: %d constrain the word, %d are feature or vector-length checks that the assumptions satisfy",
		g.asl.Conditions, g.asl.Emitted, g.asl.Satisfied)
	log.Printf("decoder: %d descriptors, %d operands, %d arg lists, %d ops, %d symbols",
		len(g.descs), len(g.args), len(g.lists), len(g.ops), len(g.syms.all))

	if err := g.err(); err != nil {
		log.Fatal(err)
	}
	g.write(out)
}

// armOperands splits an ARM asm template into operands at brace/bracket depth 0.
func armOperands(asm string) []string {
	i := strings.IndexAny(asm, " \t")
	if i < 0 {
		return nil
	}
	rest := asm[i:]
	var out []string
	depth, start := 0, 0
	for j, c := range rest {
		switch c {
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(rest[start:j]))
				start = j + 1
			}
		}
	}
	if s := strings.TrimSpace(rest[start:]); s != "" {
		out = append(out, s)
	}
	return out
}

func (g *gen) form(ir xmlspec.IREncoding) {
	armMnem := strings.Fields(ir.Asm)[0]
	store := strings.HasPrefix(armMnem, "ST")

	var list []int
	for _, op := range ir.Operands[1:] {
		var descs []int
		var dd []dDesc
		bad := ""
		for _, el := range op.Elems {
			g.elems++
			d, err := derive(el, g.syms, op.Typ)
			if err != nil {
				bad = op.Typ + ": " + err.Error()
				break
			}
			// Only the encoder needs to know that an absent register means
			// register 31. A word that holds 31 there is register 31.
			d.ZR = false
			// An element that contributes nothing carries no descriptor at all;
			// the check it performs is entailed by a sibling.
			if d.Role == roleSkip {
				continue
			}
			dd = append(dd, d)
			descs = append(descs, g.descIdx(d))
		}
		if bad != "" {
			g.fail(bad)
			return
		}
		kind, err := argKind(op.Typ, dd)
		if err != nil {
			g.fail(err.Error())
			return
		}
		list = append(list, g.argIdx(dArg{Kind: kind, Descs: descs, Name: op.Name}))
	}

	// Inst.Args holds ARM manual order, which is arm64asm's documented
	// convention. instgen stores Go order. The two are related by one rule that
	// needs no per-form data: reversed, except for stores. Check it here rather
	// than trust it.
	armOrder := make([]int, len(list))
	copy(armOrder, list)
	if !store {
		for i, j := 0, len(armOrder)-1; i < j; i, j = i+1, j-1 {
			armOrder[i], armOrder[j] = armOrder[j], armOrder[i]
		}
	}
	// Check the rule against this form's own operand names. Two things make a
	// naive check under-report: the deduplicated arg table keeps only the first
	// name that produced each entry, so it must not be consulted here, and
	// instgen's stored spelling differs from the asm template's in whitespace.
	names := make([]string, 0, len(ir.Operands)-1)
	for _, op := range ir.Operands[1:] {
		names = append(names, strings.Join(strings.Fields(op.Name), " "))
	}
	if !store {
		for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
			names[i], names[j] = names[j], names[i]
		}
	}
	want := armOperands(ir.Asm)
	if len(want) != len(names) {
		g.fail("operand count differs from the assembly template")
		return
	}
	for i, n := range names {
		if n != strings.Join(strings.Fields(want[i]), " ") {
			g.fail("operand order differs from the assembly template")
			return
		}
	}

	if len(armOrder) > 5 {
		g.fail("more than 5 operands: Inst.Args is [5]Arg")
		return
	}
	op := g.opIdx(armMnem, ir.GoOp[1:])
	al := g.listIdx(armOrder)
	// ARM writes "<R><n>" for an operand whose width is pinned elsewhere in the
	// encoding. For WHILE* and CTERM* that elsewhere is the one-bit register-width
	// split instgen already performs, so the width is a property of the row: value
	// 0 is the 32-bit Go op, value 1 the 64-bit one. INDEX writes "<R><m>" too but
	// takes its width from the element size, and has no split -- hence 0 here and
	// the runtime falling back to the arrangement.
	gpr := 0
	if ir.SplitHi-ir.SplitLo == 1 && len(ir.SplitValues) == 1 && g.hasAuto(armOrder) {
		if ir.SplitValues[0] == 0 {
			gpr = 32
		} else {
			gpr = 64
		}
	}
	for _, r := range splitRows(ir) {
		g.forms = append(g.forms, dForm{
			Mask: r.mask, Value: r.value, Op: op, ArgList: al,
			GPR: gpr, Asm: ir.Asm, Guards: g.guards(),
		})
	}
}

// hasAuto reports whether any operand in the list leaves its GPR width unstated.
func (g *gen) hasAuto(list []int) bool {
	for _, ai := range list {
		for _, di := range g.args[ai].Descs {
			if g.descs[di].Regfile == rfRauto {
				return true
			}
		}
	}
	return false
}

// splitRows expands one encoding into its dispatch rows.
//
// SplitInstByRegWidth turns one XML encoding into several Go ops by pinning a
// field -- the element size, or the 32-versus-64-bit register width -- but it
// records the pinned value in Binary without widening the mask, and one Go op can
// accept a *set* of field values ("GNU prefers 00 for W", so the W op takes
// size in {0,1,2} while Binary pins 0). So the mask has to be widened here, once
// per accepted value.
//
// Skipping this is invisible in a spot check, because the first form of each
// split family decodes correctly and only its siblings are shadowed.
func splitRows(ir xmlspec.IREncoding) []struct{ mask, value uint32 } {
	type row = struct{ mask, value uint32 }
	if ir.SplitHi == 0 || len(ir.SplitValues) == 0 {
		return []row{{ir.Mask, ir.Binary}}
	}
	lo, hi := ir.SplitLo, ir.SplitHi
	field := uint32(1<<uint(hi-lo)-1) << uint(lo)
	out := make([]row, 0, len(ir.SplitValues))
	for _, v := range ir.SplitValues {
		out = append(out, row{ir.Mask | field, (ir.Binary &^ field) | (v << uint(lo))})
	}
	return out
}

// render returns the text of the generated file, formatted.
func (g *gen) render() ([]byte, error) {
	var b strings.Builder
	p := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }

	pre := "sve"
	if g.sample {
		// The tables of instgen's own test data, for an arm64asm test to install.
		pre = "sveSample"
		p("// Code generated by 'go test golang.org/x/arch/arm64/instgen -run=TestSampleDecoderTables -update-sample'. DO NOT EDIT.\n")
	} else {
		p("// Code generated by 'go run golang.org/x/arch/arm64/instgen -decoder=sve_gen.go'. DO NOT EDIT.\n")
		p("//\n")
		p("// Run that command in this directory to regenerate the file.\n")
	}
	p("//\n")
	p("// SVE decoder tables: %d forms over %d mnemonics, built from %d deduplicated\n",
		len(g.forms), len(g.ops), len(g.descs))
	p("// operand descriptors and %d operand shapes. Nothing here is authored per\n", len(g.args))
	p("// instruction; sverun.go is the hand-written runtime that reads it.\n\n")
	p("package arm64asm\n\n")

	// The comment on each table says what it holds, for the runtime that reads it.
	p("// %sOpARM is the mnemonic of each op as the ARM manual spells it, which\n", pre)
	p("// is what [Op.String] returns and what [GNUSyntax] prints.\n")
	p("var %sOpARM = []string{\n", pre)
	for _, o := range g.ops {
		p("\t%q,\n", o.arm)
	}
	p("}\n\n")
	p("// %sOpGo is the mnemonic of each op as the Go assembler spells it. It\n", pre)
	p("// adds a register file prefix and, for %d ops, a pinned arrangement, so\n", g.pinnedSuffixCount())
	p("// it cannot be derived from [%sOpARM].\n", pre)
	p("var %sOpGo = []string{\n", pre)
	for _, o := range g.ops {
		p("\t%q,\n", o.goName)
	}
	p("}\n\n")

	p("// %sSyms are the spellings that ARM's value tables name, interned. A\n", pre)
	p("// descriptor refers to one by index, either as a pinned value or as the\n")
	p("// result of looking a field up in its table.\n")
	p("var %sSyms = []string{\n", pre)
	for _, s := range g.syms.all {
		p("\t%q,\n", s)
	}
	p("}\n\n")

	p("// %sDescs are the parts of the operands: a register, an arrangement, a\n", pre)
	p("// qualifier, an immediate. See [sveDesc] for the knobs.\n")
	p("var %sDescs = []sveDesc{\n", pre)
	for i, d := range g.descs {
		p("\t/*%4d*/ {role: %s", i, roleName[d.Role])
		if len(d.Fields) > 0 {
			p(", fields: []sveField{")
			for j, f := range d.Fields {
				if j > 0 {
					p(", ")
				}
				p("{%d, %d}", f.Lo, f.Width)
			}
			p("}")
		}
		if d.Signed {
			p(", signed: true")
		}
		if d.HasRng {
			p(", hasRange: true, min: %d, max: %d", d.Min, d.Max)
		}
		if d.Stride != 1 {
			p(", stride: %d", d.Stride)
		}
		if d.Bias != 0 {
			p(", bias: %d", d.Bias)
		}
		if d.Modulo != 0 {
			p(", modulo: %d", d.Modulo)
		}
		if d.Regfile != rfNone {
			p(", regfile: %s", regfileName[d.Regfile])
		}
		if d.SP {
			p(", sp: true")
		}
		if d.AltBit >= 0 {
			p(", altBit: %d, altShift: %d", d.AltBit, d.AltSh)
		}
		if d.FP8 {
			p(", fp8: true")
		}
		if d.TszMode != "" {
			p(", tsz: %s", d.TszMode)
		}
		if d.Bitmask {
			p(", bitmask: true")
		}
		if d.VWidth != 0 {
			p(", vwidth: %d", d.VWidth)
		}
		if d.Invert != 0 {
			p(", invert: %d", d.Invert)
		}
		if d.Fixed >= 0 {
			p(", fixed: %d", d.Fixed)
		} else {
			p(", fixed: -1")
		}
		if len(d.Table) > 0 {
			p(", table: []sveRow{")
			for j, r := range d.Table {
				if j > 0 {
					p(", ")
				}
				p("{%#x, %#x, %d}", r.Mask, r.Value, r.Sym)
			}
			p("}")
		}
		p("},\n")
	}
	p("}\n\n")

	p("// %sArgs are the operands. Each has a kind, which chooses the [Arg] it\n", pre)
	p("// becomes, and the descriptors that supply its parts, as indexes into\n")
	p("// [%sDescs].\n", pre)
	p("var %sArgs = []sveArg{\n", pre)
	for i, a := range g.args {
		p("\t/*%4d*/ {kind: %s, descs: []uint16{", i, a.Kind)
		for j, di := range a.Descs {
			if j > 0 {
				p(", ")
			}
			p("%d", di)
		}
		p("}}, // %s\n", a.Name)
	}
	p("}\n\n")

	p("// %sArgLists names the operands of each form, in ARM manual order, as\n", pre)
	p("// indexes into [%sArgs].\n", pre)
	p("var %sArgLists = [][]uint16{\n", pre)
	for i, l := range g.lists {
		p("\t/*%4d*/ {", i)
		for j, ai := range l {
			if j > 0 {
				p(", ")
			}
			p("%d", ai)
		}
		p("},\n")
	}
	p("}\n\n")

	// Rows are emitted most-specific-mask-first, so the linear scan needs no
	// further disambiguation. Two rows that match a common word must not have
	// as many mask bits, or their order is the XML's; arm64asm's
	// TestSVEFormOrder checks that.
	sort.SliceStable(g.forms, func(i, j int) bool {
		return popcount(g.forms[i].Mask) > popcount(g.forms[j].Mask)
	})
	p("// %sForms has one row per encoding, most specific mask first, so the\n", pre)
	p("// first row whose [sveForm.mask] and [sveForm.value] match and whose\n")
	p("// operands all resolve wins. [sveForm.op] indexes [%sOpARM] and\n", pre)
	p("// [%sOpGo], [sveForm.args] indexes [%sArgLists], and [sveForm.gpr] is\n", pre, pre)
	p("// 32 or 64 when the form fixes the width of its <R> register.\n")
	p("var %sForms = []sveForm{\n", pre)
	// Forms that the decode pseudocode constrains in the same way share a class,
	// which the form carries in its reject field.
	perForm := make([][]aslGuard, len(g.forms))
	for i, f := range g.forms {
		perForm[i] = f.Guards
	}
	cases := groupGuards(perForm)
	rejectClass := make([]int, len(g.forms))
	for k, c := range cases {
		for _, i := range c.Forms {
			rejectClass[i] = k + 1
		}
	}
	for i, f := range g.forms {
		gpr := ""
		if f.GPR != 0 {
			gpr = fmt.Sprintf(", gpr: %d", f.GPR)
		}
		rej := ""
		if c := rejectClass[i]; c != 0 {
			rej = fmt.Sprintf(", reject: %d", c)
		}
		// The template goes on a line of its own: as a trailing comment, gofmt
		// would align it across the block, so a field added to some rows would
		// move the comment on every row.
		p("\t// %s\n", f.Asm)
		p("\t{mask: %#08x, value: %#08x, op: %d, args: %d%s%s},\n",
			f.Mask, f.Value, f.Op, f.ArgList, gpr, rej)
	}
	p("}\n\n")

	lo, hi, used := gateBitmap(g.forms)
	p("// %sGateLo and %sGateHi are a bitmap of the values of bits 31:25 that any\n", pre, pre)
	p("// form can match: bit n of the 128 is set when some form's mask and value allow\n")
	p("// n there. The forms use %d of the 128 values. sveDecode skips the scan for a\n", used)
	p("// word whose value is not in the set, which no form could have matched.\n")
	p("var (\n\t%sGateLo uint64 = %#016x\n\t%sGateHi uint64 = %#016x\n)\n", pre, lo, pre, hi)

	if !g.sample {
		p("\nfunc init() { sveReject = sveRejected }\n\n")
	}
	p("// %sRejected reports whether the architecture declares w to be an invalid\n", pre)
	p("// instance of a form whose reject field is class. The classes are the\n")
	p("// constraints that the decode pseudocode of the ARM XML states, %d of them over\n", len(cases))
	p("// %d of the %d forms, each with the pseudocode it comes from. Every feature is\n", len(g.forms)-countZero(rejectClass), len(g.forms))
	p("// taken to be implemented and the maximum vector length to be %d bits, so the\n", aslMaxVL)
	p("// conditions on those never reject: see instgen's asl.go.\n")
	p("func %sRejected(class int, w uint32) bool {\n\tswitch class {\n", pre)
	for k, c := range cases {
		p("\tcase %d:\n", k+1)
		for _, gd := range c.Guards {
			p("\t\t// %s\n", gd.ASL)
		}
		p("\t\treturn %s\n", guardExpr(c.Guards))
	}
	p("\t}\n\treturn false\n}\n")

	return format.Source([]byte(b.String()))
}

// gateBitmap returns, as two 64-bit words, the set of values of bits 31:25 that
// at least one form can match, and how many values that is. A form that leaves
// some of those bits free contributes every value its fixed bits allow, so the
// set is exact whether or not the forms pin the field.
func gateBitmap(forms []dForm) (lo, hi uint64, n int) {
	for v := uint32(0); v < 128; v++ {
		for _, f := range forms {
			if v&(f.Mask>>25) == f.Value>>25&(f.Mask>>25) {
				if v < 64 {
					lo |= 1 << v
				} else {
					hi |= 1 << (v - 64)
				}
				n++
				break
			}
		}
	}
	return lo, hi, n
}

func (g *gen) write(path string) {
	text, err := g.render()
	if err != nil {
		log.Fatalf("decoder: formatting the tables: %v", err)
	}
	if err := os.WriteFile(path, text, 0644); err != nil {
		log.Fatalf("decoder: %v", err)
	}
	log.Printf("decoder: wrote %s (%d lines)", path, bytes.Count(text, []byte("\n")))
}

func (g *gen) pinnedSuffixCount() int {
	n := 0
	for _, o := range g.ops {
		if o.goName[1:] != o.arm {
			n++
		}
	}
	return n
}

func countZero(xs []int) int {
	n := 0
	for _, x := range xs {
		if x == 0 {
			n++
		}
	}
	return n
}

func popcount(x uint32) int {
	n := 0
	for ; x != 0; x &= x - 1 {
		n++
	}
	return n
}
