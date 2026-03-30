// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xmlspec

import (
	"fmt"
	"strings"
)

// String methods for Instruction and its types to provide a comprehensive recursive print.

func (i *InstructionParsed) String() string {
	if i == nil {
		return "Instruction{ <nil> }"
	}
	var sb strings.Builder
	sb.WriteString("Instruction{")
	sb.WriteString(indent("Title: "+i.Title, 1))
	if i.ParseError != "" {
		sb.WriteString(indent("ParseError: "+i.ParseError, 1))
	}
	sb.WriteString(indent("Type: "+i.Type, 1))
	sb.WriteString(indent("Desc:", 1))
	sb.WriteString(indent(i.Desc.String(), 2))
	sb.WriteString(indent("PsSections:", 1))
	for _, ps := range i.PsSections {
		sb.WriteString(indent(ps.String(), 2))
	}
	sb.WriteString(indent("DocVars:", 1))
	for _, dv := range i.DocVars {
		sb.WriteString(indent(dv.String(), 2))
	}
	sb.WriteString(indent("Classes:", 1))
	sb.WriteString(indent(i.Classes.String(), 2))
	sb.WriteString(indent("Explanations:", 1))
	sb.WriteString(indent(i.Explanations.String(), 2))
	sb.WriteString(indent("file: "+i.file, 1))
	sb.WriteString("\n}")
	return sb.String()
}

func (dv DocVar) String() string {
	var sb strings.Builder
	sb.WriteString("DocVar{")
	sb.WriteString(indent("Key: "+dv.Key, 1))
	sb.WriteString(indent("Value: "+dv.Value, 1))
	sb.WriteString("\n}")
	return sb.String()
}

func (c Classes) String() string {
	var sb strings.Builder
	sb.WriteString("Classes{")
	sb.WriteString(indent("Classesintro:", 1))
	sb.WriteString(indent("Iclass:", 1))
	for _, ic := range c.Iclass {
		sb.WriteString(indent(ic.String(), 2))
	}
	sb.WriteString("\n}")
	return sb.String()
}

func (ic Iclass) String() string {
	var sb strings.Builder
	sb.WriteString("Iclass{")
	sb.WriteString(indent("Name: "+ic.Name, 1))
	sb.WriteString(indent("DocVars:", 1))
	for _, dv := range ic.DocVars {
		sb.WriteString(indent(dv.String(), 2))
	}

	sb.WriteString(indent("PsSection:", 1))
	for _, ps := range ic.PsSection {
		sb.WriteString(indent(ps.String(), 2))
	}

	sb.WriteString(indent("Regdiagram:", 1))
	sb.WriteString(indent(ic.RegDiagram.String(), 2))

	sb.WriteString(indent("Encodings:", 1))
	for _, enc := range ic.Encodings {
		sb.WriteString(indent(enc.String(), 2))
	}
	sb.WriteString("\n}")
	return sb.String()
}

func (rd RegDiagramParsed) String() string {
	var sb strings.Builder
	sb.WriteString("Regdiagram{")
	sb.WriteString(indent("PsName: "+rd.PsName, 1))
	sb.WriteString(indent("Boxes:", 1))
	for _, b := range rd.Boxes {
		sb.WriteString(indent(b.String(), 2))
	}
	sb.WriteString(indent("--- Parsed Data ---", 1))
	sb.WriteString(indent(fmt.Sprintf("fixedBin: 0x%x", rd.fixedBin), 2))
	sb.WriteString(indent(fmt.Sprintf("mask: 0x%x", rd.mask), 2))
	sb.WriteString(indent("varBin:", 2))
	for k, v := range rd.varBin {
		sb.WriteString(indent(fmt.Sprintf("%s: [%d, %d]", k, v.lo, v.hi), 3))
	}
	sb.WriteString(indent(fmt.Sprintf("Parsed: %t", rd.Parsed), 2))
	sb.WriteString(indent("--- Parsed Data ---", 1))
	sb.WriteString("\n}")
	return sb.String()
}

func (b Box) String() string {
	var sb strings.Builder
	sb.WriteString("Box{")
	sb.WriteString(indent("HiBit: "+b.HiBit, 1))
	sb.WriteString(indent("Name: "+b.Name, 1))
	sb.WriteString(indent("Cs:", 1))
	for _, c := range b.Cs {
		sb.WriteString(indent(c.String(), 2))
	}
	sb.WriteString("\n}")
	return sb.String()
}

func (c C) String() string {
	var sb strings.Builder
	sb.WriteString("C{")
	sb.WriteString(indent("Value: "+c.Value, 1))
	sb.WriteString(indent("ColSpan: "+c.ColSpan, 1))
	sb.WriteString("\n}")
	return sb.String()
}

