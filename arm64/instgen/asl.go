// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The decode pseudocode of an iclass can declare a word invalid on a field that
// the form's row in the decoder tables leaves free. The row of PRFB with a scalar
// index matches any Rm, so 0x841fde84 (Rm = 31) would decode as
// prfb pldl3keep, p7, [x20, xzr]. Its pseudocode says
//
//	if Rm == '11111' then EndOfDecode(Decode_UNDEF); end;
//
// This file compiles each such condition into a test of the word, here
// w&0x1f0000 == 0x1f0000. Forms with the same conditions share a class, whose
// number goes in the reject field of their sveForms rows, and the generated
// sveRejected runs the class's tests. instgen stops on a condition it cannot
// compile.

package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/arch/arm64/instgen/xmlspec"
)

// aslMaxVL is the maximum vector length, in bits, that the conditions on
// MaxImplementedAnyVL are evaluated against: the largest that SVE allows.
const aslMaxVL = 2048

// ---------------------------------------------------------------------------
// Extracting the conditions
// ---------------------------------------------------------------------------

var (
	// A statement "if cond then body end;", with the body running to the end of
	// the statement or to the next elsif or else. The pseudocode is not parsed
	// as a language; the statements that matter all have this shape, and the
	// expression form "let x = if c then a else b;" has no reject in its body
	// and falls out at the next step.
	reASLIf = regexp.MustCompile(`(?s)\bif\s+(.+?)\s+then\s*(.*?)(?:\bend;|\belsif\b|\belse\b)`)
	// A body declares the instance invalid if it ends the decode.
	reASLReject = regexp.MustCompile(`EndOfDecode\s*\(|\bUNDEFINED\b`)
	reASLNested = regexp.MustCompile(`\bif\b`)
	// A local alias for a field or a concatenation of fields, which is how the
	// narrowing instructions name tszh::tszl.
	reASLLet = regexp.MustCompile(`let\s+(\w+)\s*:\s*bits\(\d+\)\s*=\s*([\w:\[\]]+)\s*;`)
)

