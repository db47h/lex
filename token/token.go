// Package token defines constants and types representing lexical tokens
// in assembler source text.
//
package token

import (
	"io"
	"sync"
)

//go:generate stringer -type Token

// Token represents a token's numeric ID
//
type Token uint

// Token IDs
//
const (
	EOF          Token = iota // end of file
	EOL                       // end of line
	Error                     // error -- the associated value is a string
	Identifier                // any valid identifier
	Immediate                 // immediate
	Space                     // unicode.IsSpace() == true
	Comment                   // ; -- and skip to EOL
	LeftParen                 // (
	RightParen                // )
	LeftBracket               // [
	RightBracket              // ]
	Colon                     // :
	Comma                     // ,
	Dot                       // .
	Backslash                 // \
	OpPlus                    // +
	OpMinus                   // -
	OpFactor                  // *
	OpDiv                     // /
	OpMod                     // %
	OpXor                     // ^
	OpAnd                     // &
	OpOr                      // |
)

// Pos represents a token's position within a File
//
type Pos int

// IsValid returns true if p is a valid position.
//
func (p Pos) IsValid() bool {
	return p >= 0
}

// A File represents an input file.
//
type File struct {
	name string
	r    io.RuneReader

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
	return f.r.ReadRune()
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
func (f *File) Position(pos Pos) (line int, col int) {
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
	col = int(pos - f.lines[i-1] + 1)
	f.m.RUnlock()
	return i, col
}
