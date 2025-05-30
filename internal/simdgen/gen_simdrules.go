// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sort"
)

const simdrulesTmpl = `// Code generated by x/arch/internal/simdgen using 'go run . -xedPath $XED_PATH -o godefs -goroot $GOROOT go.yaml types.yaml categories.yaml'; DO NOT EDIT.

// The AVX instruction encodings orders vector register from right to left, for example:
// VSUBPS X Y Z means Z=Y-X
// The rules here swapped the order of such X and Y because the ssa to prog lowering in simdssa.go assumes a
// left to right order.
// TODO: we should offload the logic to simdssa.go, instead of here.
//
// Masks are always at the end, immediates always at the beginning.

{{- range .Ops }}
({{.Op.Go}}{{(index .Op.In 0).Go}} {{.Args}}) => ({{.Op.Asm}} {{.ReverseArgs}})
{{- end }}
{{- range .OpsImm }}
({{.Op.Go}}{{(index .Op.In 1).Go}} {{.Args}}) => ({{.Op.Asm}} [{{(index .Op.In 0).Const}}] {{.ReverseArgs}})
{{- end }}
{{- range .OpsMask}}
({{.Op.Go}}{{(index .Op.In 0).Go}} {{.Args}} mask) => ({{.Op.Asm}} {{.ReverseArgs}} (VPMOVVec{{(index .Op.In 0).ElemBits}}x{{(index .Op.In 0).Lanes}}ToM <types.TypeMask> mask))
{{- end }}
{{- range .OpsImmMask}}
({{.Op.Go}}{{(index .Op.In 1).Go}} {{.Args}} mask) => ({{.Op.Asm}} [{{(index .Op.In 0).Const}}] {{.ReverseArgs}} (VPMOVVec{{(index .Op.In 1).ElemBits}}x{{(index .Op.In 1).Lanes}}ToM <types.TypeMask> mask))
{{- end }}
{{- range .OpsMaskOut}}
({{.Op.Go}}{{(index .Op.In 0).Go}} {{.Args}}) => (VPMOVMToVec{{(index .Op.In 0).ElemBits}}x{{(index .Op.In 0).Lanes}} ({{.Op.Asm}} {{.ReverseArgs}}))
{{- end }}
{{- range .OpsImmInMaskOut}}
({{.Op.Go}}{{(index .Op.In 1).Go}} {{.Args}}) => (VPMOVMToVec{{(index .Op.In 1).ElemBits}}x{{(index .Op.In 1).Lanes}} ({{.Op.Asm}} [{{(index .Op.In 0).Const}}] {{.ReverseArgs}}))
{{- end }}
{{- range .OpsMaskInMaskOut}}
({{.Op.Go}}{{(index .Op.In 0).Go}} {{.Args}} mask) => (VPMOVMToVec{{(index .Op.In 0).ElemBits}}x{{(index .Op.In 0).Lanes}} ({{.Op.Asm}} {{.ReverseArgs}} (VPMOVVec{{(index .Op.In 0).ElemBits}}x{{(index .Op.In 0).Lanes}}ToM <types.TypeMask> mask)))
{{- end }}
{{- range .OpsImmMaskInMaskOut}}
({{.Op.Go}}{{(index .Op.In 1).Go}} {{.Args}} mask) => (VPMOVMToVec{{(index .Op.In 1).ElemBits}}x{{(index .Op.In 1).Lanes}} ({{.Op.Asm}} [{{(index .Op.In 0).Const}}] {{.ReverseArgs}} (VPMOVVec{{(index .Op.In 1).ElemBits}}x{{(index .Op.In 1).Lanes}}ToM <types.TypeMask> mask)))
{{- end }}
`

