// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"golang.org/x/arch/x86/xeddata"
)

// instGroup holds a list of instructions with same opcode.
type instGroup struct {
	opcode string
	list   []*instruction
}

// context is x86avxgen program execution state.
type context struct {
	db *xeddata.Database

	groups []*instGroup

	optabs    map[string]*optab
	ytabLists map[string]*ytabList

	// Command line arguments:

	xedPath string
}

func main() {
	log.SetPrefix("x86avxgen: ")
	log.SetFlags(log.Lshortfile)

	var ctx context

	runSteps(&ctx,
		parseFlags,
		openDatabase,
		buildTables,
		printTables)
}

func buildTables(ctx *context) {
	// Order of steps is significant.
	runSteps(ctx,
		decodeGroups,
		mergeRegMem,
		addGoSuffixes,
		mergeWIG,
		assignZforms,
		sortGroups,
		generateOptabs)
}

func runSteps(ctx *context, steps ...func(*context)) {
	for _, f := range steps {
		f(ctx)
	}
}

func parseFlags(ctx *context) {
	flag.StringVar(&ctx.xedPath, "xedPath", "./xedpath",
		"XED datafiles location")

	flag.Parse()
}

func openDatabase(ctx *context) {
	db, err := xeddata.NewDatabase(ctx.xedPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	ctx.db = db
}

// mergeRegMem merges reg-only with mem-only instructions.
// For example: {MOVQ reg, mem} + {MOVQ reg, reg} = {MOVQ reg, reg/mem}.
func mergeRegMem(ctx *context) {
	mergeKey := func(inst *instruction) string {
		return strings.Join([]string{
			fmt.Sprint(len(inst.args)),
			inst.enc.opbyte,
			inst.enc.opdigit,
			inst.enc.vex.P,
			inst.enc.vex.L,
			inst.enc.vex.M,
			inst.enc.vex.W,
		}, " ")
	}

	for _, g := range ctx.groups {
		regOnly := make(map[string]*instruction)
		memOnly := make(map[string]*instruction)
		list := g.list[:0]
		for _, inst := range g.list {
			switch {
			case inst.pset.Is("RegOnly"):
				regOnly[mergeKey(inst)] = inst
			case inst.pset.Is("MemOnly"):
				memOnly[mergeKey(inst)] = inst
			default:
				if len(inst.args) == 0 {
					list = append(list, inst)
					continue
				}
				log.Fatalf("%s: unexpected MOD value", inst)
			}
		}

		for k, m := range memOnly {
			r := regOnly[k]
			if r != nil {
				index := m.ArgIndexByZkind("reg/mem")
				arg := m.args[index]
				switch ytype := r.args[index].ytype; ytype {
				case "Yrl":
					arg.ytype = "Yml"
				case "Yxr":
					arg.ytype = "Yxm"
				case "YxrEvex":
					arg.ytype = "YxmEvex"
				case "Yyr":
					arg.ytype = "Yym"
				case "YyrEvex":
					arg.ytype = "YymEvex"
				case "Yzr":
					arg.ytype = "Yzm"
				case "Yk":
					arg.ytype = "Ykm"
				default:
					log.Fatalf("%s: unexpected register type: %s", r, ytype)
				}
				// Merge EVEX flags into m.
				m.enc.evex.SAE = m.enc.evex.SAE || r.enc.evex.SAE
				m.enc.evex.Rounding = m.enc.evex.Rounding || r.enc.evex.Rounding
				m.enc.evex.Zeroing = m.enc.evex.Zeroing || r.enc.evex.Zeroing
				delete(regOnly, k)
			}
			list = append(list, m)
		}
		for _, r := range regOnly {
			list = append(list, r)
		}

		g.list = list
	}
}

// mergeWIG merges [E]VEX.W0 + [E]VEX.W1 into [E]VEX.WIG.
func mergeWIG(ctx *context) {
	mergeKey := func(inst *instruction) string {
		return strings.Join([]string{
			fmt.Sprint(len(inst.args)),
			inst.enc.opbyte,
			inst.enc.opdigit,
			inst.enc.vex.P,
			inst.enc.vex.L,
			inst.enc.vex.M,
		}, " ")
	}

	for _, g := range ctx.groups {
		w0map := make(map[string]*instruction)
		w1map := make(map[string]*instruction)
		list := g.list[:0]
		for _, inst := range g.list {
			switch w := inst.enc.vex.W; w {
			case "evexW0", "vexW0":
				w0map[mergeKey(inst)] = inst
			case "evexW1", "vexW1":
				w1map[mergeKey(inst)] = inst
			default:
				log.Fatalf("%s: unexpected vex.W: %s", inst, w)
			}
		}

		for k, w0 := range w0map {
			w1 := w1map[k]
			if w1 != nil {
				w0.enc.vex.W = strings.Replace(w0.enc.vex.W, "W0", "WIG", 1)
				delete(w1map, k)
			}
			list = append(list, w0)
		}
		for _, w1 := range w1map {
			list = append(list, w1)
		}

		g.list = list
	}
}

// assignZforms initializes zform field of every instruction in ctx.
func assignZforms(ctx *context) {
	for _, g := range ctx.groups {
		for _, inst := range g.list {
			var parts []string
			if inst.pset.Is("EVEX") {
				parts = append(parts, "evex")
			}
			for _, arg := range inst.args {
				parts = append(parts, arg.zkind)
			}
			if inst.enc.opdigit != "" {
				parts = append(parts, "opdigit")
			}
			inst.zform = strings.Join(parts, " ")
		}
	}
}

// sortGroups sorts each instruction group by opcode as well as instructions
// inside groups by special rules (see below).
//
// The order of instructions inside group determine ytab
// elements order inside ytabList.
//
// We want these rules to be satisfied:
//   - EVEX-encoded entries go after VEX-encoded entries.
//     This way, VEX forms are selected over EVEX variants.
//   - EVEX forms with SAE/RC must go before forms without them.
//     This helps to avoid problems with reg-reg instructions
//     that encode either of them in ModRM.R/M which causes
//     ambiguity in ytabList (more than 1 ytab can match args).
//     If first matching ytab has SAE/RC, problem will not occur.
//   - Memory argument position affects order.
//     Required to be in sync with XED encoder when there
//     are multiple choices of how to encode instruction.
func sortGroups(ctx *context) {
	sort.SliceStable(ctx.groups, func(i, j int) bool {
		return ctx.groups[i].opcode < ctx.groups[j].opcode
	})

	for _, g := range ctx.groups {
		sortInstList(g.list)
	}
}

func sortInstList(insts []*instruction) {
	// Use strings for sorting to get reliable transitive "less".
	order := make(map[*instruction]string)
	for _, inst := range insts {
		encTag := 'a'
		if inst.pset.Is("EVEX") {
			encTag = 'b'
		}
		memTag := 'a'
		if index := inst.ArgIndexByZkind("reg/mem"); index != -1 {
			memTag = 'z' - rune(index)
		}
		rcsaeTag := 'a'
		if !(inst.enc.evex.SAE || inst.enc.evex.Rounding) {
			rcsaeTag = 'b'
		}
		order[inst] = fmt.Sprintf("%c%c%c %s",
			encTag, memTag, rcsaeTag, inst.YtypeListString())
	}

	sort.SliceStable(insts, func(i, j int) bool {
		return order[insts[i]] < order[insts[j]]
	})
}

// addGoSuffixes splits some groups into several groups by introducing a suffix.
// For example, ANDN group becomes ANDNL and ANDNQ (ANDN becomes empty itself).
// Empty groups are removed.
func addGoSuffixes(ctx *context) {
	var opcodeSuffixMatchers map[string][]string
	{
		opXY := []string{"VL=0", "X", "VL=1", "Y"}
		opXYZ := []string{"VL=0", "X", "VL=1", "Y", "VL=2", "Z"}
		opQ := []string{"REXW=1", "Q"}
		opLQ := []string{"REXW=0", "L", "REXW=1", "Q"}

		opcodeSuffixMatchers = map[string][]string{
			"VCVTPD2DQ":   opXY,
			"VCVTPD2PS":   opXY,
			"VCVTTPD2DQ":  opXY,
			"VCVTQQ2PS":   opXY,
			"VCVTUQQ2PS":  opXY,
			"VCVTPD2UDQ":  opXY,
			"VCVTTPD2UDQ": opXY,

			"VFPCLASSPD": opXYZ,
			"VFPCLASSPS": opXYZ,

			"VCVTSD2SI":  opQ,
			"VCVTTSD2SI": opQ,
			"VCVTTSS2SI": opQ,
			"VCVTSS2SI":  opQ,

			"VCVTSD2USI":  opLQ,
			"VCVTSS2USI":  opLQ,
			"VCVTTSD2USI": opLQ,
			"VCVTTSS2USI": opLQ,
			"VCVTUSI2SD":  opLQ,
			"VCVTUSI2SS":  opLQ,
			"VCVTSI2SD":   opLQ,
			"VCVTSI2SS":   opLQ,
			"ANDN":        opLQ,
			"BEXTR":       opLQ,
			"BLSI":        opLQ,
			"BLSMSK":      opLQ,
			"BLSR":        opLQ,
			"BZHI":        opLQ,
			"MULX":        opLQ,
			"PDEP":        opLQ,
			"PEXT":        opLQ,
			"RORX":        opLQ,
			"SARX":        opLQ,
			"SHLX":        opLQ,
			"SHRX":        opLQ,
		}
	}

	newGroups := make(map[string][]*instruction)
	for _, g := range ctx.groups {
		kv := opcodeSuffixMatchers[g.opcode]
		if kv == nil {
			continue
		}

		list := g.list[:0]
		for _, inst := range g.list {
			newOp := inst.opcode + inst.pset.Match(kv...)
			if newOp != inst.opcode {
				inst.opcode = newOp
				newGroups[newOp] = append(newGroups[newOp], inst)
			} else {
				list = append(list, inst)
			}
		}
		g.list = list
	}
	groups := ctx.groups[:0] // Filled with non-empty groups
	// Some groups may become empty due to opcode split.
	for _, g := range ctx.groups {
		if len(g.list) != 0 {
			groups = append(groups, g)
		}
	}
	for op, insts := range newGroups {
		groups = append(groups, &instGroup{
			opcode: op,
			list:   insts,
		})
	}
	ctx.groups = groups
}

func printTables(ctx *context) {
	writeTables(os.Stdout, ctx)
}
