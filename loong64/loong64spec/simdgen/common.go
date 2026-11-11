// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simdgen

import (
	"strings"

	"github.com/ledongthuc/pdf"
)

func IsFirstPage(page pdf.Page) bool {
	content := page.Content()
	simdAppendixbPrefix := "LoongArchReferenceManualVolumeII-a:VectorInstructionExtensions"
	simdAppendixbSuffix := "AppendixB"
	simdAppendixbLen := len(simdAppendixbPrefix) + len(simdAppendixbSuffix) + 4
	ct := ""
	for _, t := range content.Text {
		ct += t.S
		if strings.HasSuffix(ct, simdAppendixbSuffix) {
			return true
		}
		if strings.HasPrefix(simdAppendixbPrefix, ct) || strings.HasPrefix(ct, simdAppendixbPrefix) {
			if len(ct) >= simdAppendixbLen {
				return false
			}
			continue
		} else {
			return false

		}
	}
	return false
}

func ExtrText(page pdf.Page, isFP bool) []pdf.Text {
	content := page.Content()

	var text []pdf.Text
	for _, t := range content.Text {
		text = append(text, t)
	}

	// page header(62), page num(3), table name(30), table header(64)
	if isFP {
		text = text[159:len(text)] // 62+3+30+64
	} else {
		text = text[129:len(text)] // 62+3+64
	}
	return text
}

func GetInst(chars []pdf.Text) (instWords []pdf.Text) {
	for i := 0; i < len(chars); {
		xRange := []float64{chars[i].X, chars[i].X}
		j := i + 1

		// Find all chars on one line.
		for j < len(chars) && chars[j].Y == chars[i].Y {
			xRange[1] = chars[j].X
			j++
		}

		// Split line into words (really, phrases).
		for k := i; k < j; {
			ck := &chars[k]
			s := ck.S
			var preX float64 = ck.X
			l := k + 1
			for ; l < j; l++ {
				cl := &chars[l]
				// Based on observation, if the distance between two characters exceeds 10,
				// they are considered not to belong to the same word.
				if cl.X-preX < 10 {
					s += cl.S
					preX = cl.X
					continue
				}
				if cl.X-preX >= 10 {
					f := ck.Font
					instWords = append(instWords, pdf.Text{
						Font:     f,
						FontSize: ck.FontSize,
						X:        ck.X,
						Y:        ck.Y,
						W:        preX - ck.X,
						S:        s,
					})
					s = cl.S
					preX = cl.X
					continue
				}
			}
			f := ck.Font
			instWords = append(instWords, pdf.Text{
				Font:     f,
				FontSize: ck.FontSize,
				X:        ck.X,
				Y:        ck.Y,
				W:        preX - ck.X,
				S:        s,
			})
			k = l
		}
		i = j
	}

	return instWords
}
