package lexer

import (
	"github.com/db47h/asm/token"
)

type nodeList map[rune]*node

// A node is a node in the token search tree of a language.
//
type node struct {
	c nodeList // child nodes
	s StateFn
	t token.Type
}

// match returns the first child node that matches the given rune.
//
func (n *node) match(r rune) *node {
	return n.c[r]
}

// A Lang represents the tokens (terminals) used in a language.
//
type Lang struct {
	i StateFn
	e *node // exact matches
}

// NewLang returns a new language with initFn as its initial state function.
//
func NewLang(initFn StateFn) *Lang {
	if initFn == nil {
		initFn = func(l *Lexer) StateFn {
			r := l.Next()
			if r == EOF {
				return stateEOF
			}
			l.Emit(token.RawChar, r)
			return nil
		}
	}
	return &Lang{i: initFn, e: &node{c: make(nodeList)}}
}

func stateEOF(l *Lexer) StateFn {
	l.Emit(token.EOF, nil)
	return stateEOF
}

func (lang *Lang) doMatch(l *Lexer) StateFn {
	var match *node
	var i, mi int

	r := l.Next()

	for n := lang.e.match(r); n != nil; n = n.match(r) {
		if n.s != nil {
			mi = i
			match = n
		}
		if len(n.c) == 0 || r == EOF {
			// avoid unnecessary Next() / Backup() steps
			break
		}
		i++
		r = l.Next()
	}

	if match != nil {
		l.BackupN(i - mi)
		l.T = match.t
		return match.s(l)
	}

	l.BackupN(i + 1)
	return lang.i(l)
}

// MatchRunes registers the state f for input starting with the runes in s.
// When in its initial state, if the input matches s, it sets l.T = t and switches its state to f.
//
func (lang *Lang) MatchRunes(t token.Type, s []rune, f StateFn) {
	n := lang.e
	for _, r := range s {
		i, ok := n.c[r]
		if !ok {
			i = &node{c: make(nodeList)}
			n.c[r] = i
		}
		n = i
	}
	if n.s != nil {
		panic("token registered twice")
	}
	n.s = f
	n.t = t
}

// Match registers the state f for input starting with the string s.
// When in its initial state, if the input matches s, it sets l.T = t and switches its state to f.
//
func (lang *Lang) Match(t token.Type, s string, f StateFn) {
	lang.MatchRunes(t, []rune(s), f)
}

// MatchAny registers the state f for input starting with any of the runes in s.
//
func (lang *Lang) MatchAny(t token.Type, s []rune, f StateFn) {
	c := lang.e.c
	for _, r := range s {
		if n := c[r]; n != nil {
			if n.s != nil {
				panic("token registered twice")
			}
		} else {
			c[r] = &node{c: make(nodeList), s: f, t: t}
		}
	}
}