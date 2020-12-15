// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ppc64asm

import (
	"testing"
)

func panicOrNot(f func()) (panicked bool) {
	defer func() {
		if err := recover(); err != nil {
			panicked = true
		}
	}()
	f()
	return false
}

func TestBitField(t *testing.T) {
	var tests = []struct {
		b    BitField
		i    uint32 // input
		u    uint32 // unsigned output
		s    int32  // signed output
		fail bool   // if the check should panic
	}{
		{BitField{0, 0, 0}, 0, 0, 0, true},
		{BitField{31, 2, 0}, 0, 0, 0, true},
		{BitField{31, 1, 0}, 1, 1, -1, false},
		{BitField{29, 2, 0}, 0 << 1, 0, 0, false},
		{BitField{29, 2, 0}, 1 << 1, 1, 1, false},
		{BitField{29, 2, 0}, 2 << 1, 2, -2, false},
		{BitField{29, 2, 0}, 3 << 1, 3, -1, false},
		{BitField{0, 32, 0}, 1<<32 - 1, 1<<32 - 1, -1, false},
		{BitField{16, 3, 0}, 1 << 15, 4, -4, false},
	}
	for i, tst := range tests {
		var (
			ou uint32
			os int32
		)
		failed := panicOrNot(func() {
			ou = tst.b.Parse([2]uint32{tst.i})
			os = tst.b.ParseSigned([2]uint32{tst.i})
		})
		if failed != tst.fail {
			t.Errorf("case %d: %v: fail test failed, got %v, expected %v", i, tst.b, failed, tst.fail)
			continue
		}
		if ou != tst.u {
			t.Errorf("case %d: %v.Parse(%d) returned %d, expected %d", i, tst.b, tst.i, ou, tst.u)
			continue
		}
		if os != tst.s {
			t.Errorf("case %d: %v.ParseSigned(%d) returned %d, expected %d", i, tst.b, tst.i, os, tst.s)
		}
	}
}

func TestBitFields(t *testing.T) {
	var tests = []struct {
		b    BitFields
		i    [2]uint32 // input
		u    uint64    // unsigned output
		s    int64     // signed output
		fail bool      // if the check should panic
	}{
		{BitFields{{0, 0, 1}}, [2]uint32{0, 0}, 0, 0, true},
		{BitFields{{31, 2, 1}}, [2]uint32{0, 0}, 0, 0, true},
		{BitFields{{31, 1, 1}}, [2]uint32{0, 1}, 1, -1, false},
		{BitFields{{29, 2, 1}}, [2]uint32{0, 0 << 1}, 0, 0, false},
		{BitFields{{29, 2, 1}}, [2]uint32{0, 1 << 1}, 1, 1, false},
		{BitFields{{29, 2, 1}}, [2]uint32{0, 2 << 1}, 2, -2, false},
		{BitFields{{29, 2, 1}}, [2]uint32{0, 3 << 1}, 3, -1, false},
		{BitFields{{0, 32, 1}}, [2]uint32{0, 1<<32 - 1}, 1<<32 - 1, -1, false},
		{BitFields{{16, 3, 1}}, [2]uint32{0, 1 << 15}, 4, -4, false},
		{BitFields{{16, 16, 0}, {16, 16, 1}}, [2]uint32{0x8016, 0x32}, 0x80160032, -0x7FE9FFCE, false},
		{BitFields{{14, 18, 0}, {16, 16, 1}}, [2]uint32{0x38016, 0x32}, 0x380160032, -0x07FE9FFCE, false},
	}
	for i, tst := range tests {
		var (
			ou uint64
			os int64
		)
		failed := panicOrNot(func() {
			ou = tst.b.Parse(tst.i)
			os = tst.b.ParseSigned(tst.i)
		})
		if failed != tst.fail {
			t.Errorf("case %d: %v: fail test failed, got %v, expected %v", i, tst.b, failed, tst.fail)
			continue
		}
		if ou != tst.u {
			t.Errorf("case %d: %v.Parse(%d) returned %d, expected %d", i, tst.b, tst.i, ou, tst.u)
			continue
		}
		if os != tst.s {
			t.Errorf("case %d: %v.ParseSigned(%d) returned %d, expected %d", i, tst.b, tst.i, os, tst.s)
		}
	}
}
