// Copyright 2017-2020 Denis Bernard <db047h@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package lex

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

// IsValidOffset returns true if offset is a valid file offset (i.e. p >= 0).
//
func IsValidOffset(offset int) bool {
	return offset >= 0
}

// Common errors.
var (
	ErrSeek   = errors.New("wrong file offset after seek")
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
	lines []int // 0-based line/offset information
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

// AddLine adds a new line at the given file offset.
//
// line is the 1-based line index.
//
// If offset is before the offset of the last known line,
// or if line is not equal to the last know line number plus one, AddLine will
// panic.
//
func (f *File) AddLine(offset int, line int) {
	l := len(f.lines)
	if (l > 0 && f.lines[l-1] >= offset) || l+1 != line {
		panic(ErrLine)
	}
	f.lines = append(f.lines, offset)
}

// Position returns the 1-based line and column for a given file offset.
// The returned column is a byte offset, not a rune offset.
//
func (f *File) Position(offset int) Position {
	i, j := 0, len(f.lines)
	for i < j {
		h := int(uint(i+j) >> 1)
		if !(f.lines[h] > offset) {
			i = h + 1
		} else {
			j = h
		}
	}
	return Position{f.name, i, int(offset - f.lines[i-1] + 1)}
}

// LineOffset returns the file offset of the given line.
//
func (f *File) LineOffset(line int) int {
	if line < 1 || line > len(f.lines) {
		return -1
	}
	return f.lines[line-1]
}

// GetLineBytes returns a string containing the line for the given file offset.
//
func (f *File) GetLineBytes(offset int) (l []byte, err error) {
	lp := f.LineOffset(f.Position(offset).Line)
	if !IsValidOffset(lp) {
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
