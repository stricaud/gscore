/*
 * dtn.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package dorico

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// The tree a Dorico project keeps its score in.
//
// A .dorico file is an OASIS container — a zip whose META-INF/container.xml
// names the member holding the music — and that member, score.dtn, is this:
//
//	u32 4, u32 1
//	u32 nameCount,  that many NUL-terminated element names
//	u32 valueCount, that many NUL-terminated values, the first of them empty
//	u32 nodeCount,  that many records, the first being the root
//
// and each record is a tag byte, a name index and a value index, with a
// child count and that many child node numbers where the tag has its 2 bit
// set. Every number after the tag is a LEB128 varint.
//
// It is a flat array of nodes that refer to each other by number rather
// than a nested structure, and both the names and the values are pooled, so
// the thousand staves of an orchestral part that all read "kNormal" cost
// one copy of the word and a thousand indices.
//
// Nothing of this is documented by Steinberg. It was worked out from the
// files, and the decoder refuses anything that does not fit it exactly —
// which is the point of the node count being in the header: a correct read
// ends on the last byte of the file, and one that does not has
// misunderstood something and says so rather than handing back a score made
// of noise.

// tree is a decoded score.dtn.
type tree struct {
	names  []string
	values []string

	name  []int32
	value []int32
	kids  [][]int32
}

// nodes is how many nodes the tree holds.
func (t *tree) nodes() int { return len(t.name) }

// nameOf is the element name of a node: "flow", "pitch", "position".
func (t *tree) nameOf(n int32) string {
	if n < 0 || int(n) >= len(t.name) {
		return ""
	}
	return t.names[t.name[n]]
}

// text is a node's own value, which is empty for a node that only holds
// children. Everything in the file is text, numbers included: a position is
// "69/2" and a pitch is "67".
func (t *tree) text(n int32) string {
	if n < 0 || int(n) >= len(t.value) {
		return ""
	}
	return t.values[t.value[n]]
}

// children lists a node's children, in the order the file holds them, which
// is the order they were written in and the order they are meant in.
func (t *tree) children(n int32) []int32 {
	if n < 0 || int(n) >= len(t.kids) {
		return nil
	}
	return t.kids[n]
}

// child finds a node's first child of a given name, or -1.
//
// First, not only: a node may hold several children of one name — the
// events of a block, the flows of a score — and those are asked for with
// all. Where a name is a node's single property, this is the one to use.
func (t *tree) child(n int32, name string) int32 {
	for _, k := range t.children(n) {
		if t.nameOf(k) == name {
			return k
		}
	}
	return -1
}

// all lists a node's children of a given name.
func (t *tree) all(n int32, name string) []int32 {
	var out []int32
	for _, k := range t.children(n) {
		if t.nameOf(k) == name {
			out = append(out, k)
		}
	}
	return out
}

// str reads a named child's value: t.str(note, "pitch") is "67".
func (t *tree) str(n int32, name string) string {
	return t.text(t.child(n, name))
}

// path walks down a chain of single named children, which is how most of
// the file reads: a time signature is a barDivisionData, a timeSignature, a
// timeSignaturesAndDivisions, a timeSignatureAndDivision and a timeSignature
// again before there is a numerator to read.
//
// -1 as soon as any step is missing, so a caller writes one walk rather than
// five checks.
func (t *tree) path(n int32, names ...string) int32 {
	for _, name := range names {
		if n < 0 {
			return -1
		}
		n = t.child(n, name)
	}
	return n
}

// Dorico keeps its lists as an "array" node holding the items, so nearly
// every walk has one of these in the middle of it.
func (t *tree) list(n int32, name string) []int32 {
	a := t.path(n, name, "array")
	if a < 0 {
		// Some lists are written without the wrapper.
		if k := t.child(n, name); k >= 0 {
			return t.children(k)
		}
		return nil
	}
	return t.children(a)
}

// every lists every node of a given name, wherever it is in the tree.
//
// The tree is a flat array, so this is a walk of it rather than a search
// down from the root — which is what is wanted for the things the file
// keeps in one place and refers to from everywhere: the instrument
// definitions, say, which are in a library the music points into by name.
func (t *tree) every(name string) []int32 {
	var out []int32
	for i := range t.name {
		if t.names[t.name[i]] == name {
			out = append(out, int32(i))
		}
	}
	return out
}

// decodeDTN reads a score.dtn.
func decodeDTN(b []byte) (*tree, error) {
	if len(b) < 12 {
		return nil, errors.New("the score in this project is too short to " +
			"be one")
	}
	if binary.LittleEndian.Uint32(b) != 4 ||
		binary.LittleEndian.Uint32(b[4:]) != 1 {
		return nil, errors.New("this is not a Dorico score, or it is one " +
			"written in a way gscore has not seen")
	}

	at := 8
	names, at, err := pool(b, at, "element names")
	if err != nil {
		return nil, err
	}
	values, at, err := pool(b, at, "values")
	if err != nil {
		return nil, err
	}
	if at+4 > len(b) {
		return nil, errors.New("the score ends where its nodes should begin")
	}
	count := int(binary.LittleEndian.Uint32(b[at:]))
	at += 4
	if count < 1 || count > (len(b)-at)+1 {
		return nil, fmt.Errorf("the score says it holds %d nodes, which "+
			"cannot fit in the %d bytes it has left", count, len(b)-at)
	}

	t := &tree{
		names:  names,
		values: values,
		name:   make([]int32, count),
		value:  make([]int32, count),
		kids:   make([][]int32, count),
	}
	for i := 0; i < count; i++ {
		if at >= len(b) {
			return nil, fmt.Errorf("the score ends at node %d of %d",
				i, count)
		}
		tag := b[at]
		at++
		var name, value, n uint64
		if name, at, err = varint(b, at); err != nil {
			return nil, fmt.Errorf("node %d: %w", i, err)
		}
		if value, at, err = varint(b, at); err != nil {
			return nil, fmt.Errorf("node %d: %w", i, err)
		}
		if int(name) >= len(names) || int(value) >= len(values) {
			return nil, fmt.Errorf("node %d names entry %d of %d and value "+
				"%d of %d, and one of them is not there",
				i, name, len(names), value, len(values))
		}
		t.name[i], t.value[i] = int32(name), int32(value)

		// The 2 bit says the node has children. Everything else in the tag
		// is the file's own business and is not read.
		if tag%4 < 2 {
			continue
		}
		if n, at, err = varint(b, at); err != nil {
			return nil, fmt.Errorf("node %d: %w", i, err)
		}
		if n > uint64(count) {
			return nil, fmt.Errorf("node %d claims %d children, and the "+
				"score holds %d nodes altogether", i, n, count)
		}
		kids := make([]int32, 0, n)
		for j := uint64(0); j < n; j++ {
			var k uint64
			if k, at, err = varint(b, at); err != nil {
				return nil, fmt.Errorf("node %d, child %d: %w", i, j, err)
			}
			if k >= uint64(count) {
				return nil, fmt.Errorf("node %d names child %d, and the "+
					"score holds %d nodes", i, k, count)
			}
			kids = append(kids, int32(k))
		}
		t.kids[i] = kids
	}

	// The read has to end on the last byte. The node count is in the header,
	// so anything left over means the records were not the shape this
	// believes they are — and a tree misread that way is not a score with a
	// mistake in it, it is a different score.
	if at != len(b) {
		return nil, fmt.Errorf("the score was read to byte %d of %d: the "+
			"file is laid out in a way gscore does not know", at, len(b))
	}
	return t, nil
}

// pool reads a count and that many NUL-terminated strings.
func pool(b []byte, at int, what string) ([]string, int, error) {
	if at+4 > len(b) {
		return nil, at, fmt.Errorf("the score ends where its %s begin", what)
	}
	n := int(binary.LittleEndian.Uint32(b[at:]))
	at += 4
	if n < 0 || n > len(b)-at+1 {
		return nil, at, fmt.Errorf("the score says it holds %d %s, which "+
			"cannot fit in what is left of the file", n, what)
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		end := at
		for end < len(b) && b[end] != 0 {
			end++
		}
		if end >= len(b) {
			return nil, at, fmt.Errorf("the score ends inside its %s, at "+
				"%d of %d", what, i, n)
		}
		out = append(out, string(b[at:end]))
		at = end + 1
	}
	return out, at, nil
}

// varint reads one LEB128 number.
func varint(b []byte, at int) (uint64, int, error) {
	var v uint64
	var shift uint
	for {
		if at >= len(b) {
			return 0, at, errors.New("a number runs off the end of the file")
		}
		c := b[at]
		at++
		if shift >= 64 {
			return 0, at, errors.New("a number is too big to be one")
		}
		v |= uint64(c&0x7f) << shift
		if c&0x80 == 0 {
			return v, at, nil
		}
		shift += 7
	}
}