// rejectConditions returns the conditions in the decode pseudocode that end the
// decode, each with its white space normalized. It is an error if a statement
// that ends the decode is not directly inside such an if: in an elsif or else
// branch, or inside a nested if, which the pattern above would drop or truncate.
func rejectConditions(decode string) ([]string, error) {
	var out []string
	for _, m := range reASLIf.FindAllStringSubmatch(decode, -1) {
		if !reASLReject.MatchString(m[2]) {
			continue
		}
		if reASLNested.MatchString(m[2]) {
			return nil, fmt.Errorf("a reject inside a nested if: %q", m[0])
		}
		out = append(out, strings.Join(strings.Fields(m[1]), " "))
	}
	if n := len(reASLReject.FindAllString(decode, -1)); n != len(out) {
		return nil, fmt.Errorf("%d statements end the decode and %d of them are directly inside an if", n, len(out))
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// The condition language
// ---------------------------------------------------------------------------

// aslBits is a value made of bits of the instruction word, most significant
// first: aslBits{23, 22} is the field size.
type aslBits []int

// aslNode is a boolean expression over the instruction word.
type aslNode interface{ aslNode() }

type (
	// aslConst is a condition that does not depend on the word.
	aslConst bool
	// aslMatch is w&mask == value, or w&mask != value if ne is set.
	aslMatch struct {
		mask, value uint32
		ne          bool
	}
	aslNot struct{ x aslNode }
	aslAnd []aslNode
	aslOr  []aslNode
)

func (aslConst) aslNode() {}
func (aslMatch) aslNode() {}
func (aslNot) aslNode()   {}
func (aslAnd) aslNode()   {}
func (aslOr) aslNode()    {}

// The constructors fold what they can, so that a condition which is decided by
// the assumptions in compileCondition's doc comes out as a constant and one that
// is partly decided comes out without the decided part.

func mkNot(x aslNode) aslNode {
	switch x := x.(type) {
	case aslConst:
		return !x
	case aslMatch:
		x.ne = !x.ne
		return x
	case aslNot:
		return x.x
	}
	return aslNot{x}
}

func mkAnd(xs ...aslNode) aslNode {
	var out aslAnd
	for _, x := range xs {
		switch x := x.(type) {
		case aslConst:
			if !x {
				return aslConst(false)
			}
		case aslAnd:
			out = append(out, x...)
		default:
			out = append(out, x)
		}
	}
	switch len(out) {
	case 0:
		return aslConst(true)
	case 1:
		return out[0]
	}
	return out
}

func mkOr(xs ...aslNode) aslNode {
	var out aslOr
	for _, x := range xs {
		switch x := x.(type) {
		case aslConst:
			if x {
				return aslConst(true)
			}
		case aslOr:
			out = append(out, x...)
		default:
			out = append(out, x)
		}
	}
	switch len(out) {
	case 0:
		return aslConst(false)
	case 1:
		return out[0]
	}
	return out
}

// ---------------------------------------------------------------------------
// Lexer
// ---------------------------------------------------------------------------

type aslTok struct {
	kind byte // 'i' name, 'n' number, 'b' bit pattern, 'p' punctuation, 0 at the end
	text string
}

var reASLTok = regexp.MustCompile(`^(?:([A-Za-z_]\w*)|(\d+)|'([01x]*)'|(::|&&|\|\||==|!=|<=|>=|[<>!(){}\[\],:]))`)

func aslLex(s string) ([]aslTok, error) {
	var toks []aslTok
	for s = strings.TrimSpace(s); s != ""; s = strings.TrimSpace(s) {
		m := reASLTok.FindStringSubmatch(s)
		if m == nil {
			return nil, fmt.Errorf("unexpected %q", s)
		}
		switch {
		case m[1] != "":
			toks = append(toks, aslTok{'i', m[1]})
		case m[2] != "":
			toks = append(toks, aslTok{'n', m[2]})
		case m[4] != "":
			toks = append(toks, aslTok{'p', m[4]})
		default:
			toks = append(toks, aslTok{'b', m[3]})
		}
		s = s[len(m[0]):]
	}
	return toks, nil
}

// ---------------------------------------------------------------------------
// Parser
// ---------------------------------------------------------------------------

type aslParser struct {
	toks    []aslTok
	pos     int
	fields  map[string]xmlspec.IRField
	aliases map[string]aslBits
}

func (p *aslParser) peek() aslTok {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return aslTok{}
}

func (p *aslParser) next() aslTok {
	t := p.peek()
	if p.pos < len(p.toks) {
		p.pos++
	}
	return t
}

func (p *aslParser) punct(s string) bool {
	if t := p.peek(); t.kind == 'p' && t.text == s {
		p.pos++
		return true
	}
	return false
}

func (p *aslParser) expect(s string) error {
	if !p.punct(s) {
		return fmt.Errorf("expected %q, found %q", s, p.peek().text)
	}
	return nil
}

// compileCondition parses one condition, which has this grammar:
//
//	or      = and { "||" and }
//	and     = unary { "&&" unary }
//	unary   = "!" unary | "(" or ")" | atom
//	atom    = field "==" bits | field "!=" bits | field "IN" "{" bits {"," bits} "}"
//	        | "IsFeatureImplemented" "(" name ")" | "Have" name "(" ")"
//	        | "MaxImplementedAnyVL" "(" ")" relop number
//	field   = term { "::" term }
//	term    = name [ "[" hi [ ":" lo ] "]" ]
//
// A name is a box of the register diagram, or an alias that the pseudocode binds
// with "let name : bits(n) = field;". A bits literal may contain x, for a bit
// that does not matter.
//
// Every feature is taken to be implemented, as objdump and llvm-mc assume, and
// the maximum vector length to be aslMaxVL, so the tests of features and of
// MaxImplementedAnyVL never reject. A disassembler reads other machines'
// binaries, so it must not refuse an instruction that this machine lacks.
func compileCondition(cond string, fields map[string]xmlspec.IRField, aliases map[string]aslBits) (aslNode, error) {
	toks, err := aslLex(cond)
	if err != nil {
		return nil, err
	}
	p := &aslParser{toks: toks, fields: fields, aliases: aliases}
	n, err := p.or()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.kind != 0 {
		return nil, fmt.Errorf("unexpected %q after the condition", t.text)
	}
	return n, nil
}

func (p *aslParser) or() (aslNode, error) {
	var xs []aslNode
	for {
		x, err := p.and()
		if err != nil {
			return nil, err
		}
		xs = append(xs, x)
		if !p.punct("||") {
			return mkOr(xs...), nil
		}
	}
}

func (p *aslParser) and() (aslNode, error) {
	var xs []aslNode
	for {
		x, err := p.unary()
		if err != nil {
			return nil, err
		}
		xs = append(xs, x)
		if !p.punct("&&") {
			return mkAnd(xs...), nil
		}
	}
}

func (p *aslParser) unary() (aslNode, error) {
	if p.punct("!") {
		x, err := p.unary()
		if err != nil {
			return nil, err
		}
		return mkNot(x), nil
	}
	if p.punct("(") {
		x, err := p.or()
		if err != nil {
			return nil, err
		}
		if err := p.expect(")"); err != nil {
			return nil, err
		}
		return x, nil
	}
	return p.atom()
}

func (p *aslParser) atom() (aslNode, error) {
	t := p.peek()
	if t.kind != 'i' {
		return nil, fmt.Errorf("expected a condition, found %q", t.text)
	}
	// A call: a feature check, or the maximum vector length.
	if n := p.pos + 1; n < len(p.toks) && p.toks[n].kind == 'p' && p.toks[n].text == "(" {
		return p.call()
	}
	bits, err := p.field()
	if err != nil {
		return nil, err
	}
	op := p.next()
	switch {
	case op.kind == 'p' && (op.text == "==" || op.text == "!="):
		lit := p.next()
		if lit.kind != 'b' {
			return nil, fmt.Errorf("expected a bit pattern after %s, found %q", op.text, lit.text)
		}
		m, err := aslPattern(bits, lit.text)
		if err != nil {
			return nil, err
		}
		if op.text == "!=" {
			return mkNot(m), nil
		}
		return m, nil
	case op.kind == 'i' && op.text == "IN":
		if err := p.expect("{"); err != nil {
			return nil, err
		}
		var alts []aslNode
		for {
			lit := p.next()
			if lit.kind != 'b' {
				return nil, fmt.Errorf("expected a bit pattern in the set, found %q", lit.text)
			}
			m, err := aslPattern(bits, lit.text)
			if err != nil {
				return nil, err
			}
			alts = append(alts, m)
			if p.punct(",") {
				continue
			}
			if err := p.expect("}"); err != nil {
				return nil, err
			}
			return mkOr(alts...), nil
		}
	}
	return nil, fmt.Errorf("expected ==, != or IN after a field, found %q", op.text)
}

// call handles the predicates that do not depend on the word.
func (p *aslParser) call() (aslNode, error) {
	name := p.next().text
	p.next() // (
	switch {
	case name == "IsFeatureImplemented":
		if t := p.next(); t.kind != 'i' || !strings.HasPrefix(t.text, "FEAT_") {
			return nil, fmt.Errorf("IsFeatureImplemented of %q", t.text)
		}
		if err := p.expect(")"); err != nil {
			return nil, err
		}
		return aslConst(true), nil
	case strings.HasPrefix(name, "Have"):
		if err := p.expect(")"); err != nil {
			return nil, err
		}
		return aslConst(true), nil
	case name == "MaxImplementedAnyVL":
		if err := p.expect(")"); err != nil {
			return nil, err
		}
		op, n := p.next(), p.next()
		v, err := strconv.Atoi(n.text)
		if op.kind != 'p' || n.kind != 'n' || err != nil {
			return nil, fmt.Errorf("MaxImplementedAnyVL() compared with %q %q", op.text, n.text)
		}
		switch op.text {
		case "<":
			return aslConst(aslMaxVL < v), nil
		case "<=":
			return aslConst(aslMaxVL <= v), nil
		case ">":
			return aslConst(aslMaxVL > v), nil
		case ">=":
			return aslConst(aslMaxVL >= v), nil
		case "==":
			return aslConst(aslMaxVL == v), nil
		case "!=":
			return aslConst(aslMaxVL != v), nil
		}
		return nil, fmt.Errorf("MaxImplementedAnyVL() %s", op.text)
	}
	return nil, fmt.Errorf("unknown function %s", name)
}

// field parses term { "::" term }.
func (p *aslParser) field() (aslBits, error) {
	var out aslBits
	for {
		b, err := p.term()
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
		if !p.punct("::") {
			return out, nil
		}
	}
}

// term parses a name with an optional bit slice. Bits are numbered from the least
// significant, as in the pseudocode, so name[0] is the lowest bit of the name.
func (p *aslParser) term() (aslBits, error) {
	t := p.next()
	if t.kind != 'i' {
		return nil, fmt.Errorf("expected a field, found %q", t.text)
	}
	bits, err := aslResolve(t.text, p.fields, p.aliases)
	if err != nil {
		return nil, err
	}
	if !p.punct("[") {
		return bits, nil
	}
	hi := p.next()
	lo := hi
	if p.punct(":") {
		lo = p.next()
	}
	h, errH := strconv.Atoi(hi.text)
	l, errL := strconv.Atoi(lo.text)
	if errH != nil || errL != nil || l > h || h >= len(bits) {
		return nil, fmt.Errorf("bad slice of %s", t.text)
	}
	if err := p.expect("]"); err != nil {
		return nil, err
	}
	// bits[0] is the most significant bit, which is number len-1.
	return bits[len(bits)-1-h : len(bits)-l], nil
}

func aslResolve(name string, fields map[string]xmlspec.IRField, aliases map[string]aslBits) (aslBits, error) {
	if b, ok := aliases[name]; ok {
		return b, nil
	}
	f, ok := fields[name]
	if !ok {
		return nil, fmt.Errorf("unknown field %q", name)
	}
	var b aslBits
	for i := f.Hi - 1; i >= f.Lo; i-- {
		b = append(b, i)
	}
	return b, nil
}

// aslPattern compares the bits with a pattern written most significant first.
func aslPattern(bits aslBits, pat string) (aslNode, error) {
	if len(pat) != len(bits) {
		return nil, fmt.Errorf("pattern '%s' for %d bits", pat, len(bits))
	}
	var m aslMatch
	for i, c := range pat {
		bit := uint32(1) << uint(bits[i])
		switch c {
		case '0':
			m.mask |= bit
		case '1':
			m.mask |= bit
			m.value |= bit
		}
	}
	if m.mask == 0 {
		return aslConst(true), nil
	}
	return m, nil
}

// aslAliases collects the aliases that the pseudocode binds before it uses them.
// An alias whose right-hand side is not a field, a slice or a concatenation of
// such is not recorded, and a condition that uses it fails to compile.
func aslAliases(decode string, fields map[string]xmlspec.IRField) map[string]aslBits {
	out := map[string]aslBits{}
	for _, m := range reASLLet.FindAllStringSubmatch(decode, -1) {
		p := &aslParser{aliases: out, fields: fields}
		toks, err := aslLex(m[2])
		if err != nil {
			continue
		}
		p.toks = toks
		if b, err := p.field(); err == nil && p.peek().kind == 0 {
			out[m[1]] = b
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Emitting
// ---------------------------------------------------------------------------

// goExpr returns n as a Go expression over w, parenthesized as its context needs:
// prec is 2 inside an && and 1 inside an ||.
func goExpr(n aslNode, prec int) string {
	switch n := n.(type) {
	case aslConst:
		return strconv.FormatBool(bool(n))
	case aslMatch:
		op := "=="
		if n.ne {
			op = "!="
		}
		return fmt.Sprintf("w&%#x %s %s", n.mask, op, hexOrZero(n.value))
	case aslNot:
		return "!(" + goExpr(n.x, 0) + ")"
	case aslAnd:
		return joinExpr(n, " && ", 2)
	case aslOr:
		s := joinExpr(n, " || ", 1)
		if prec > 1 {
			return "(" + s + ")"
		}
		return s
	}
	panic(fmt.Sprintf("goExpr: %T", n))
}

func joinExpr(xs []aslNode, sep string, prec int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = goExpr(x, prec)
	}
	return strings.Join(parts, sep)
}

func hexOrZero(v uint32) string {
	if v == 0 {
		return "0"
	}
	return fmt.Sprintf("%#x", v)
}

// ---------------------------------------------------------------------------
// The constraints of one iclass
// ---------------------------------------------------------------------------

// aslGuard is one reject condition: the pseudocode that states it, and the test
// of the word that it compiles to.
type aslGuard struct {
	ASL  string
	Expr aslNode
}

// aslStats counts what became of the conditions that end the decode.
type aslStats struct {
	Conditions int // all of them
	Emitted    int // those that constrain the word
	Satisfied  int // those that the assumptions decide, which never reject
}

// iclassGuards returns the reject constraints of an iclass, in the order the
// pseudocode states them. A condition that the grammar does not cover is an
// error: dropping it would make the decoder accept what the architecture
// forbids, silently.
func iclassGuards(ic xmlspec.IRIclass) ([]aslGuard, aslStats, error) {
	var gs []aslGuard
	var st aslStats
	if ic.Decode == "" {
		return nil, st, nil
	}
	aliases := aslAliases(ic.Decode, ic.Fields)
	conds, err := rejectConditions(ic.Decode)
	if err != nil {
		return nil, st, fmt.Errorf("iclass %s: %v", ic.Name, err)
	}
	for _, cond := range conds {
		st.Conditions++
		n, err := compileCondition(cond, ic.Fields, aliases)
		if err != nil {
			return nil, st, fmt.Errorf("iclass %s: %q: %v", ic.Name, cond, err)
		}
		if c, ok := n.(aslConst); ok && !bool(c) {
			st.Satisfied++
			continue
		}
		st.Emitted++
		gs = append(gs, aslGuard{cond, n})
	}
	return gs, st, nil
}

// guardKey identifies a list of guards, so that forms with the same constraints
// can share a case of the generated switch. The pseudocode is not enough: it names
// fields, and the same name is a different place in the word in different iclasses.
func guardKey(gs []aslGuard) string {
	var b strings.Builder
	for _, g := range gs {
		b.WriteString(g.ASL)
		b.WriteByte(0)
	}
	b.WriteString(guardExpr(gs))
	return b.String()
}

// guardExpr is the test of the word that rejects it under any of the guards.
func guardExpr(gs []aslGuard) string {
	xs := make([]aslNode, len(gs))
	for i, g := range gs {
		xs[i] = g.Expr
	}
	return goExpr(mkOr(xs...), 0)
}

// guardCase is one case of the generated switch.
type guardCase struct {
	Forms  []int
	Guards []aslGuard
}

// groupGuards groups the forms that share their constraints, in the order of
// each group's first form. perForm[i] is the guards of form i.
func groupGuards(perForm [][]aslGuard) []guardCase {
	idx := map[string]int{}
	var cases []guardCase
	for i, gs := range perForm {
		if len(gs) == 0 {
			continue
		}
		k := guardKey(gs)
		j, ok := idx[k]
		if !ok {
			j = len(cases)
			idx[k] = j
			cases = append(cases, guardCase{Guards: gs})
		}
		cases[j].Forms = append(cases[j].Forms, i)
	}
	sort.SliceStable(cases, func(a, b int) bool { return cases[a].Forms[0] < cases[b].Forms[0] })
	return cases
}