func (e EncodingParsed) String() string {
	var sb strings.Builder
	sb.WriteString("Encoding{")
	sb.WriteString(indent("Name: "+e.Name, 1))
	sb.WriteString(indent("DocVars:", 1))
	for _, dv := range e.DocVars {
		sb.WriteString(indent(dv.String(), 2))
	}
	sb.WriteString(indent("Boxes:", 1))
	for _, b := range e.Boxes {
		sb.WriteString(indent(b.String(), 2))
	}
	sb.WriteString(indent("Asmtemplate: "+e.AsmTemplate.String(), 1))

	sb.WriteString(indent("--- Parsed Data ---", 1))
	sb.WriteString(indent(fmt.Sprintf("Binary: 0x%x", e.Binary), 2))
	sb.WriteString(indent(fmt.Sprintf("mask: 0x%x", e.mask), 2))
	sb.WriteString(indent("Asm: "+e.Asm, 2))
	sb.WriteString(indent("GoOp: "+e.GoOp, 2))
	sb.WriteString(indent("arm64Op: "+e.arm64Op, 2))
	sb.WriteString(indent("class: "+e.class.String(), 2))
	sb.WriteString(indent(fmt.Sprintf("invalid: %t", e.invalid), 2))
	sb.WriteString(indent(fmt.Sprintf("Alias: %t", e.Alias), 2))
	sb.WriteString(indent("prefix: "+e.prefix, 2))
	sb.WriteString(indent("Operands:", 2))
	for _, op := range e.Operands {
		sb.WriteString(indent(op.String(), 3))
	}
	sb.WriteString(indent("Parsed: "+fmt.Sprintf("%t", e.Parsed), 2))
	sb.WriteString(indent("--- Parsed Data ---", 1))
	sb.WriteString("\n}")
	return sb.String()
}

func (at AsmTemplate) String() string {
	var sb strings.Builder
	sb.WriteString("Asmtemplate{")
	sb.WriteString(indent("TextA:", 1))
	for _, ta := range at.TextA {
		sb.WriteString(indent(ta.String(), 2))
	}
	sb.WriteString("\n}")
	return sb.String()
}

func (op Operand) String() string {
	var sb strings.Builder
	sb.WriteString("Operand{")
	sb.WriteString(indent("name: "+op.Name, 1))
	sb.WriteString(indent("typ: "+op.Typ, 1))
	sb.WriteString(indent("elems:", 1))
	for _, elem := range op.Elems {
		sb.WriteString(indent(elem.String(), 2))
	}
	for _, v := range op.constraints {
		sb.WriteString(indent(v, 2))
	}
	sb.WriteString("\n}")
	return sb.String()
}

func (e Element) String() string {
	var sb strings.Builder
	sb.WriteString("Element{")
	sb.WriteString(indent("encodedIn: "+e.encodedIn, 1))
	sb.WriteString(indent("textExp: "+e.textExp, 1))
	sb.WriteString(indent("textExpWithRanges: "+e.TextExpWithRanges, 1))
	sb.WriteString(indent("symbol: "+e.symbol, 1))
	sb.WriteString(indent(fmt.Sprintf("fixedArng: %s", e.fixedArng), 1))
	sb.WriteString(indent(fmt.Sprintf("fixedLSL: %s", e.fixedLSL), 1))
	sb.WriteString(indent(fmt.Sprintf("fixedSXTW: %t", e.fixedSXTW), 1))
	sb.WriteString(indent(fmt.Sprintf("fixedUXTW: %t", e.fixedUXTW), 1))
	sb.WriteString(indent(fmt.Sprintf("fixedModAmt: %s", e.fixedModAmt), 1))
	sb.WriteString(indent(fmt.Sprintf("fixedScalarWidth: %d", e.fixedScalarWidth), 1))
	sb.WriteString(indent(fmt.Sprintf("hasMod: %t", e.hasMod), 1))
	sb.WriteString(indent(fmt.Sprintf("isP: %t", e.isP), 1))
	sb.WriteString(indent(fmt.Sprintf("isZ: %t", e.isZ), 1))
	sb.WriteString("\n}")
	return sb.String()
}

func (ex Explanations) String() string {
	var sb strings.Builder
	sb.WriteString("Explanations{")
	sb.WriteString(indent("Scope: "+ex.Scope, 1))
	sb.WriteString(indent("Explanations:", 1))
	for _, e := range ex.Explanations {
		sb.WriteString(indent(e.String(), 2))
	}
	sb.WriteString("\n}")
	return sb.String()
}

func (e Explanation) String() string {
	var sb strings.Builder
	sb.WriteString("Explanation{")
	sb.WriteString(indent("Symbol: "+e.Symbol.String(), 1))
	sb.WriteString(indent("Account: "+e.Account.String(), 1))
	sb.WriteString(indent("Definition: "+e.Definition.String(), 1))
	sb.WriteString("\n}")
	return sb.String()
}

func (s Symbol) String() string {
	var sb strings.Builder
	sb.WriteString("Symbol{")
	sb.WriteString(indent("Value: "+s.Value, 1))
	sb.WriteString(indent("Link: "+s.Link, 1))
	sb.WriteString("\n}")
	return sb.String()
}

