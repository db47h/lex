package lexer

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/db47h/parsekit/token"
)

var buf = make([]byte, 50<<20) // 50MiB

func init() {
	rand.Seed(123456)
	for i := range buf {
		buf[i] = byte('a' + rand.Intn(26))
	}
}

func BenchmarkLexer(b *testing.B) {
	l := New(token.NewFile("", bytes.NewReader(buf)), nil)
	s := (*State)(l)
	rand.Seed(123456)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if rand.Intn(3) == 0 {
			s.Backup()
		} else {
			s.Next()
		}
	}
}
