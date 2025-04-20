// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"log"

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
	Go     string // Go type of this operand
	AsmPos int    // Position of this operand in the assembly instruction

	Base string // Base Go type ("int", "uint", "float")
	Bits int    // Element bit width
	W    int    // Total vector bit width
}

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
			continue
		}

		fmt.Fprintf(w, "func (x %s) %s(", op.In[0].Go, op.Go)
		for i, arg := range op.In[1:] {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			fmt.Fprintf(w, "%c %s", 'y'+i, arg.Go)
		}
		fmt.Fprintf(w, ") (")
		for i, res := range op.Out {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			fmt.Fprintf(w, "%c %s", 'o'+i, res.Go)
		}
		fmt.Fprintf(w, ") {\n")

		asmPosToArg := make(map[int]byte)
		asmPosToRes := make(map[int]byte)
		for i, arg := range op.In {
			asmPosToArg[arg.AsmPos] = 'x' + byte(i)
		}
		for i, res := range op.Out {
			asmPosToRes[res.AsmPos] = 'o' + byte(i)
		}
		fmt.Fprintf(w, "\t// %s", op.Asm)
		for i := 0; ; i++ {
			arg, okArg := asmPosToArg[i]
			if okArg {
				fmt.Fprintf(w, " %c", arg)
			}
			res, okRes := asmPosToRes[i]
			if okRes {
				if okArg {
					fmt.Fprintf(w, "/")
				} else {
					fmt.Fprintf(w, " ")
				}
				fmt.Fprintf(w, "%c", res)
			}
			if !okArg && !okRes {
				break
			}
		}
		fmt.Fprintf(w, "\n")

		fmt.Fprintf(w, "}\n")
	}
}
