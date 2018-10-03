package token

import (
	"fmt"
	"io"
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
	name string
	io.Reader
	lines []Pos // 0-based line/Pos information
}

// NewFile returns a new File.
//
func NewFile(name string, r io.Reader) *File {
	return &File{
		name:   name,
		Reader: r,
	}
}

// Name returns the file name.
//
func (f *File) Name() string {
	return f.name
}

// AddLine adds a new line at the given offset.
//
// line is the 1-based line index.
//
// The current implementation will only accept a new line if line == last line + 1
// and pos greater than the position of the previous line.
//
func (f *File) AddLine(pos Pos, line int) {
	l := len(f.lines)
	if l > 0 && f.lines[l-1] >= pos {
		// line already known
		return
	}
	if l+1 != line {
		panic("invalid line number")
	}
	f.lines = append(f.lines, pos)
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
