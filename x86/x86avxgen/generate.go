// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"log"
	"strings"
)

// ytab is ytabList element.
type ytab struct {
	Zcase   string
	Zoffset int
	ArgList string // Ytypes that are matched by this ytab.
}

// ytabList is a named set of ytab objects.
// In asm6.go represented as []ytab.
type ytabList struct {
	Name  string
	Ytabs []ytab
}

// optab describes instruction encodings for specific opcode.
type optab struct {
	Opcode   string
	YtabList *ytabList
	OpLines  []string
}

type generator struct {
	ctx       *context
	ytabLists map[string]*ytabList
}

// generateOptabs fills ctx.optabs and ctx.ytabLists with objects created
// from decoded instructions.
func generateOptabs(ctx *context) {
	gen := generator{ctx: ctx, ytabLists: make(map[string]*ytabList)}
	optabs := make(map[string]*optab)
	for _, g := range ctx.groups {
		optabs[g.opcode] = gen.GenerateGroup(g)
	}
	ctx.optabs = optabs
	ctx.ytabLists = gen.ytabLists
}

// GenerateGroup converts g into optab.
// Populates internal ytab list map.
func (gen *generator) GenerateGroup(g *instGroup) *optab {
	var opLines []string
	for _, inst := range g.list {
		opLines = append(opLines, gen.generateOpLine(inst))
	}
	return &optab{
		Opcode:   "A" + g.opcode,
		OpLines:  opLines,
		YtabList: gen.internYtabList(g),
	}
}

// generateOpLine returns string that describes opBytes for single instruction form.
func (gen *generator) generateOpLine(inst *instruction) string {
	parts := []string{gen.prefixExpr(inst)}
	if inst.pset.Is("EVEX") {
		parts = append(parts, gen.evexPrefixExpr(inst))
	}
	parts = append(parts, inst.enc.opbyte)
	if inst.enc.opdigit != "" {
		parts = append(parts, inst.enc.opdigit)
	}
	return strings.Join(parts, ", ")
}

func (gen *generator) prefixExpr(inst *instruction) string {
	enc := inst.enc
	return gen.joinPrefixParts([]string{
		// Special constant that makes AVX byte different from 0x0F,
		// making it unnecessary to check for both VEX+EVEX when
		// assigning dealing with legacy instructions that skip it
		// without advancing "z" counter.
		"avxEscape",
		enc.vex.L,
		enc.vex.P,
		enc.vex.M,
		enc.vex.W,
	})
}

func (gen *generator) evexPrefixExpr(inst *instruction) string {
	enc := inst.enc
	parts := []string{
		enc.evexScale,
		enc.evexBcstScale,
	}
	if enc.evex.SAE {
		parts = append(parts, "evexSaeEnabled")
	}
	if enc.evex.Rounding {
		parts = append(parts, "evexRoundingEnabled")
	}
	if enc.evex.Zeroing {
		parts = append(parts, "evexZeroingEnabled")
	}
	return gen.joinPrefixParts(parts)
}

// joinPrefixParts returns the Go OR-expression for every non-empty name.
// If every name is empty, returns "0".
func (gen *generator) joinPrefixParts(names []string) string {
	filterEmptyStrings := func(xs []string) []string {
		ys := xs[:0]
		for _, x := range xs {
			if x != "" {
				ys = append(ys, x)
			}
		}
		return ys
	}

	names = filterEmptyStrings(names)
	if len(names) == 0 {
		return "0"
	}
	return strings.Join(names, "|")
}

// internYtabList returns ytabList for given group.
//
// Returned ytab lists are interned.
// Same ytab list can be returned for different groups.
func (gen *generator) internYtabList(g *instGroup) *ytabList {
	var key string
	{
		var buf bytes.Buffer
		for _, inst := range g.list {
			buf.WriteString(inst.zform)
			buf.WriteByte('=')
			buf.WriteString(inst.YtypeListString())
			buf.WriteByte(';')
		}
		key = buf.String()
	}
	if ylist := gen.ytabLists[key]; ylist != nil {
		return ylist
	}

	var ytabs []ytab
	for _, inst := range g.list {
		zoffset := 2
		if inst.pset.Is("EVEX") {
			zoffset++ // Always at least 3 bytes
		}
		if inst.enc.opdigit != "" {
			zoffset++
		}

		if inst.mask != nil {
			ytabs = append(ytabs, gen.makeMaskYtabs(zoffset, inst)...)
		} else {
			ytabs = append(ytabs, gen.makeYtab(zoffset, inst.zform, inst.args))
		}
	}
	ylist := &ytabList{
		Name:  "_y" + strings.ToLower(g.opcode),
		Ytabs: ytabs,
	}
	gen.ytabLists[key] = ylist
	return ylist
}

