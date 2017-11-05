package token

import (
	"fmt"
	"io"
)

const (
	bufSz = 256
	mask  = (bufSz - 1)
)

// Position describes an arbitrary source position including the file, line, and column location.
//
type Position struct {
	Filename string
	Offset   int // rune index in the file
	Line     int // 1-based line number
	Column   int // 1-based column number (rune index)
}

func (p Position) String() string {
	return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
}

// A File represents an input file. It's a wrapper around an io.RuneReader that
// handles file offset to line/column conversion.
//
type File struct {
	name  string
	r     io.RuneReader
	lines []Pos // 0-based line/Pos information
}

// NewFile returns a new File.
//
func NewFile(name string, r io.RuneReader) *File {
	return &File{
		name:  name,
		r:     r,
		lines: []Pos{0}, // auto-add line 0 at pos 0
	}
}

// Name returns the file name.
//
func (f *File) Name() string {
	return f.name
}

// ReadRune implements io.RuneReader
//
func (f *File) ReadRune() (r rune, sz int, err error) {
	return f.r.ReadRune()
}

// AddLine adds the line offset for a new line. Note that the first line is
// automatically added at offset 0 by NewFile().
//
func (f *File) AddLine(pos Pos) {
	if l := len(f.lines); l == 0 || f.lines[l-1] < pos {
		f.lines = append(f.lines, pos)
	}
}

// Position returns the 1-based line and column for a given pos.
//
func (f *File) Position(pos Pos) Position {
	i, j := 0, len(f.lines)
	for i < j {
		h := int(uint(i+j) >> 1)
		if !(f.lines[h] > pos) {
			i = h + 1
		} else {
			j = h
		}
	}
	return Position{f.name, int(pos), i, int(pos - f.lines[i-1] + 1)}
}
