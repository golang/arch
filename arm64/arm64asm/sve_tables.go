// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package arm64asm

// The tables that sverun.go reads. They are empty here, so Decode accepts no SVE
// instruction until a generated set replaces this file. Tests install small
// hand-written tables by assigning to these variables.
var (
	// sveOpARM is the mnemonic of each op as the ARM manual spells it, which
	// is what [Op.String] returns and what [GNUSyntax] prints.
	sveOpARM []string

	// sveOpGo is the mnemonic of each op as the Go assembler spells it. It
	// adds a register file prefix and, for some ops, a pinned arrangement, so
	// it cannot be derived from [sveOpARM].
	sveOpGo []string

	// sveSyms are the spellings that ARM's value tables name, interned. A
	// descriptor refers to one by index, either as a pinned value or as the
	// result of looking a field up in its table.
	sveSyms []string

	// sveDescs are the parts of the operands: a register, an arrangement, a
	// qualifier, an immediate. See [sveDesc] for the knobs.
	sveDescs []sveDesc

	// sveArgs are the operands. Each has a kind, which chooses the [Arg] it
	// becomes, and the descriptors that supply its parts, as indexes into
	// [sveDescs].
	sveArgs []sveArg

	// sveArgLists names the operands of each form, in ARM manual order, as
	// indexes into [sveArgs].
	sveArgLists [][]uint16

	// sveForms has one row per encoding, most specific mask first, so the
	// first row whose [sveForm.mask] and [sveForm.value] match and whose
	// operands all resolve wins. [sveForm.op] indexes [sveOpARM] and
	// [sveOpGo], [sveForm.args] indexes [sveArgLists], and [sveForm.gpr] is
	// 32 or 64 when the form fixes the width of its <R> register.
	sveForms []sveForm
)
