package libp2pquic

import (
	"sync"

	quic "github.com/lucas-clemente/quic-go"
)

type tokenStore struct {
	capacity int

	mutex  sync.Mutex
	tokens []*quic.ClientToken
}

var _ quic.TokenStore = &tokenStore{}

func newTokenStore(cap int) *tokenStore {
	return &tokenStore{capacity: cap}
}

func (s *tokenStore) Put(_ string, token *quic.ClientToken) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.tokens) >= s.capacity {
		s.tokens = s.tokens[1:]
	}
	s.tokens = append(s.tokens, token)
}

func (s *tokenStore) Pop(_ string) *quic.ClientToken {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.tokens) == 0 {
		return nil
	}
	token := s.tokens[len(s.tokens)-1]
	s.tokens = s.tokens[:len(s.tokens)-1]
	return token
}
