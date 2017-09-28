// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// ytabMap maps keys generated with keyFromInsts to ytab identifiers.
var ytabMap = map[string]ytabID{
	"": "yvex",

	// 1 form:
	"m":           "yvex_m",
	"y/m,x":       "yvex_y2",
	"x/m,r":       "yvex_vcvtsd2si",
	"m,y":         "yvex_vbroadcastf",
	"m,x;m,y":     "yvex_mxy",
	"x,x":         "yvex_xx2",
	"x/m,x":       "yvex_x2",
	"x/m,xV,x":    "yvex_x3",
	"x,xV,x":      "yvex_xx3",
	"y/m,yV,y":    "yvex_yy3",
	"r/m,rV":      "yvex_r2",
	"r/m,rV,r":    "yvex_r3",
	"r/m,xV,x":    "yvex_rx3",
	"rV,r/m,r":    "yvex_vmr3",
	"i8,r/m,r":    "yvex_ri3",
	"i8,x/m,x":    "yvex_xi3",
	"i8,x,r/m":    "yvex_vpextr",
	"i8,y,x/m":    "yvex_yi3",
	"i8,y/m,y":    "yvex_vpermpd",
	"i8,r/m,xV,x": "yvex_rxi4",
	"i8,x/m,xV,x": "yvex_xxi4",
	"i8,x/m,yV,y": "yvex_xyi4",
	"i8,y/m,yV,y": "yvex_yyi4",

	// 2 forms:
	"m,y;x,y":                   "yvex_vpbroadcast_sd",
	"i8,x,r;i8,x,r/m":           "yvex_vpextrw",
	"i8,y,x/m;i8,x,x/m":         "yvex_vcvtps2ph",
	"i8,x/m,x;i8,y/m,y":         "yvex_xyi3",
	"i8,x/m,xV,x;i8,y/m,yV,y":   "yvex_vpalignr",
	"i8,x,xV;i8,y,yV":           "yvex_shift_dq",
	"x/m,xV,x;y/m,yV,y":         "yvex_xy3",
	"x/m,xV,x;i8,x,xV":          "yvex_shift",
	"x/m,x;x/m,y":               "yvex_vpbroadcast",
	"x/m,x;y/m,y":               "yvex_xy2",
	"x,m;y,m":                   "yvex_vmovntdq",
	"x,r/m;r/m,x":               "yvex_vmovd",
	"x,r;y,r":                   "yvex_xyr2",
	"x,m;m,xV,x":                "yvex_vmovhpd",
	"xIH,x/m,xV,x;yIH,y/m,yV,y": "yvex_xy4",

	// 4 forms:
	"m,x;x,x;m,y;x,y":                     "yvex_vpbroadcast_ss",
	"x,m;m,x;x,xV,x;x,xV,x":               "yvex_vmov",
	"x,xV,m;y,yV,m;m,xV,x;m,yV,y":         "yvex_vblendvpd",
	"x/m,x;x,x/m;y/m,y;y,y/m":             "yvex_vmovdqa",
	"x/m,xV,x;i8,x,xV;x/m,yV,y;i8,y,yV":   "yvex_vps",
	"i8,x/m,x;x/m,xV,x;i8,y/m,y;y/m,yV,y": "yvex_vpermilp",

	// 5 forms:
	"x,r/m;m,x;r/m,x;x,x;x,x/m": "yvex_vmovq",
}

