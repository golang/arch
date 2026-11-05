// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Operand descriptors. A descriptor says how one element of an instruction's
// operand maps between the bits of the instruction word and a value: which bits,
// whether the value is signed, its range, a stride and a bias, a register file,
// a value table, and a few readings that depend on the element size.
//
// They are derived here, once, from the ARM XML's own description text, which
// is what instgen already keys its encoder functions on. The encoder generator
// and the decoder generator both render them, so the two directions cannot
// disagree about a field.

package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/arch/arm64/instgen/xmlspec"
)

// dRole says what an element contributes to its operand. The runtime switches on
// this, one case per role, not per instruction.
type dRole int

const (
	roleSkip   dRole = iota // a pure validity check that adds no spelling
	roleReg                 // a register number
	roleArng                // an element-size arrangement: .B .H .S .D .Q, or a NEON one
	roleQual                // a predication qualifier: .M .Z
	roleIndex               // a lane or segment index
	roleImm                 // an immediate
	roleOff                 // a memory offset
	roleMod                 // an index extend/shift specifier: LSL UXTW SXTW
	roleAmount              // an index shift amount
	roleVL                  // a VLx group size
	rolePrfop               // a prefetch operation
)

var roleName = map[dRole]string{
	roleSkip: "roleSkip", roleReg: "roleReg", roleArng: "roleArng",
	roleQual: "roleQual", roleIndex: "roleIndex", roleImm: "roleImm",
	roleOff: "roleOff", roleMod: "roleMod", roleAmount: "roleAmount",
	roleVL: "roleVL", rolePrfop: "rolePrfop",
}

// dRegfile is the register file a roleReg element names.
type dRegfile int

const (
	rfNone dRegfile = iota
	rfZ
	rfP
	rfPN
	rfR32
	rfR64
	rfRauto // width follows the element size: W below 64 bits, X at 64
	rfV
)

// dField is one contiguous piece of an encoded value.
type dField struct{ Lo, Width int }

// dRow is one value-table row. Don't-care bits in the ARM table ("x1", "1x")
// become a mask, so a row matches when field&Mask == Value. Sym indexes the
// generated symbol table.
type dRow struct {
	Mask, Value uint32
	Sym         int
}

// dDesc is one element's descriptor: which bits, and how the field value maps to
// a spelling. Fixed >= 0 means the element contributes a constant symbol and
// occupies no bits, which is how the encoder's pure checks read in reverse.
type dDesc struct {
	Role    dRole
	Fields  []dField // low-to-high significance
	Signed  bool
	HasRng  bool
	Min     int64
	Max     int64
	Stride  int64
	Bias    int64
	Modulo  int64
	Regfile dRegfile
	SP      bool // "or stack pointer": field 31 spells SP/RSP
	AltBit  int  // the "LSL #8" alternative's bit, or -1
	AltSh   uint
	Table   []dRow
	Fixed   int    // a pinned symbol index, or -1
	FP8     bool   // the field is an 8-bit floating-point immediate
	TszMode string // how the element size enters the value, or ""
	Bitmask bool   // the field is a DecodeBitMasks logical immediate
	VWidth  int    // an explicitly stated SIMD&FP register width, or 0
	Invert  int64  // value = Invert - field, when the field holds the distance from a constant; 0 means no
	ZR      bool   // an absent operand stands for register 31, as "defaulting to XZR" says
}

func (d *dDesc) width() int {
	n := 0
	for _, f := range d.Fields {
		n += f.Width
	}
	return n
}

// ---------------------------------------------------------------------------
// Prose derivation. Everything below reads the ARM XML's own description text,
// which is what instgen already keys its encoder implementations on. If ARM
// rewords a sentence that a pattern here matches, an operand's value can change
// without an error; the pinned encodings in arm64asm's testdata catch that.
// ---------------------------------------------------------------------------

