package lexer

import (
	"math/rand"
	"testing"

	"github.com/db47h/parsekit/token"
)

type mockReader struct{}

func (mockReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	return len(p), nil
}

func BenchmarkLexer(b *testing.B) {
	l := New(token.NewFile("", mockReader{}), nil)
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
