/*
 * Copyright (C) 2021 Nuts community
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 *
 */

package crypto

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"

	"github.com/nuts-foundation/nuts-node/core"

	"github.com/sirupsen/logrus"
)

// NewTestCryptoInstance returns a new Crypto instance to be used for integration tests. Any data is stored in the
// specified test directory.
func NewTestCryptoInstance(testDirectory string) *Crypto {
	newInstance := NewCryptoInstance()
	if err := newInstance.Configure(core.ServerConfig{Datadir: testDirectory}); err != nil {
		logrus.Fatal(err)
	}
	return newInstance
}

// StringNamingFunc can be used to give a key a simple string name
func StringNamingFunc(name string) KIDNamingFunc {
	return func(key crypto.PublicKey) (string, error) {
		return name, nil
	}
}

func ErrorNamingFunc(err error) KIDNamingFunc {
	return func(key crypto.PublicKey) (string, error) {
		return "", err
	}
}

func NewTestKey(kid string) Key {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	return keySelector{
		privateKey: key,
		kid:        kid,
	}
}

// TestKey is a Key impl for testing purposes
type TestKey struct {
	PrivateKey crypto.Signer
	Kid        string
}

func (t TestKey) Signer() crypto.Signer {
	return t.PrivateKey
}

func (t TestKey) KID() string {
	return t.Kid
}

func (t TestKey) Public() crypto.PublicKey {
	return t.PrivateKey.Public()
}

func (t TestKey) Private() crypto.PrivateKey {
	return t.PrivateKey
}