var (
	reRange     = regexp.MustCompile(`(?m)^(\w+): \[(\d+):(\d+)\)$`)
	reComposite = regexp.MustCompile(`"(\w+(?::\w+)+)"`)
	reOneField  = regexp.MustCompile(`"(\w+)" field`)
	reBits      = regexp.MustCompile(`^[01x]+$`)
	reInRange   = regexp.MustCompile(`in the range (-?\d+) to (-?\d+)`)
	reMultiple  = regexp.MustCompile(`a multiple of (\d+)`)
	reTimes     = regexp.MustCompile(`encoded as "\w+" times (\d+)`)
	rePlus      = regexp.MustCompile(`plus (\d+)`)
	reOrdinal   = regexp.MustCompile(`\b(first|second|third|fourth)\b`)
	rePNRange   = regexp.MustCompile(`PN(\d+)-PN(\d+)`)
	reZRange    = regexp.MustCompile(`\bZ(\d+)-Z(\d+)\b`)
	rePRange    = regexp.MustCompile(`\bP(\d+)-P(\d+)\b`)
	rePNDigits  = regexp.MustCompile(`predicate-as-counter|\bPN\d`)
	// "unsigned immediate" contains "signed immediate" as a substring, so this
	// has to be a word-boundary match. A plain substring test makes every
	// unsigned 8-bit immediate signed, which turns 255 into -1 and then, through
	// the optional LSL #8, 65280 into -256.
	reSigned     = regexp.MustCompile(`\bsigned immediate\b`)
	reNamedWidth = regexp.MustCompile(`\b(\d+)-bit name\b`)
	reCheckArng  = regexp.MustCompile(`^Check this is an? ([0-9A-Z]+) arrangement$`)
	reCheckMod   = regexp.MustCompile(`^Check this is mod and is (\w+)$`)
	reCheckAmt   = regexp.MustCompile(`^Check this is (?:mod )?amount and is #?(\d+)$`)

	reShiftNarrow = regexp.MustCompile(`is the immediate shift amount, in the range 1 to (\d+), encoded in the "imm\d+" field`)
	reDefaultsXZR = regexp.MustCompile(`, defaulting to XZR,`)
)

// ranges pulls the machine-readable "bit range mappings:" block instgen appends
// to every description. This block is why the IR is usable as a decoder source
// at all: without it the field would have to be recovered by matching the
// prose's field name against the regdiagram.
func ranges(key string) map[string]dField {
	out := map[string]dField{}
	for _, m := range reRange.FindAllStringSubmatch(key, -1) {
		lo, _ := strconv.Atoi(m[2])
		hi, _ := strconv.Atoi(m[3])
		out[m[1]] = dField{lo, hi - lo}
	}
	return out
}

// symtab interns the spellings the value tables name. The strings come straight
// out of the XML, so nothing here is authored per instruction.
type symtab struct {
	idx map[string]int
	all []string
}

func newSymtab() *symtab { return &symtab{idx: map[string]int{}} }

func (s *symtab) get(v string) int {
	if i, ok := s.idx[v]; ok {
		return i
	}
	i := len(s.all)
	s.idx[v] = i
	s.all = append(s.all, v)
	return i
}

