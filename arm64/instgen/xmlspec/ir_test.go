// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xmlspec

import (
	"encoding/xml"
	"reflect"
	"testing"
)

// The pseudocode is markup, with its links as elements and its operators
// escaped.
func TestIRForIclassDecode(t *testing.T) {
	doc := `<iclass name="SVE"><ps_section howmany="1"><ps name="x" sections="1" secttype="noheading">` +
		`<pstext mayhavelinks="1" section="Decode">if !IsFeatureImplemented(FEAT_SVE) &amp;&amp; size == '00' then
    EndOfDecode(<a link="enum_Decode_UNDEF" file="shared_pseudocode.xml">Decode_UNDEF</a>);
end;
let esize : integer{} = 16 &lt;&lt; <a link="f">HighestSetBitNZ</a>(tsize);</pstext>` +
		`</ps></ps_section></iclass>`
	var ic Iclass
	if err := xml.Unmarshal([]byte(doc), &ic); err != nil {
		t.Fatal(err)
	}
	want := "if !IsFeatureImplemented(FEAT_SVE) && size == '00' then\n" +
		"    EndOfDecode(Decode_UNDEF);\n" +
		"end;\n" +
		"let esize : integer{} = 16 << HighestSetBitNZ(tsize);\n"
	v := IRForIclass(ic)
	if v.Decode != want {
		t.Errorf("Decode = %q, want %q", v.Decode, want)
	}
	if v.Name != "SVE" {
		t.Errorf("Name = %q, want SVE", v.Name)
	}
}

func TestIRForIclassFields(t *testing.T) {
	ic := Iclass{
		Name: "x",
		RegDiagram: RegDiagramParsed{
			varBin: map[string]bitRange{"size": {lo: 22, hi: 24}, "Zn": {lo: 5, hi: 10}},
		},
	}
	v := IRForIclass(ic)
	want := map[string]IRField{"size": {22, 24}, "Zn": {5, 10}}
	if !reflect.DeepEqual(v.Fields, want) {
		t.Errorf("Fields = %v, want %v", v.Fields, want)
	}
	if v.Decode != "" {
		t.Errorf("an iclass with no pseudocode has Decode %q", v.Decode)
	}
}
