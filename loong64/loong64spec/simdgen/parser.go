// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simdgen

import (
	"github.com/ledongthuc/pdf"
	"log"
	"strings"
)

var conds = [22]string{
	"CAF",
	"CUN",
	"CEQ",
	"CUEQ",
	"CLT",
	"CULT",
	"CLE",
	"CULE",
	"CNE",
	"COR",
	"CUNE",
	"SAF",
	"SUN",
	"SEQ",
	"SUEQ",
	"SLT",
	"SULT",
	"SLE",
	"SULE",
	"SNE",
	"SOR",
	"SUNE",
}

func parsePage(p pdf.Page, isFP bool) (insts []*Instruction) {
	validText := ExtrText(p, isFP)
	text := GetInst(validText)
	for i := 0; i < len(text); {
		j := i + 1
		instWords := []string{text[i].S}
		for ; j < len(text) && text[j].Y == text[i].Y; j++ {
			instWords = append(instWords, text[j].S)
		}

		asm := instWords[0]

		if strings.Contains(asm, "FCMP") {
			asmStrs := strings.Split(asm, ".")
			for _, con := range conds {
				fcmpInstAsm := asmStrs[0] + con + "." + asmStrs[2]
				insts = append(insts, &Instruction{Asm: fcmpInstAsm, Operands: instWords[1]})
			}
		}
		i = j

		// Irregular cell partitioning results in the retrieved data not
		// constituting complete instructions.
		if strings.ToUpper(asm) != asm || asm == "1" {
			continue
		}

		insts = append(insts, &Instruction{Asm: asm, Operands: instWords[1]})
	}
	return
}

// RawInsts loads and parses Loong64 instruction definitions from given file.
func RawInsts(path string) (insts []*Instruction) {
	f, r, err := pdf.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	n := r.NumPage()
	var fp int
	var curP int
	for curP = 1; curP <= n; curP++ {
		p := r.Page(curP)
		if fp == 0 {
			if !IsFirstPage(p) {
				continue
			}
			fp = curP
		}

	}
	if fp == 0 {
		log.Fatal("not found appendix B from given file: ", path)
	}
	for curP = fp; curP <= n; curP++ {
		p := r.Page(curP)
		insts = append(insts, parsePage(p, fp == curP)...)
	}
	return
}

type Instruction struct {
	Asm      string
	Operands string
}
