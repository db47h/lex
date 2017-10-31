package lexer

import (
	"errors"

	"github.com/db47h/asm/token"
)

const (
	bufSz = 256
	mask  = (bufSz - 1)
)

type reader struct {
	r *token.File
	c int
	t int

	buf [bufSz]struct {
		r  rune
		sz int
	}
}

func (r *reader) ReadRune() (ru rune, sz int, err error) {
	if r.c == r.t {
		ru, sz, err = r.r.ReadRune()
		i := &r.buf[r.c]
		i.r, i.sz = ru, sz
		r.c = (r.c + 1) & mask
		r.t = r.c
		return
	}
	i := &r.buf[r.c]
	r.c = (r.c + 1) & mask
	return i.r, i.sz, nil
}

func (r *reader) UnreadRune() error {
	c := (r.c - 1) & mask
	if c == r.t {
		return errors.New("ring buffer overflow")
	}
	r.c = c
	return nil
}
