package token

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

// Common errors.
var (
	ErrSeek   = errors.New("wrong file position after seek")
	ErrNoSeek = errors.New("io.Reader dos not support Seek")
	ErrLine   = errors.New("invalid line number")
)

// Position describes an arbitrary source position including the file, line, and column location.
//
type Position struct {
	Filename string
	Line     int // 1-based line number
	Column   int // 1-based column number (byte index)
}

func (p Position) String() string {
	return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
}

// A File represents an input file. It's a wrapper around an io.Reader that
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
// If pos represents a position before the position of the last known line,
// or if line is not equal to the last know line number plus one, AddLine will
// panic.
//
func (f *File) AddLine(pos Pos, line int) {
	l := len(f.lines)
	if (l > 0 && f.lines[l-1] >= pos) || l+1 != line {
		panic(ErrLine)
	}
	f.lines = append(f.lines, pos)
}

// Position returns the 1-based line and column for a given pos. The returned
// column is a byte offset, not a rune offset.
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
	return Position{f.name, i, int(pos - f.lines[i-1] + 1)}
}

// LinePos return the file offset of the given line.
//
func (f *File) LinePos(line int) Pos {
	if line < 1 || line > len(f.lines) {
		return -1
	}
	return f.lines[line-1]
}

// GetLineBytes returns a string containing the line for position pos.
//
func (f *File) GetLineBytes(pos Pos) (l []byte, err error) {
	lp := f.LinePos(f.Position(pos).Line)
	if !lp.IsValid() {
		return nil, ErrLine
	}
	rs, ok := f.Reader.(io.ReadSeeker)
	if !ok {
		return nil, ErrNoSeek
	}
	cur, err := rs.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	defer func() {
		p, err := rs.Seek(cur, io.SeekStart)
		if err != nil {
			// cannot resume normal operation, panic
			panic(err)
		}
		if p != cur {
			panic(ErrSeek)
		}
	}()
	fp, err := rs.Seek(int64(lp), io.SeekStart)
	if err != nil {
		return nil, err
	}
	if fp != int64(lp) {
		return nil, ErrSeek
	}

	// read the line
	r := bufio.NewReader(rs)
	for {
		buf, pref, err := r.ReadLine()
		if err != nil {
			return nil, err
		}
		l = append(l, buf...)
		if !pref {
			break
		}
	}

	return l, nil
}
