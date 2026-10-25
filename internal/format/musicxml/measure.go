/*
 * measure.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package musicxml

import "encoding/xml"

// A measure's children have to keep the order they were written in.
//
// MusicXML interleaves <direction> and <note>, and the order is the meaning: a
// dynamic applies to the notes that follow it, so three dynamics in a bar are
// three separate instructions at three separate points. Go's XML decoder
// gathers same-named elements into their own slices and loses that, which
// silently reduced a bar to one dynamic in each direction. Reading and writing
// the children as one ordered list is the fix.

// measureItem is one child of a measure. Exactly one field is set.
type measureItem struct {
	Attributes *attributes
	Direction  *direction
	Note       *xmlNote
	Barline    *barline
}

// xmlMeasure is a measure with its children in document order.
type xmlMeasure struct {
	Number  string
	Content []measureItem
}

// add appends a child.
func (m *xmlMeasure) add(it measureItem) { m.Content = append(m.Content, it) }

// notes returns the measure's notes, for callers that do not care about order.
func (m *xmlMeasure) notes() []xmlNote {
	var out []xmlNote
	for _, it := range m.Content {
		if it.Note != nil {
			out = append(out, *it.Note)
		}
	}
	return out
}

// MarshalXML writes the measure with its children in the order held.
func (m xmlMeasure) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "measure"}
	start.Attr = []xml.Attr{{Name: xml.Name{Local: "number"}, Value: m.Number}}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, it := range m.Content {
		var err error
		switch {
		case it.Attributes != nil:
			err = e.EncodeElement(it.Attributes, xml.StartElement{
				Name: xml.Name{Local: "attributes"}})
		case it.Direction != nil:
			err = e.EncodeElement(it.Direction, xml.StartElement{
				Name: xml.Name{Local: "direction"}})
		case it.Note != nil:
			err = e.EncodeElement(it.Note, xml.StartElement{
				Name: xml.Name{Local: "note"}})
		case it.Barline != nil:
			err = e.EncodeElement(it.Barline, xml.StartElement{
				Name: xml.Name{Local: "barline"}})
		}
		if err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

// UnmarshalXML reads the children in the order they appear, ignoring the ones
// gscore does not model rather than failing on them.
func (m *xmlMeasure) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		if a.Name.Local == "number" {
			m.Number = a.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "attributes":
				var v attributes
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				m.add(measureItem{Attributes: &v})
			case "direction":
				var v direction
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				m.add(measureItem{Direction: &v})
			case "note":
				var v xmlNote
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				m.add(measureItem{Note: &v})
			case "barline":
				var v barline
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				m.add(measureItem{Barline: &v})
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil
		}
	}
}
