// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "testing"

func TestEncodingValue(t *testing.T) {
	tests := []struct {
		expr string
		n    int
		want uint64
	}{
		{"0b010", 0, 0b010},
		{"0b010:m[3]", 8, 0b0101},
		{"0b1:n[1:0]", 2, 0b110},
		{"m[2:0]", 5, 0b101},
		{"m[2:0]:0b0", 5, 0b1010},
	}
	for _, test := range tests {
		got, err := encodingValue(test.expr, test.n)
		if err != nil {
			t.Errorf("encodingValue(%q, %d): %v", test.expr, test.n, err)
			continue
		}
		if got != test.want {
			t.Errorf("encodingValue(%q, %d) = %#b, want %#b", test.expr, test.n, got, test.want)
		}
	}
}

func TestEncodingValueError(t *testing.T) {
	for _, expr := range []string{"0b00x", "m[", "m[0:1]", "op1[2:0]"} {
		if _, err := encodingValue(expr, 0); err == nil {
			t.Errorf("encodingValue(%q, 0) succeeded, want error", expr)
		}
	}
}
