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
	LeftParen                 // (
	RightParen                // )
	LeftBracket               // [
	RightBracket              // ]
	Colon                     // :
	Comma                     // ,
	Space                     //
	Comment                   // ; -- and skip to EOL
	Dot                       // '.'
	Identifier                // any valid identifier
	Immediate                 // immediate
	LocalLabel                // local label (1f, 0b, etc.)
	Backslash                 // \
	OpPlus                    // +
	OpMinus                   // -
	OpFactor                  // *
	OpDiv                     // /
	OpMod                     // %
	OpXor                     // ^, also used as unary 'not'
	OpAnd                     // &
	OpOr                      // |
	BuiltIn                   // %identifier not identified by the lexer but by the parser
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
	f.lines = append(f.lines, pos)
	f.m.Unlock()
}

func (f *File) findPos(p Pos, s, e int) int {
	if e-s <= 1 {
		return s
	}
	m := s + (e-s)/2
	if f.lines[m] > p {
		return f.findPos(p, s, m)
	}
	return f.findPos(p, m, e)
}

// Position returns the 1-based line and column for a given pos.
//
func (f *File) Position(pos Pos) (line int, col int) {
	f.m.RLock()
	line = f.findPos(pos, 0, len(f.lines))
	f.m.RUnlock()
	return line + 1, int(pos - f.lines[line] + 1)
}
