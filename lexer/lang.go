package lexer

import "github.com/db47h/asm/token"

type nodeList map[rune]*node

// A node is a node in the token search tree of a language.
//
type node struct {
	c nodeList // child nodes
	s StateFn
	t token.Token
}

// match returns the first child node that matches the given rune.
//
func (n *node) match(r rune) *node {
	return n.c[r]
}

type filter struct {
	m func(r rune) bool
	s StateFn
	t token.Token
}

// A Lang represents the tokens (terminals) used in a language.
//
type Lang struct {
	last token.Token // last token id allocated in NewToken().
	e    *node       // exact matches
	b    []filter
}

func (l *Lang) init() {
	if l.e == nil {
		l.e = &node{c: make(nodeList)}
	}
}

// filter looks up the first registered MatchFn that returns true for the given rune.
//
func (l *Lang) filter(r rune) (StateFn, token.Token) {
	for i := range l.b {
		if l.b[i].m(r) {
			return l.b[i].s, l.b[i].t
		}
	}
	return nil, token.Invalid
}

// Match registers the state f for input starting with the string s.
// When in its initial state, if the input matches s, it switches its state to f.
//
func (l *Lang) Match(s string, f StateFn) token.Token {
	l.init()
	n := l.e
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
	l.last--
	n.s = f
	n.t = l.last
	return l.last
}

// MatchAny registers the state f for input starting with any of the runes in the
// string s.
//
func (l *Lang) MatchAny(s string, f StateFn) token.Token {
	l.init()
	c := l.e.c
	l.last--
	for _, r := range s {
		if n := c[r]; n != nil {
			if n.s != nil {
				panic("token registered twice")
			}
		} else {
			c[r] = &node{c: make(nodeList), s: f, t: l.last}
		}
	}
	return l.last
}

// MatchFn registers the state f for input where matchFn returns true for the first rune in a token.
//
func (l *Lang) MatchFn(matchFn func(r rune) bool, f StateFn) token.Token {
	l.init()
	l.last--
	l.b = append(l.b, filter{m: matchFn, s: f, t: l.last})
	return l.last
}
