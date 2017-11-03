package token

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

const (
	bufSz = 256
	mask  = (bufSz - 1)
)

var errBufferOverflow = errors.New("ring buffer overflow")

// Position describes an arbitrary source position including the file, line, and column location.
//
type Position struct {
	Filename string
	Offset   int
	Line     int
	Column   int
}

func (p Position) String() string {
	return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
}

// A File represents an input file.
//
type File struct {
	name string
	r    io.RuneReader
	buf  [bufSz]struct {
		r  rune
		sz int
	}
	o int // output index
	i int // input index

	m     sync.RWMutex
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
	if f.o == f.i {
		r, sz, err = f.r.ReadRune()
		if err != nil && err != io.EOF {
			return 0, 0, err
		}
		f.buf[f.i].r, f.buf[f.i].sz = r, sz
		f.i = (f.i + 1) & mask
		f.o = f.i
		return
	}
	r, sz = f.buf[f.o].r, f.buf[f.o].sz
	f.o = (f.o + 1) & mask
	return
}

// UnreadRune implements io.RuneScanner.
//
// File uses a 256 runes ring buffer, so calling UnreadRune over 255 times in a
// row without interleaved calls to ReadRune will return an error.
//
func (f *File) UnreadRune() error {
	o := (f.o - 1) & mask
	if o == f.i {
		return errBufferOverflow
	}
	f.o = o
	return nil
}

// AddLine adds the line offset for a new line. Note that the first line is
// automatically added at offset 0 by NewFile().
//
func (f *File) AddLine(pos Pos) {
	f.m.Lock()
	if l := len(f.lines); l == 0 || f.lines[l-1] < pos {
		f.lines = append(f.lines, pos)
	}
	f.m.Unlock()
}

// Position returns the 1-based line and column for a given pos.
//
func (f *File) Position(pos Pos) Position {
	f.m.RLock()
	i, j := 0, len(f.lines)
	for i < j {
		h := int(uint(i+j) >> 1)
		if !(f.lines[h] > pos) {
			i = h + 1
		} else {
			j = h
		}
	}
	p := Position{f.name, int(pos), i, int(pos - f.lines[i-1] + 1)}
	f.m.RUnlock()
	return p
}
