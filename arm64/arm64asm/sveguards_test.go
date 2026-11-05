// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package arm64asm

import (
	"math/rand"
	"testing"
)

func decodeGNU(w uint32) (string, error) {
	inst, err := Decode([]byte{byte(w), byte(w >> 8), byte(w >> 16), byte(w >> 24)})
	if err != nil {
		return "", err
	}
	return GNUSyntax(inst), nil
}

// The classes are numbered from 1 and every one of them is some form's.
func TestSVEGuardsClasses(t *testing.T) {
	if sveReject == nil {
		t.Fatal("sveReject is not installed")
	}
	if sveRejected(0, 0xffffffff) {
		t.Error("sveRejected rejects class 0, which means no constraint")
	}
	n := 0
	for _, f := range sveForms {
		n = max(n, int(f.reject))
	}
	used := make([]bool, n+1)
	for _, f := range sveForms {
		used[f.reject] = true
	}
	for c := 1; c <= n; c++ {
		if !used[c] {
			t.Errorf("no form has reject class %d", c)
		}
	}
	if n == 0 {
		t.Error("no form has a reject class")
	}
}

// Each word below matches the encoding of a form and is declared invalid by the
// decode pseudocode of the ARM manual, for a reason the dispatch table cannot
// see because it is in a variable field. Without the constraints the decoder
// prints it as if it were an instruction.
func TestSVEGuardsReject(t *testing.T) {
	tests := []struct {
		bad, good uint32
		goodText  string
		why       string
	}{
		{0x2520e48e, 0x2520c48e, "add z14.b, z14.b, #36", "a shifted immediate needs a size above '00'"},
		{0x4564a267, 0x4524a267, "histseg z7.b, z19.b, z4.b", "HISTSEG has only the byte size: size != '00'"},
		{0x45926b0a, 0x45126b0a, "pmullb z10.q, z24.d, z18.d", "the size field with size::sh == '001' is reserved"},
		{0x841fde84, 0x840fde84, "prfb pldl3keep, p7, [x20,x15]", "Rm == '11111' is reserved in a scalar-plus-scalar prefetch"},
	}
	for _, tc := range tests {
		if got, err := decodeGNU(tc.good); err != nil || got != tc.goodText {
			t.Errorf("Decode(%#08x) = %q, %v; want %q", tc.good, got, err, tc.goodText)
		}
		if got, err := decodeGNU(tc.bad); err == nil {
			t.Errorf("Decode(%#08x) = %q, want an error (%s)", tc.bad, got, tc.why)
		}
		// The word is declined because of the constraint and nothing else.
		old := sveReject
		sveReject = nil
		_, err := decodeGNU(tc.bad)
		sveReject = old
		if err != nil {
			t.Errorf("Decode(%#08x) without the constraints = %v, want it decoded (%s)", tc.bad, err, tc.why)
		}
	}
}

// A constraint takes out some words of a form, not all of them: a form that every
// word is rejected from would be dead, and the oracle would still not see it if
// its pins happened to be absent.
func TestSVEGuardsLeaveEveryFormAlive(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	rejecting := 0
	for i, f := range sveForms {
		alive, rejects := false, false
		for k := 0; k < 4096 && !(alive && rejects); k++ {
			if f.reject != 0 && sveRejected(int(f.reject), f.value|rng.Uint32()&^f.mask) {
				rejects = true
			} else {
				alive = true
			}
		}
		if !alive {
			t.Errorf("form %d (%s) rejects every word it matches", i, sveOpARM[f.op])
		}
		if rejects {
			rejecting++
		}
	}
	// About a quarter of the forms have a constraint; allow the table to move.
	if rejecting < 100 {
		t.Errorf("only %d forms reject any word; the constraints are not doing their work", rejecting)
	}
}
