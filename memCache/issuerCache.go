package memCache

import (
	"crypto/sha256"
	"crypto/x509"
	"sync"
)

type issuerCache struct {
	hashed map[[32]byte]*x509.Certificate
	mu     sync.RWMutex
}

func newIssuerCache() *issuerCache {
	return &issuerCache{hashed: make(map[[32]byte]*x509.Certificate)}
}

func (ic *issuerCache) get(issuerSubject, akid []byte) *x509.Certificate {
	hashed := sha256.Sum256(append(issuerSubject, akid...))
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	return ic.hashed[hashed]
}

func (ic *issuerCache) add(issuer *x509.Certificate) error {
	hashed := sha256.Sum256(append(issuer.RawSubject, issuer.SubjectKeyId...))
	ic.mu.Lock()
	defer ic.mu.Unlock()
	ic.hashed[hashed] = issuer
	return nil
}