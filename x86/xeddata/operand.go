// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xeddata

import (
	"errors"
	"strings"
)

// OperandVisibility describes operand visibility in XED terms.
type OperandVisibility int

const (
	// VisExplicit is a default operand visibility.
	// Explicit operand is "real" kind of operands that
	// is shown in syntax and can be specified by the programmer.
	VisExplicit OperandVisibility = iota

	// VisImplicit is for fixed arg (like EAX); usually shown in syntax.
	VisImplicit

	// VisSuppressed is like VisImplicit, but not shown in syntax.
	// In some very rare exceptions, they are also shown in syntax string.
	VisSuppressed

	// VisEcond is encoder-only conditions. Can be ignored.
	VisEcond
)

// Operand holds data that is encoded inside
// instruction's "OPERANDS" field.
//
// Use NewOperand function to decode operand fields into Operand object.
type Operand struct {
	// Name is an ID with optional nonterminal name part.
	//
	// Possible values: "REG0=GPRv_B", "REG1", "MEM0", ...
	//
	// If nonterminal part is present, name
	// can be split into LHS and RHS with NonTerminalName method.
	Name string

	// Action describes argument types.
	//
	// Possible values: "r", "w", "rw", "cr", "cw", "crw", "rcw".
	// The "c" prefix before "r" or "w" represents conditional read or write.
	Action string

	// Width descriptor. It can express simple width like "w" (word, 16bit)
	// or meta-width like "v", which corresponds to {16, 32, 64} bits.
	//
	// The first column in all-widths.txt lists all possible widths.
	//
	// To deterine the size given a width string and a mode, use
	// [Database.WidthSize].
	//
	// Possible values: "", "q", "ds", "dq", ...
	// Optional.
	Width string

	// Xtype holds XED-specific type information.
	//
	// Possible values: "", "f64", "i32", ...
	// Optional.
	Xtype string

	// Attributes serves as container for all other properties.
	//
	// Possible values:
	//   EVEX.b context {
	//     TXT=ZEROSTR  - zeroing
	//     TXT=SAESTR   - suppress all exceptions
	//     TXT=ROUNDC   - rounding
	//     TXT=BCASTSTR - broadcasting
	//   }
	//   MULTISOURCE4 - 4FMA multi-register operand.
	//
	// Optional. For most operands, it's nil.
	Attributes map[string]bool

	// Visibility tells if operand is explicit, implicit or suspended.
	Visibility OperandVisibility
}

var xedVisibilities = map[string]OperandVisibility{
	"EXPL":  VisExplicit,
	"IMPL":  VisImplicit,
	"SUPP":  VisSuppressed,
	"ECOND": VisEcond,
}

// NewOperand decodes operand string.
//
// See "$XED/pysrc/opnds.py" to learn about fields format
// and valid combinations.
//
// Requires database with xtypes, widths, and extraWidths info.
func NewOperand(db *Database, s string) (*Operand, error) {
	if db.widths == nil {
		return nil, errors.New("Database.widths is nil")
	}
	if db.extraWidths == nil {
		return nil, errors.New("Database.extraWidths is nil")
	}

	fields := strings.Split(s, ":")
	switch len(fields) {
	case 0:
		return nil, errors.New("empty operand fields string")
	case 1:
		return &Operand{Name: fields[0]}, nil
	}
	var op Operand

	// First two fields are fixed.
	op.Name = fields[0]
	op.Action = fields[1]

	// Optional fields.
	var w string
	for _, f := range fields[2:] {
		if db.widths[f] != nil && w == "" {
			w = f
		} else if vis, ok := xedVisibilities[f]; ok {
			op.Visibility = vis
		} else if xtype := db.xtypes[f]; xtype != nil {
			op.Xtype = f
		} else {
			if op.Attributes == nil {
				op.Attributes = make(map[string]bool)
			}
			op.Attributes[f] = true
		}
	}

	// Get default width from operand type.
	if w == "" {
		if op.NonterminalName() {
			if strings.HasPrefix(op.NameLHS(), "REG") {
				rhs := op.NameRHS()
				if strings.HasPrefix(rhs, "XED_REG_") {
					// Register
					w = db.extraWidths[rhs]
				} else if strings.HasSuffix(rhs, "()") {
					// Non-terminal
					w = db.extraWidths[rhs]
				}
			}
		} else {
			// Try as an immediate.
			w = db.extraWidths[op.Name]
		}
	}

	if w != "" {
		op.Width = w
		// If operand did not specify an xtype, get the default from the width
		if op.Xtype == "" && db.widths[w] != nil {
			op.Xtype = db.widths[w].xtype
		}
	}

	return &op, nil
}

// NonterminalName returns true if op.Name consist
// of LHS and RHS parts.
//
// RHS is non-terminal name lookup function expression.
// Example: "REG0=GPRv()" has "GPRv()" name lookup function.
func (op *Operand) NonterminalName() bool {
	return strings.Contains(op.Name, "=")
}

// NameLHS returns left hand side part of the non-terminal name.
// Example: NameLHS("REG0=GPRv()") => "REG0".
func (op *Operand) NameLHS() string {
	lhs, _, _ := strings.Cut(op.Name, "=")
	return lhs
}

// NameRHS returns right hand side part of the non-terminal name.
// Example: NameLHS("REG0=GPRv()") => "GPRv()".
func (op *Operand) NameRHS() string {
	_, rhs, _ := strings.Cut(op.Name, "=")
	return rhs
}

// IsVisible returns true for operands that are usually
// shown in syntax strings.
func (op *Operand) IsVisible() bool {
	return op.Visibility == VisExplicit ||
		op.Visibility == VisImplicit
}
