package pt

import (
	"sync/atomic"
	"time"
)

const (
	uint56mask uint64 = 0x00FFFFFFFFFFFFFF
)

var (
	DefaultTokenGenerator = TokenGenerator{
		lastToken: uint64(time.Now().UnixMicro()),
	}
)

type TokenGenerator struct {
	lastToken uint64
}

func (g *TokenGenerator) Last() uint64 {
	return atomic.LoadUint64(&g.lastToken) & uint56mask
}

func (g *TokenGenerator) Next() uint64 {
	token := uint64(time.Now().UnixMicro())

	for {
		last := atomic.LoadUint64(&g.lastToken)
		if token <= last {
			token = last + 1
		}

		swapped := atomic.CompareAndSwapUint64(&g.lastToken, last, token)

		if swapped {
			return (token & uint56mask)
		}
	}
}