func (a Account) String() string {
	var sb strings.Builder
	sb.WriteString("Account{")
	sb.WriteString(indent("Encodedin: "+a.Encodedin, 1))
	sb.WriteString(indent("DocVars:", 1))
	for _, dv := range a.DocVars {
		sb.WriteString(indent(dv.String(), 2))
	}
	sb.WriteString(indent("Intro: "+a.Intro, 1))
	sb.WriteString("\n}")
	return sb.String()
}

func (d Definition) String() string {
	var sb strings.Builder
	sb.WriteString("Definition{")
	sb.WriteString(indent("Encodedin: "+d.Encodedin, 1))
	sb.WriteString(indent("Intro: "+d.Intro, 1))
	sb.WriteString(indent("Table:", 1))
	sb.WriteString(indent(d.Table.String(), 2))
	sb.WriteString("\n}")
	return sb.String()
}

func (t Table) String() string {
	var sb strings.Builder
	sb.WriteString("Table{")
	sb.WriteString(indent("Class: "+t.Class, 1))
	sb.WriteString(indent("TGroup:", 1))
	sb.WriteString(indent(t.TGroup.String(), 2))
	sb.WriteString("\n}")
	return sb.String()
}

func (tg TGroup) String() string {
	var sb strings.Builder
	sb.WriteString("TGroup{")
	sb.WriteString(indent("THead:", 1))
	sb.WriteString(indent(tg.THead.String(), 2))
	sb.WriteString(indent("TBody:", 1))
	sb.WriteString(indent(tg.TBody.String(), 2))
	sb.WriteString("\n}")
	return sb.String()
}

func (th THead) String() string {
	var sb strings.Builder
	sb.WriteString("THead{")
	sb.WriteString(indent("Row: "+th.Row.String(), 1))
	sb.WriteString("\n}")
	return sb.String()
}

func (tb TBody) String() string {
	var sb strings.Builder
	sb.WriteString("TBody{")
	sb.WriteString(indent("Rows:", 1))
	for _, r := range tb.Row {
		sb.WriteString(indent(r.String(), 2))
	}
	sb.WriteString("\n}")
	return sb.String()
}

func (r Row) String() string {
	var sb strings.Builder
	sb.WriteString("Row{")
	sb.WriteString(indent("Entries:", 1))
	for _, e := range r.Entries {
		sb.WriteString(indent(e.String(), 2))
	}
	sb.WriteString("\n}")
	return sb.String()
}

func (e Entry) String() string {
	var sb strings.Builder
	sb.WriteString("Entry{")
	sb.WriteString(indent("Value: "+e.Value, 1))
	sb.WriteString(indent("Class: "+e.Class, 1))
	sb.WriteString("\n}")
	return sb.String()
}

func (c class) String() string {
	switch c {
	case C_NONE:
		return "C_NONE"
	case C_SVE:
		return "C_SVE"
	case C_SVE2:
		return "C_SVE2"
	}
	return fmt.Sprintf("class(%d)", int(c))
}

func indent(s string, level int) string {
	prefix := strings.Repeat("  ", level)
	return "\n" + prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}

func (d Desc) String() string {
	var sb strings.Builder
	sb.WriteString("Desc{")
	sb.WriteString(indent("Brief:", 1))
	sb.WriteString(indent(d.Brief.String(), 2))
	sb.WriteString(indent("Authored:", 1))
	sb.WriteString(indent(d.Authored.String(), 2))
	sb.WriteString("\n}")
	return sb.String()
}

func (b Brief) String() string {
	var sb strings.Builder
	sb.WriteString("Brief{")
	sb.WriteString(indent("Para:", 1))
	for _, p := range b.Para {
		sb.WriteString(indent(p.String(), 2))
	}
	sb.WriteString("\n}")
	return sb.String()
}

func (a Authored) String() string {
	var sb strings.Builder
	sb.WriteString("Authored{")
	sb.WriteString(indent("Paragraphs:", 1))
	for _, p := range a.Paragraphs {
		sb.WriteString(indent(p.String(), 2))
	}
	sb.WriteString("\n}")
	return sb.String()
}

func (p Para) String() string {
	var sb strings.Builder
	sb.WriteString("Para{")
	sb.WriteString(indent("Text: "+p.Text, 1))
	sb.WriteString("\n}")
	return sb.String()
}

func (ps PsSection) String() string {
	var sb strings.Builder
	sb.WriteString("PsSection{")
	sb.WriteString(indent("Ps:", 1))
	for _, p := range ps.Ps {
		sb.WriteString(indent(p.String(), 2))
	}
	sb.WriteString("\n}")
	return sb.String()
}

func (p Ps) String() string {
	var sb strings.Builder
	sb.WriteString("Ps{")
	for _, t := range p.PSText {
		sb.WriteString(indent("PSText: "+t, 1))
	}
	sb.WriteString("\n}")
	return sb.String()
}

func (ta TextA) String() string {
	var sb strings.Builder
	sb.WriteString("TextA{")
	sb.WriteString(indent("Value: "+ta.Value, 1))
	if ta.Link != "" {
		sb.WriteString(indent("Link: "+ta.Link, 1))
	}
	sb.WriteString("\n}")
	return sb.String()
}
