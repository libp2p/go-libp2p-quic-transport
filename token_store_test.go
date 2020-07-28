package libp2pquic

import (
	"unsafe"

	quic "github.com/lucas-clemente/quic-go"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type quicClientToken struct {
	data []byte
}

func toQuicToken(data string) *quic.ClientToken {
	return (*quic.ClientToken)(unsafe.Pointer(&quicClientToken{data: []byte(data)}))
}

func fromQuicToken(token *quic.ClientToken) string {
	t := (*quicClientToken)(unsafe.Pointer(token))
	return string(t.data)
}

var _ = Describe("Token Store", func() {
	var store *tokenStore
	const key = "key"

	BeforeEach(func() {
		store = newTokenStore(3)
	})

	It("converts to quic.Token and back", func() {
		Expect(fromQuicToken(toQuicToken("foobar"))).To(Equal("foobar"))
	})

	It("pops nil if there are no tokens stored", func() {
		Expect(store.Pop(key)).To(BeNil())
	})

	It("stores and retrieves a token", func() {
		store.Put(key, toQuicToken("foo"))
		token := store.Pop(key)
		Expect(token).ToNot(BeNil())
		Expect(fromQuicToken(token)).To(Equal("foo"))
	})

	It("uses the most recent token first", func() {
		store.Put(key, toQuicToken("token1"))
		store.Put(key, toQuicToken("token2"))
		token := store.Pop(key)
		Expect(token).ToNot(BeNil())
		Expect(fromQuicToken(token)).To(Equal("token2"))
		token = store.Pop(key)
		Expect(token).ToNot(BeNil())
		Expect(fromQuicToken(token)).To(Equal("token1"))
	})

	It("limits the number of stored tokens", func() {
		store.Put(key, toQuicToken("token1"))
		store.Put(key, toQuicToken("token2"))
		store.Put(key, toQuicToken("token3"))
		store.Put(key, toQuicToken("token4"))
		token := store.Pop(key)
		Expect(token).ToNot(BeNil())
		Expect(fromQuicToken(token)).To(Equal("token4"))
		token = store.Pop(key)
		Expect(token).ToNot(BeNil())
		Expect(fromQuicToken(token)).To(Equal("token3"))
		token = store.Pop(key)
		Expect(token).ToNot(BeNil())
		Expect(fromQuicToken(token)).To(Equal("token2"))
		Expect(store.Pop(key)).To(BeNil())
	})
})