// parseTable reads a bits->symbol block. The block carries a header naming the
// fields it is keyed on, so it supplies the field order as well as the rows;
// that matters for the two-field cross products, whose "encoded_in" is
// "(tszh :: tszl)" and whose prose has no quoted composite to parse.
//
// ARM writes the header high-part-first and the row bits in the same order, so
// the key is assembled left to right, shifting by each field's width.
//
// A RESERVED row is dropped rather than recorded: that field value is not
// architecturally valid, and reporting it as a register arrangement would be
// worse than declining. Dropping it is what makes the tables reject such a word.
func parseTable(key string, rs map[string]dField, syms *symtab) ([]dField, []dRow, bool) {
	lines := strings.Split(key, "\n")
	var hdr []dField
	for _, ln := range lines {
		cols := strings.Split(ln, "\t")
		if len(cols) < 2 {
			continue
		}
		fs := make([]dField, 0, len(cols)-1)
		for _, c := range cols[:len(cols)-1] {
			f, ok := headerField(c, rs)
			if !ok {
				fs = nil
				break
			}
			fs = append(fs, f)
		}
		if len(fs) > 0 {
			hdr = fs
			break
		}
	}
	if len(hdr) == 0 {
		return nil, nil, false
	}
	// Fields low-to-high, which is the reverse of the header's order.
	var fields []dField
	for i := len(hdr) - 1; i >= 0; i-- {
		fields = append(fields, hdr[i])
	}

	var rows []dRow
	for _, ln := range lines {
		cols := strings.Split(ln, "\t")
		if len(cols) != len(hdr)+1 {
			continue
		}
		ok := true
		for i, c := range cols[:len(hdr)] {
			if !reBits.MatchString(c) || len(c) != hdr[i].Width {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		sym := strings.TrimSpace(cols[len(hdr)])
		if sym == "RESERVED" || sym == "UNDEFINED" {
			continue
		}
		var mask, val uint32
		for i, c := range cols[:len(hdr)] {
			w := hdr[i].Width
			mask <<= uint(w)
			val <<= uint(w)
			for j := 0; j < len(c); j++ {
				bit := uint32(1) << uint(w-1-j)
				switch c[j] {
				case '0':
					mask |= bit
				case '1':
					mask |= bit
					val |= bit
				}
			}
		}
		rows = append(rows, dRow{mask, val, syms.get(sym)})
	}
	if len(rows) == 0 {
		return nil, nil, false
	}
	return fields, rows, true
}

// headerField resolves one value-table header column to a bit field. ARM writes
// most of them as a bare field name, but some tables are keyed on a *slice* of a
// field -- "size[0]" for the narrowing pairs -- and reading that as the whole
// field makes the table twice as wide as its rows, so it silently stops parsing
// and the operand falls through to being treated as an immediate.
func headerField(c string, rs map[string]dField) (dField, bool) {
	if f, ok := rs[c]; ok {
		return f, true
	}
	m := reSlice.FindStringSubmatch(c)
	if m == nil {
		return dField{}, false
	}
	f, ok := rs[m[1]]
	if !ok {
		return dField{}, false
	}
	hi, _ := strconv.Atoi(m[2])
	lo := hi
	if m[3] != "" {
		lo, _ = strconv.Atoi(m[3])
	}
	if lo > hi || hi >= f.Width {
		return dField{}, false
	}
	return dField{f.Lo + lo, hi - lo + 1}, true
}

var reSlice = regexp.MustCompile(`^(\w+)\[(\d+)(?::(\d+))?\]$`)

// arngSyms are the arrangements that arm64asm's sveArrangement knows: the
// element sizes, and the NEON arrangements of the quadword reductions.
var arngSyms = map[string]bool{
	"B": true, "H": true, "S": true, "D": true, "Q": true,
	"16B": true, "8H": true, "4S": true, "2D": true,
}

// tableRole says what a value-table-bearing element contributes, from the symbol
// ARM's own asm template names it by.
//
// The symbol is also the gate on whether a table is read at all: the composite
// "#<imm>{, <shift>}" operand carries an "sh -> LSL #0 / LSL #8" block that looks
// like a value table but is the optional-shift knob, and reading it as a table
// silently turns every shifted immediate into a shift specifier.
func tableRole(sym string) (dRole, bool) {
	switch strings.Trim(sym, "<>") {
	case "T", "Tb", "Ta":
		return roleArng, true
	case "mod":
		return roleMod, true
	case "amount":
		return roleAmount, true
	case "vl":
		return roleVL, true
	case "prfop":
		return rolePrfop, true
	case "ZM":
		return roleQual, true
	case "const":
		return roleImm, true
	}
	return roleSkip, false
}

// checkDomain validates that every symbol in a table belongs to its role's
// domain. Every domain is closed and small, and a symbol outside one is an error
// rather than a default, so a new ARM table shows up as a generator diagnostic
// instead of as a wrong disassembly.
func checkDomain(role dRole, syms *symtab, rows []dRow) error {
	for _, r := range rows {
		s := syms.all[r.Sym]
		if s == "[absent]" {
			continue
		}
		ok := false
		switch role {
		case roleArng:
			ok = arngSyms[s]
		case roleQual:
			ok = s == "M" || s == "Z"
		case roleMod:
			ok = s == "LSL" || s == "UXTW" || s == "SXTW"
		case roleVL:
			ok = strings.HasPrefix(s, "VLx")
		case rolePrfop:
			ok = strings.HasPrefix(s, "PLD") || strings.HasPrefix(s, "PST") || s == "#uimm4"
		case roleAmount:
			if strings.HasPrefix(s, "#") {
				_, err := strconv.Atoi(s[1:])
				ok = err == nil
			}
		case roleImm:
			if strings.HasPrefix(s, "#") {
				if strings.Contains(s, ".") {
					_, err := strconv.ParseFloat(s[1:], 64)
					ok = err == nil
				} else {
					_, err := strconv.Atoi(s[1:])
					ok = err == nil
				}
			}
		}
		if !ok {
			return fmt.Errorf("symbol %q outside the %s domain", s, roleName[role])
		}
	}
	return nil
}

// deriveCheck handles the elements that occupy no bits. instgen writes these as
// short sentinel descriptions rather than XML prose, and each one either pins a
// spelling the decoder would otherwise infer, or asserts something a sibling
// element already entails.
func deriveCheck(prose string, syms *symtab) (dDesc, bool) {
	d := dDesc{Role: roleSkip, AltBit: -1, Stride: 1, Fixed: -1}
	p := strings.TrimSpace(prose)
	if m := reCheckArng.FindStringSubmatch(p); m != nil {
		d.Role, d.Fixed = roleArng, syms.get(m[1])
		return d, true
	}
	if m := reCheckMod.FindStringSubmatch(p); m != nil {
		d.Role, d.Fixed = roleMod, syms.get(m[1])
		return d, true
	}
	if m := reCheckAmt.FindStringSubmatch(p); m != nil {
		d.Role, d.Fixed = roleAmount, syms.get("#"+m[1])
		return d, true
	}
	switch {
	case p == "Check this is a merging predication":
		d.Role, d.Fixed = roleQual, syms.get("M")
	case p == "Check this is a zeroing predication":
		d.Role, d.Fixed = roleQual, syms.get("Z")
	case strings.Contains(p, "0.0"):
		d.Role, d.Fixed = roleImm, syms.get("#0.0")
	case p == "No-op check, returns true",
		strings.HasPrefix(p, "Check this is a 32-bit scalar register"),
		strings.HasPrefix(p, "Check this is a 64-bit scalar register"),
		strings.HasPrefix(p, "Check this is not a mod"),
		strings.HasPrefix(p, "Check that there is no modifier"),
		strings.HasPrefix(p, "Check this has no mod"),
		strings.HasPrefix(p, "Check this has no amount"),
		strings.HasPrefix(p, "Check this is no amount"):
		// Nothing to spell. The width checks are entailed by the register file a
		// sibling element names; the no-mod/no-amount checks assert an absence.
		// "Check that there is no modifier amount" is matched by the same prefix.
	default:
		return d, false
	}
	return d, true
}

// derive turns one IR element into a descriptor.
func derive(el xmlspec.IRElement, syms *symtab, cls string) (dDesc, error) {
	key := el.TextExpWithRanges
	rs := ranges(key)

	if len(rs) == 0 {
		if d, ok := deriveCheck(key, syms); ok {
			return d, nil
		}
		return dDesc{}, fmt.Errorf("no bit ranges and not a known check: %q", firstLine(key))
	}

	d := dDesc{AltBit: -1, Stride: 1, Fixed: -1}

	// --- a value table supplies both the field order and the mapping.
	if role, isTable := tableRole(el.Symbol); isTable {
		fields, rows, ok := parseTable(key, rs, syms)
		if ok {
			if err := checkDomain(role, syms, rows); err != nil {
				return d, err
			}
			d.Fields, d.Table, d.Role = fields, rows, role
			return d, nil
		}
		// <const> is the one table symbol that is usually *not* a table: it names
		// whatever single immediate the form takes. The others are tables in every
		// occurrence, so a missing table there is a parse failure to report rather
		// than a form to fall through.
		if el.Symbol != "<const>" {
			return d, fmt.Errorf("%s names a value table but none parsed", el.Symbol)
		}
	}

	// --- the 8-bit floating-point immediate. The prose states the representation
	// exactly, and it is Imm_fp's: 1 sign bit, a 3-bit exponent biased by 3, and a
	// 4-bit fraction over an implicit 16. So this needs no new Arg type
	// and no value table.
	if strings.Contains(key, "expressible as") && strings.Contains(key, "sign bit") {
		if f, ok := rs["imm8"]; ok {
			d.Fields, d.Role, d.FP8 = []dField{f}, roleImm, true
			return d, nil
		}
	}

	fs, err := fieldOrder(key, rs)
	if err != nil {
		return d, err
	}
	d.Fields = fs

	// --- register file. instgen's own is_p/is_z classification comes first and
	// is more reliable than the vocabulary; the prose is needed only to separate
	// P from PN and to size the general-purpose registers.
	switch {
	case rePNDigits.MatchString(key):
		d.Regfile = rfPN
	case el.IsP:
		d.Regfile = rfP
	case el.IsZ:
		d.Regfile = rfZ
	case el.FixedScalarWidth == 64:
		d.Regfile = rfR64
	case el.FixedScalarWidth == 32:
		d.Regfile = rfR32
	case strings.Contains(key, "SIMD&FP register"):
		d.Regfile = rfV
		// Most of these say "the number [0-31] of the ... SIMD&FP register" and
		// take their width from the element size. The widening reductions say "the
		// 64-bit name" instead, because their accumulator is 64-bit whatever the
		// source elements are, and honouring that is the difference between
		// "saddv d2, p3, z6.b" and "saddv b2, p3, z6.b".
		if m := reNamedWidth.FindStringSubmatch(key); m != nil {
			d.VWidth, _ = strconv.Atoi(m[1])
		}
	case strings.Contains(key, "scalable vector register"):
		d.Regfile = rfZ
	case strings.Contains(key, "scalable predicate register"):
		d.Regfile = rfP
	case strings.Contains(key, "general-purpose"):
		switch {
		case strings.Contains(key, "32-bit"):
			d.Regfile = rfR32
		case strings.Contains(key, "64-bit"):
			d.Regfile = rfR64
		default:
			// ARM writes "<R><m>" and leaves the width to the element size, as
			// INDEX does: "Is the number [0-30] of the source general-purpose
			// register or the name ZR (31)". Go spells both W and X as Rn, so this
			// only shows up in GNU syntax -- where guessing X prints x3 for w3.
			d.Regfile = rfRauto
		}
	}
	if strings.Contains(key, "or stack pointer") || strings.Contains(key, "the name SP (31)") {
		d.SP = true
	}
	if reDefaultsXZR.MatchString(key) {
		d.ZR = true
	}

	// --- numeric transforms, all stated by the prose.
	if m := reInRange.FindStringSubmatch(key); m != nil {
		d.HasRng = true
		d.Min, _ = strconv.ParseInt(m[1], 10, 64)
		d.Max, _ = strconv.ParseInt(m[2], 10, 64)
		if d.Min < 0 {
			d.Signed = true
		}
	}
	if reSigned.MatchString(key) {
		d.Signed = true
	}
	if m := reMultiple.FindStringSubmatch(key); m != nil {
		d.Stride, _ = strconv.ParseInt(m[1], 10, 64)
	} else if m := reTimes.FindStringSubmatch(key); m != nil {
		d.Stride, _ = strconv.ParseInt(m[1], 10, 64)
	}
	if m := rePlus.FindStringSubmatch(key); m != nil {
		d.Bias, _ = strconv.ParseInt(m[1], 10, 64)
	} else if strings.Contains(key, "group") || strings.Contains(key, "second destination") {
		// The multi-vector operands say "the second register of the group" and
		// encode the group's first, so there the ordinal IS the bias. So does the
		// second destination of a pair of predicates, which is the first plus one.
		//
		// The "group" test matters: "the second source scalable vector register"
		// is an ordinal among operands, not among registers, and taking it as a
		// bias would add 1 to the register number of most three-operand
		// instructions.
		if m := reOrdinal.FindStringSubmatch(key); m != nil {
			d.Bias = int64(map[string]int{"first": 0, "second": 1, "third": 2, "fourth": 3}[m[1]])
		}
	}
	// TODO: the second register of TBL with two tables, SPLICE and EXT also
	// wraps modulo 32, and the second predicate of PEXT's pair modulo 16, but
	// only their ASL says so, so with Zn = 31 or Pd = 15 they do not decode.
	if strings.Contains(key, "modulo 32") {
		d.Modulo = 32
	}
	// A register file restricted to a sub-range (Z0-Z7, P0-P7, PN8-PN15) says so
	// in the prose. PN is numbered from 16, so its low end is also the bias.
	if m := rePNRange.FindStringSubmatch(key); m != nil {
		lo, _ := strconv.Atoi(m[1])
		hi, _ := strconv.Atoi(m[2])
		d.Regfile, d.HasRng, d.Min, d.Max = rfPN, true, int64(lo), int64(hi)
		// "PN8-PN15" over a 3-bit field cannot hold 8..15 directly, so the low end
		// of the named range is also the bias. "Z0-Z7" over the same width can, so
		// it is a range restriction only. The field width decides which.
		if int64(1)<<uint(d.width()) <= int64(hi) {
			d.Bias = int64(lo)
		}
	} else if m := reZRange.FindStringSubmatch(key); m != nil && d.Regfile == rfZ {
		lo, _ := strconv.Atoi(m[1])
		hi, _ := strconv.Atoi(m[2])
		d.HasRng, d.Min, d.Max = true, int64(lo), int64(hi)
	} else if m := rePRange.FindStringSubmatch(key); m != nil && d.Regfile == rfP {
		lo, _ := strconv.Atoi(m[1])
		hi, _ := strconv.Atoi(m[2])
		d.HasRng, d.Min, d.Max = true, int64(lo), int64(hi)
	}
	if f, ok := rs["sh"]; ok && strings.Contains(key, "LSL #8") {
		d.AltBit, d.AltSh = f.Lo, 8
	}

	// --- the three families whose value depends on the element size.
	//
	// ARM states each one in prose and names the field composite that carries it;
	// what the prose does not spell out is the element size, which it calls "the
	// number of bits per element". Each family is one runtime case, not one per
	// instruction.
	switch {
	case strings.Contains(key, "one less than the number of elements in"):
		d.TszMode = "tszIndex"
	case strings.Contains(key, "range 1 to number of bits per element"):
		d.TszMode = "tszShiftR"
	case strings.Contains(key, "range 0 to number of bits per element minus 1"):
		d.TszMode = "tszShiftL"
	case strings.Contains(key, "bitmask consisting of replicated"):
		// The one case that needs real arithmetic rather than a knob: the imm13
		// logical immediate, decided inside ASL's DecodeBitMasks.
		d.Bitmask = true
	}
	if d.TszMode != "" || d.Bitmask {
		d.Role = roleImm
		if strings.Contains(cls, "IDX") {
			d.Role = roleIndex
		}
		return d, nil
	}

	// --- role for the bit-carrying non-table elements.
	switch {
	case d.Regfile != rfNone:
		d.Role = roleReg
	case strings.Contains(cls, "IDX") && isIndexSym(el.Symbol):
		d.Role = roleIndex
	case strings.HasPrefix(cls, "AC_MEM"):
		d.Role = roleOff
	default:
		d.Role = roleImm
	}
	// The narrowing shifts that fix the element size say "the immediate shift
	// amount, in the range 1 to N" for a field that holds N minus the amount.
	// The prose does not say so; ARM's decode pseudocode does ("shift = esize -
	// UInt(imm3)").
	if m := reShiftNarrow.FindStringSubmatch(key); m != nil {
		d.Invert, _ = strconv.ParseInt(m[1], 10, 64)
	}
	if d.Role != roleReg && !d.HasRng && d.Modulo == 0 {
		// A W-bit field with stride S and bias B holds B .. B+S*(2^W-1), and that
		// is exactly what the hand-written encode bodies enforce when the prose
		// omits a range. Modulo cases wrap instead and get no range.
		d.HasRng = true
		d.Min = d.Bias
		d.Max = d.Bias + d.Stride*(int64(1)<<uint(d.width())-1)
	}
	return d, nil
}

// fieldOrder returns the value's pieces ordered low-to-high significance.
//
// ARM names composite fields high-part-first: "i3h:i3l" means i3h holds the high
// bits, so the reversed colon list is the significance order. Reading it left to
// right swaps the halves and still passes many pins, because several of these
// fields are symmetric.
func fieldOrder(key string, rs map[string]dField) ([]dField, error) {
	if m := reComposite.FindStringSubmatch(key); m != nil {
		names := strings.Split(m[1], ":")
		out := make([]dField, 0, len(names))
		ok := true
		for i := len(names) - 1; i >= 0; i-- {
			f, have := rs[names[i]]
			if !have {
				ok = false
				break
			}
			out = append(out, f)
		}
		if ok {
			return out, nil
		}
	}
	if m := reOneField.FindStringSubmatch(key); m != nil {
		if f, ok := rs[m[1]]; ok {
			return []dField{f}, nil
		}
	}
	var body []string
	for n := range rs {
		if n != "sh" {
			body = append(body, n)
		}
	}
	if len(body) == 1 {
		return []dField{rs[body[0]]}, nil
	}
	sort.Strings(body)
	return nil, fmt.Errorf("cannot tell which field holds the value (candidates %v)", body)
}

func isIndexSym(s string) bool {
	s = strings.Trim(s, "<>")
	switch s {
	case "index", "imm", "i1", "i2", "i3", "idx", "imm1", "imm2":
		return true
	}
	return false
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