var zcaseByZform = map[string]string{
	"evex imm8 reg kmask reg/mem":          "Zevex_i_r_k_rm",
	"evex imm8 reg reg/mem":                "Zevex_i_r_rm",
	"evex imm8 reg/mem kmask reg":          "Zevex_i_rm_k_r",
	"evex imm8 reg/mem kmask regV opdigit": "Zevex_i_rm_k_vo",
	"evex imm8 reg/mem reg":                "Zevex_i_rm_r",
	"evex imm8 reg/mem regV opdigit":       "Zevex_i_rm_vo",
	"evex imm8 reg/mem regV kmask reg":     "Zevex_i_rm_v_k_r",
	"evex imm8 reg/mem regV reg":           "Zevex_i_rm_v_r",
	"evex kmask reg/mem opdigit":           "Zevex_k_rmo",
	"evex reg kmask reg/mem":               "Zevex_r_k_rm",
	"evex reg reg/mem":                     "Zevex_r_v_rm",
	"evex reg regV kmask reg/mem":          "Zevex_r_v_k_rm",
	"evex reg regV reg/mem":                "Zevex_r_v_rm",
	"evex reg/mem kmask reg":               "Zevex_rm_k_r",
	"evex reg/mem reg":                     "Zevex_rm_v_r",
	"evex reg/mem regV kmask reg":          "Zevex_rm_v_k_r",
	"evex reg/mem regV reg":                "Zevex_rm_v_r",

	"":                          "Zvex",
	"imm8 reg reg/mem":          "Zvex_i_r_rm",
	"imm8 reg/mem reg":          "Zvex_i_rm_r",
	"imm8 reg/mem regV opdigit": "Zvex_i_rm_vo",
	"imm8 reg/mem regV reg":     "Zvex_i_rm_v_r",
	"reg reg/mem":               "Zvex_r_v_rm",
	"reg regV reg/mem":          "Zvex_r_v_rm",
	"reg/mem opdigit":           "Zvex_rm_v_ro",
	"reg/mem reg":               "Zvex_rm_v_r",
	"reg/mem regV opdigit":      "Zvex_rm_r_vo",
	"reg/mem regV reg":          "Zvex_rm_v_r",
	"reg/mem":                   "Zvex_rm_v_r",
	"regIH reg/mem regV reg":    "Zvex_hr_rm_v_r",
	"regV reg/mem reg":          "Zvex_v_rm_r",
}

func (gen *generator) makeYtab(zoffset int, zform string, args []*argument) ytab {
	var ytypes []string
	for _, arg := range args {
		if arg.ytype != "Ynone" {
			ytypes = append(ytypes, arg.ytype)
		}
	}
	argList := strings.Join(ytypes, ", ")
	zcase := zcaseByZform[zform]
	if zcase == "" {
		log.Fatalf("no zcase for %q", zform)
	}
	return ytab{
		Zcase:   zcase,
		Zoffset: zoffset,
		ArgList: argList,
	}
}

// makeMaskYtabs returns 2 ytabs created from instruction with MASK1() argument.
//
// This is required due to how masking is implemented in asm6.
// Single MASK1() instruction produces 2 ytabs, for example:
//  1. OP xmm, mem     | Yxr, Yxm         | Does not permit K arguments (K0 implied)
//  2. OP xmm, K2, mem | Yxr, Yknot0, Yxm | Does not permit K0 argument
//
// This function also exploits that both ytab entries have same opbytes,
// hence it is efficient to emit only one opbytes line and 0 Z-offset
// for first ytab object.
func (gen *generator) makeMaskYtabs(zoffset int, inst *instruction) []ytab {
	var k0 ytab
	{
		zform := strings.Replace(inst.zform, "MASK1() ", "", 1)
		inst.mask.ytype = "Ynone"
		k0 = gen.makeYtab(0, zform, inst.args)
	}
	var knot0 ytab
	{
		zform := strings.Replace(inst.zform, "MASK1() ", "kmask ", 1)
		inst.mask.ytype = "Yknot0"
		knot0 = gen.makeYtab(zoffset, zform, inst.args)
	}

	inst.mask.ytype = "MASK1()" // Restore Y-type
	return []ytab{k0, knot0}
}
