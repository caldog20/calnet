package types

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"io"

	"golang.org/x/crypto/curve25519"
)

type NoCompare [0]func()

type PublicKey struct {
	k [32]byte
}

type PrivateKey struct {
	_ NoCompare
	k [32]byte
}

func NewPrivateKey() PrivateKey {
	k := [32]byte{}
	if _, err := io.ReadFull(rand.Reader, k[:]); err != nil {
		panic("error generating random bytes for private key: " + err.Error())
	}

	// clamp
	k[0] &= 248
	k[31] = (k[31] & 127) | 64
	return PrivateKey{k: k}
}

func (k PrivateKey) Public() PublicKey {
	pub := PublicKey{}
	curve25519.ScalarBaseMult(&pub.k, &k.k)
	return pub
}

func (k PrivateKey) MarshalText() ([]byte, error) {
	b := make([]byte, base64.StdEncoding.EncodedLen(len(k.k)))
	base64.StdEncoding.Encode(b, k.k[:])
	return b, nil
}

func (k *PrivateKey) UnmarshalText(text []byte) error {
	_, err := base64.StdEncoding.Decode(k.k[:], text)
	if err != nil {
		return err
	}
	return nil
}

func (k PublicKey) MarshalText() ([]byte, error) {
	b := make([]byte, base64.StdEncoding.EncodedLen(len(k.k)))
	base64.StdEncoding.Encode(b, k.k[:])
	return b, nil
}

func (k *PublicKey) UnmarshalText(text []byte) error {
	_, err := base64.StdEncoding.Decode(k.k[:], text)
	if err != nil {
		return err
	}
	return nil
}

func (k PublicKey) String() string {
	return base64.StdEncoding.EncodeToString(k.k[:])
}

func (k PublicKey) Raw() []byte {
  return bytes.Clone(k.k[:])
}

func PublicKeyFromRawBytes(raw []byte) PublicKey {
	var key PublicKey
	copy(key.k[:], raw)
	return key
}
