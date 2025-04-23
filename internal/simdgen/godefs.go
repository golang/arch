// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"log"
	"slices"

	"golang.org/x/arch/internal/unify"
)

type Operation struct {
	Go       string  // Go method name
	Category *string // General operation category (optional)

	GoArch string // GOARCH for this definition
	Asm    string // Assembly mnemonic

	In  []Operand // Arguments
	Out []Operand // Results
}

type Operand struct {
	Class string

	Go     *string // Go type of this operand
	AsmPos int     // Position of this operand in the assembly instruction

	Base     *string // Base Go type ("int", "uint", "float")
	ElemBits *int    // Element bit width
	Bits     int     // Total vector bit width

	Const *string // Optional constant value
}

func (o Operand) Compare(p Operand) int {
	// Put mask operands after others
	if o.Class != "mask" && p.Class == "mask" {
		return -1
	}
	if o.Class == "mask" && p.Class != "mask" {
		return 1
	}
	return 0
}

var argNames = []string{"x", "y", "z", "w"}

func writeGoDefs(w io.Writer, cl unify.Closure) {
	// TODO: Merge operations with the same signature but multiple
	// implementations (e.g., SSE vs AVX)

	// TODO: This code is embarrassing, but I'm very tired.

	var op Operation
	for def := range cl.All() {
		if !def.Exact() {
			continue
		}
		if err := def.Decode(&op); err != nil {
			log.Println(err.Error())
			log.Println(def)
			continue
		}

		in := slices.Clone(op.In)
		slices.SortStableFunc(in, Operand.Compare)
		out := slices.Clone(op.Out)
		slices.SortStableFunc(out, Operand.Compare)

		type argExtra struct {
			*Operand
			varName string
		}
		asmPosToArg := make(map[int]argExtra)
		asmPosToRes := make(map[int]argExtra)
		argNames := argNames

		fmt.Fprintf(w, "func (%s %s) %s(", argNames[0], *in[0].Go, op.Go)
		asmPosToArg[in[0].AsmPos] = argExtra{&in[0], argNames[0]}
		argNames = argNames[1:]
		i := 0
		for _, arg := range in[1:] {
			varName := ""

			// Drop operands with constant values
			if arg.Const == nil {
				if i > 0 {
					fmt.Fprint(w, ", ")
				}
				i++
				varName = argNames[0]
				fmt.Fprintf(w, "%s %s", varName, *arg.Go)
				argNames = argNames[1:]
			}
			asmPosToArg[arg.AsmPos] = argExtra{&arg, varName}
		}
		fmt.Fprintf(w, ") (")
		for i, res := range out {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			varName := string('o' + byte(i))
			fmt.Fprintf(w, "%s %s", varName, *res.Go)
			asmPosToRes[res.AsmPos] = argExtra{&res, varName}
		}
		fmt.Fprintf(w, ") {\n")

		fmt.Fprintf(w, "\t// %s", op.Asm)
		for i := 0; ; i++ {
			arg, okArg := asmPosToArg[i]
			if okArg {
				if arg.Const != nil {
					fmt.Fprintf(w, " %s", *arg.Const)
				} else {
					fmt.Fprintf(w, " %s", arg.varName)
				}
			}

			res, okRes := asmPosToRes[i]
			if okRes {
				if okArg {
					fmt.Fprintf(w, "/")
				} else {
					fmt.Fprintf(w, " ")
				}
				fmt.Fprintf(w, "%s", res.varName)
			}
			if !okArg && !okRes {
				break
			}
		}
		fmt.Fprintf(w, "\n")

		fmt.Fprintf(w, "}\n")
	}
}