// precomputedOptabs is used to emit some optabs that can not be
// generated with normal execution path.
var precomputedOptabs = map[string]optab{
	// This is added to avoid backwards-incompatible change.
	//
	// initially, yvex_xyi3 was added with Yi8 args.
	// Later, it was decided to make it Yu8, but Yi8 forms
	// were preserved as well.
	// So, 4 ytabs instead of 2.
	"VPSHUFD": {
		"VPSHUFD",
		"yvex_xyi3",
		[]string{
			"vexNOVSR | vex128 | vex66 | vex0F | vexWIG", "0x70",
			"vexNOVSR | vex256 | vex66 | vex0F | vexWIG", "0x70",
			"vexNOVSR | vex128 | vex66 | vex0F | vexWIG", "0x70",
			"vexNOVSR | vex256 | vex66 | vex0F | vexWIG", "0x70",
		},
	},

	// Instructions that can not be constructed from
	// "x86.csv" because it only have 2/4 forms.
	"VPSRLQ": {
		"VPSRLQ",
		"yvex_shift",
		[]string{
			"vexNDD | vex128 | vex66 | vex0F | vexWIG", "0x73", "0xD0",
			"vexNDD | vex256 | vex66 | vex0F | vexWIG", "0x73", "0xD0",
			"vexNDS | vex128 | vex66 | vex0F | vexWIG", "0xD3",
			"vexNDS | vex256 | vex66 | vex0F | vexWIG", "0xD3",
		},
	},
	"VPSLLQ": {
		"VPSLLQ",
		"yvex_shift",
		[]string{
			"vexNDD | vex128 | vex66 | vex0F | vexWIG", "0x73", "0xF0",
			"vexNDD | vex256 | vex66 | vex0F | vexWIG", "0x73", "0xF0",
			"vexNDS | vex128 | vex66 | vex0F | vexWIG", "0xF3",
			"vexNDS | vex256 | vex66 | vex0F | vexWIG", "0xF3",
		},
	},
	"VPSLLD": {
		"VPSLLD",
		"yvex_shift",
		[]string{
			"vexNDS | vex128 | vex66 | vex0F | vexWIG", "0x72", "0xF0",
			"vexNDS | vex256 | vex66 | vex0F | vexWIG", "0x72", "0xF0",
			"vexNDD | vex128 | vex66 | vex0F | vexWIG", "0xF2",
			"vexNDD | vex256 | vex66 | vex0F | vexWIG", "0xF2",
		},
	},
	"VPSRLD": {
		"VPSRLD",
		"yvex_shift",
		[]string{
			"vexNDD | vex128 | vex66 | vex0F | vexWIG", "0x72", "0xD0",
			"vexNDD | vex256 | vex66 | vex0F | vexWIG", "0x72", "0xD0",
			"vexNDD | vex128 | vex66 | vex0F | vexWIG", "0xD2",
			"vexNDD | vex256 | vex66 | vex0F | vexWIG", "0xD2",
		},
	},

	// Thease are here due to adhoc encoded
	// ModR/M opcode extension.
	"VPSLLDQ": {
		"VPSLLDQ",
		"yvex_shift_dq",
		[]string{
			"vexNDD | vex128 | vex66 | vex0F | vexWIG", "0x73", "0xF8",
			"vexNDD | vex256 | vex66 | vex0F | vexWIG", "0x73", "0xF8",
		},
	},
	"VPSRLDQ": {
		"VPSRLDQ",
		"yvex_shift_dq",
		[]string{
			"vexNDD | vex128 | vex66 | vex0F | vexWIG", "0x73", "0xD8",
			"vexNDD | vex256 | vex66 | vex0F | vexWIG", "0x73", "0xD8",
		},
	},
	"VPSLLW": {
		"VPSLLW",
		"yvex_vps",
		[]string{
			"vexNDS | vex128 | vex66 | vex0F | vexWIG", "0xF1",
			"vexNDD | vex128 | vex66 | vex0F | vexWIG", "0x71", "0xF0",
			"vexNDS | vex256 | vex66 | vex0F | vexWIG", "0xF1",
			"vexNDD | vex256 | vex66 | vex0F | vexWIG", "0x71", "0xF0",
		},
	},
	"VPSRAD": {
		"VPSRAD",
		"yvex_vps",
		[]string{
			"vexNDS | vex128 | vex66 | vex0F | vexWIG", "0xE2",
			"vexNDD | vex128 | vex66 | vex0F | vexWIG", "0x72", "0xE0",
			"vexNDS | vex256 | vex66 | vex0F | vexWIG", "0xE2",
			"vexNDD | vex256 | vex66 | vex0F | vexWIG", "0x72", "0xE0",
		},
	},
	"VPSRAW": {
		"VPSRAW",
		"yvex_vps",
		[]string{
			"vexNDS | vex128 | vex66 | vex0F | vexWIG", "0xE1",
			"vexNDD | vex128 | vex66 | vex0F | vexWIG", "0x71", "0xE0",
			"vexNDS | vex256 | vex66 | vex0F | vexWIG", "0xE1",
			"vexNDD | vex256 | vex66 | vex0F | vexWIG", "0x71", "0xE0",
		},
	},
	"VPSRLW": {
		"VPSRLW",
		"yvex_vps",
		[]string{
			"vexNDS | vex128 | vex66 | vex0F | vexWIG", "0xD1",
			"vexNDD | vex128 | vex66 | vex0F | vexWIG", "0x71", "0xD0",
			"vexNDS | vex256 | vex66 | vex0F | vexWIG", "0xD1",
			"vexNDD | vex256 | vex66 | vex0F | vexWIG", "0x71", "0xD0",
		},
	},
}
