package lang

import (
	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/token"
)

type nodeList map[rune]*node

// A node is a node in the token search tree of a language.
//
type node struct {
	c nodeList // child nodes
	s lexer.StateFn
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
	i lexer.StateFn
	e *node // exact matches
}

// New returns a new language with initFn as its initial state function.
//
func New(initFn lexer.StateFn) *Lang {
	if initFn == nil {
		panic("no initial state function provided")
	}
	return &Lang{i: initFn, e: &node{c: make(nodeList)}}
}

// Init returns the language's initial state function.
//
func (lang *Lang) Init() lexer.StateFn {
	return lang.doMatch
}

func (lang *Lang) doMatch(l *lexer.Lexer) lexer.StateFn {
	var match *node
	var i, mi int

	r := l.Next()

	for n := lang.e.match(r); n != nil; n = n.match(r) {
		if n.s != nil {
			mi = i
			match = n
		}
		if len(n.c) == 0 || r == lexer.EOF {
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
func (lang *Lang) MatchRunes(t token.Type, s []rune, f lexer.StateFn) {
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
func (lang *Lang) Match(t token.Type, s string, f lexer.StateFn) {
	lang.MatchRunes(t, []rune(s), f)
}

// MatchAny registers the state f for input starting with any of the runes in s.
//
func (lang *Lang) MatchAny(t token.Type, s []rune, f lexer.StateFn) {
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