// writeSIMDRules generates the lowering and rewrite rules for ssa and writes it to simdAMD64.rules
// within the specified directory.
func writeSIMDRules(directory string, ops []Operation) error {
	file, t, err := openFileAndPrepareTemplate(directory, "src/cmd/compile/internal/ssa/_gen/simdAMD64.rules", simdrulesTmpl)
	if err != nil {
		return err
	}
	defer file.Close()
	type OpAndArgList struct {
		Op          Operation
		Args        string // "x y", does not include masks
		ReverseArgs string // "y x", does not include masks
	}
	Ops := make([]OpAndArgList, 0)
	OpsImm := make([]OpAndArgList, 0)
	OpsMask := make([]OpAndArgList, 0)
	OpsImmMask := make([]OpAndArgList, 0)
	OpsMaskOut := make([]OpAndArgList, 0)
	OpsImmInMaskOut := make([]OpAndArgList, 0)
	OpsMaskInMaskOut := make([]OpAndArgList, 0)
	OpsImmMaskInMaskOut := make([]OpAndArgList, 0)

	for _, op := range ops {
		opInShape, opOutShape, maskType, _, op, gOp, err := op.shape()
		if err != nil {
			return err
		}
		vregInCnt := len(gOp.In)
		if maskType == OneMask {
			op.Asm += "Masked"
			vregInCnt--
		}
		op.Asm = fmt.Sprintf("%s%d", op.Asm, *op.Out[0].Bits)
		opData := OpAndArgList{Op: op}
		if vregInCnt == 1 {
			opData.Args = "x"
			opData.ReverseArgs = "x"
		} else if vregInCnt == 2 {
			opData.Args = "x y"
			opData.ReverseArgs = "y x"
		} else {
			return fmt.Errorf("simdgen does not support more than 2 vreg in inputs")
		}
		// If class overwrite is happening, that's not really a mask but a vreg.
		if opOutShape == OneVregOut || op.Out[0].OverwriteClass != nil {
			switch opInShape {
			case PureVregIn:
				Ops = append(Ops, opData)
			case OneKmaskIn:
				OpsMask = append(OpsMask, opData)
			case OneConstImmIn:
				OpsImm = append(OpsImm, opData)
			case OneKmaskConstImmIn:
				OpsImmMask = append(OpsImmMask, opData)
			case PureKmaskIn:
				return fmt.Errorf("simdgen does not support pure k mask instructions, they should be generated by compiler optimizations")
			}
		} else {
			// OneKmaskOut case
			switch opInShape {
			case PureVregIn:
				OpsMaskOut = append(OpsMaskOut, opData)
			case OneKmaskIn:
				OpsMaskInMaskOut = append(OpsMaskInMaskOut, opData)
			case OneConstImmIn:
				OpsImmInMaskOut = append(OpsImmInMaskOut, opData)
			case OneKmaskConstImmIn:
				OpsImmMaskInMaskOut = append(OpsImmMaskInMaskOut, opData)
			case PureKmaskIn:
				return fmt.Errorf("simdgen does not support pure k mask instructions, they should be generated by compiler optimizations")
			}
		}
	}
	sortKey := func(op *OpAndArgList) string {
		return *op.Op.In[0].Go + op.Op.Go
	}
	sortBySortKey := func(ops []OpAndArgList) {
		sort.Slice(ops, func(i, j int) bool {
			return sortKey(&ops[i]) < sortKey(&ops[j])
		})
	}
	sortBySortKey(Ops)
	sortBySortKey(OpsImm)
	sortBySortKey(OpsMask)
	sortBySortKey(OpsImmMask)
	sortBySortKey(OpsMaskOut)
	sortBySortKey(OpsImmInMaskOut)
	sortBySortKey(OpsMaskInMaskOut)
	sortBySortKey(OpsImmMaskInMaskOut)

	type templateData struct {
		Ops                 []OpAndArgList
		OpsImm              []OpAndArgList
		OpsMask             []OpAndArgList
		OpsImmMask          []OpAndArgList
		OpsMaskOut          []OpAndArgList
		OpsImmInMaskOut     []OpAndArgList
		OpsMaskInMaskOut    []OpAndArgList
		OpsImmMaskInMaskOut []OpAndArgList
	}

	err = t.Execute(file, templateData{
		Ops,
		OpsImm,
		OpsMask,
		OpsImmMask,
		OpsMaskOut,
		OpsImmInMaskOut,
		OpsMaskInMaskOut,
		OpsImmMaskInMaskOut})
	if err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
