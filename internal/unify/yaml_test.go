// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unify

import (
	"bytes"
	"fmt"
	"iter"
	"log"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func mustParse(expr string) Closure {
	var c Closure
	if err := yaml.Unmarshal([]byte(expr), &c); err != nil {
		panic(err)
	}
	return c
}

func oneValue(t *testing.T, c Closure) *Value {
	t.Helper()
	var v *Value
	var i int
	for v = range c.All() {
		i++
	}
	if i != 1 {
		t.Fatalf("expected 1 value, got %d", i)
	}
	return v
}

func printYaml(val any) {
	b, err := yaml.Marshal(val)
	if err != nil {
		panic(err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal(b, &node); err != nil {
		panic(err)
	}

	// Map lines to start offsets. We'll use this to figure out when nodes are
	// "small" and should use inline style.
	lines := []int{-1, 0}
	for pos := 0; pos < len(b); {
		next := bytes.IndexByte(b[pos:], '\n')
		if next == -1 {
			break
		}
		pos += next + 1
		lines = append(lines, pos)
	}
	lines = append(lines, len(b))

	// Strip comments and switch small nodes to inline style
	cleanYaml(&node, lines, len(b))

	b, err = yaml.Marshal(&node)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
}

func cleanYaml(node *yaml.Node, lines []int, endPos int) {
	node.HeadComment = ""
	node.FootComment = ""
	node.LineComment = ""

	for i, n2 := range node.Content {
		end2 := endPos
		if i < len(node.Content)-1 {
			end2 = lines[node.Content[i+1].Line]
		}
		cleanYaml(n2, lines, end2)
	}

	// Use inline style?
	switch node.Kind {
	case yaml.MappingNode, yaml.SequenceNode:
		if endPos-lines[node.Line] < 40 {
			node.Style = yaml.FlowStyle
		}
	}
}

func allYamlNodes(n *yaml.Node) iter.Seq[*yaml.Node] {
	return func(yield func(*yaml.Node) bool) {
		if !yield(n) {
			return
		}
		for _, n2 := range n.Content {
			for n3 := range allYamlNodes(n2) {
				if !yield(n3) {
					return
				}
			}
		}
	}
}

func TestRoundTripString(t *testing.T) {
	// Check that we can round-trip a string with regexp meta-characters in it.
	const y = `!string test*`
	t.Logf("input:\n%s", y)

	v1 := oneValue(t, mustParse(y))
	var buf1 strings.Builder
	enc := yaml.NewEncoder(&buf1)
	if err := enc.Encode(v1); err != nil {
		log.Fatal(err)
	}
	enc.Close()
	t.Logf("after parse 1:\n%s", buf1.String())

	v2 := oneValue(t, mustParse(buf1.String()))
	var buf2 strings.Builder
	enc = yaml.NewEncoder(&buf2)
	if err := enc.Encode(v2); err != nil {
		log.Fatal(err)
	}
	enc.Close()
	t.Logf("after parse 2:\n%s", buf2.String())

	if buf1.String() != buf2.String() {
		t.Fatal("parse 1 and parse 2 differ")
	}
}

func TestEmptyString(t *testing.T) {
	// Regression test. Make sure an empty string is parsed as an exact string,
	// not a regexp.
	const y = `""`
	t.Logf("input:\n%s", y)

	v1 := oneValue(t, mustParse(y))
	if !v1.Exact() {
		t.Fatal("expected exact string")
	}
}
