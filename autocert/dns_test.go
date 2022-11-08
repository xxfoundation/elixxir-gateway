////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package autocert

import (
	"bytes"
	"crypto"
	"fmt"
	"math/rand"
	"testing"

	"golang.org/x/crypto/acme"
	"golang.org/x/net/context"
)

func TestMain(m *testing.M) {
	rand.Seed(42) // consistent answers, more or less
	dnsClientObj = func() acmeClient {
		return &dummyACMEClient{}
	}
}

func TestGenerateCertTestGenerate(t *testing.T) {
	// Regular Gen
	rng1 := &dummyRNG{}
	key, err := GenerateCertKey(rng1)
	if err != nil {
		t.Errorf("%+v", err)
	}

	if key.Size() != 4096/8 {
		t.Errorf("bad key size, expected 4096, got %d", key.Size())
	}

	// Short read error
	// NOTE: The RSA generate function will happily loop forever
	// if the RNG just does an implied short read, it has to return an
	// error!
	rng2 := &shortRNG{}
	key, err = GenerateCertKey(rng2)
	if key != nil || err == nil {
		t.Errorf("expected failure, got success")
	}
}

// Very basic smoke test to create an object and then
// "load" the same object, which is only checking to
// make sure there are no crashes when doing this and that
// keys get loaded properly via PEM.
func TestNewAndLoad(t *testing.T) {
	n := NewDNS()
	actualN := n.(*dnsClient)

	rng1 := &dummyRNG{}
	key, err := GenerateCertKey(rng1)
	if err != nil {
		t.Errorf("%+v", err)
	}
	actualN.SetKey(key.GetGoRSA())
	actualN.PrivateKey = key

	n2, err := LoadDNS(key.MarshalPem())
	if err != nil {
		t.Errorf("%+v", err)
	}
	actualN2 := n2.(*dnsClient)

	if bytes.Compare(actualN.PrivateKey.MarshalPem(),
		actualN2.PrivateKey.MarshalPem()) != 0 {
		t.Errorf("keys do not match: %v\n%v",
			actualN.PrivateKey.MarshalPem(),
			actualN2.PrivateKey.MarshalPem())
	}
	if actualN2.GetKey() == nil {
		t.Errorf("internal key not set")
	}
}

// Smoke Test for Registration
func TestRegister(t *testing.T) {
	n := NewDNS()
	rng1 := &dummyRNG{}
	key, err := GenerateCertKey(rng1)
	if err != nil {
		t.Errorf("%+v", err)
	}

	// dGVzdHN0cmluZwo= = "teststring"
	err = n.Register(key, "eabKeyID", "dGVzdHN0cmluZwo=",
		"email@example.com")
	if err != nil {
		t.Errorf("%+v", err)
	}
	actualN := n.(*dnsClient)
	if actualN.GetKey() == nil {
		t.Errorf("internal key not set")
	}
	if bytes.Compare(actualN.PrivateKey.MarshalPem(),
		key.MarshalPem()) != 0 {
		t.Errorf("key not set, byte mismatch:\n%v\n%v\n",
			key.MarshalPem(), actualN.PrivateKey.MarshalPem())
	}
}

type dummyACMEClient struct {
	Key crypto.Signer
	URL string
}

func (a *dummyACMEClient) GetDirectoryURL() string {
	return a.URL
}
func (a *dummyACMEClient) SetDirectoryURL(d string) {
	a.URL = d
}
func (a *dummyACMEClient) GetKey() crypto.Signer {
	return a.Key
}
func (a *dummyACMEClient) SetKey(k crypto.Signer) {
	a.Key = k
}
func (a *dummyACMEClient) GetReg(ctx context.Context,
	x string) (*acme.Account, error) {
	return &acme.Account{}, nil
}
func (a *dummyACMEClient) Register(ctx context.Context, acct *acme.Account,
	tosFn func(tosURL string) bool) (*acme.Account, error) {
	return a.GetReg(ctx, "")
}
func (a *dummyACMEClient) DNS01ChallengeRecord(token string) (string, error) {
	return "dnsChallengeString", nil
}
func (a *dummyACMEClient) AuthorizeOrder(ctx context.Context,
	authzIDs []acme.AuthzID) (*acme.Order, error) {
	return &acme.Order{}, nil
}
func (a *dummyACMEClient) CreateOrderCert(ctx context.Context,
	finalURL string, csr []byte, ty bool) ([][]byte, string, error) {
	return nil, "", nil
}
func (a *dummyACMEClient) GetAuthorization(ctx context.Context,
	authzURL string) (*acme.Authorization, error) {
	return &acme.Authorization{}, nil
}
func (a *dummyACMEClient) Accept(ctx context.Context,
	chal *acme.Challenge) (*acme.Challenge, error) {
	return chal, nil
}
func (a *dummyACMEClient) WaitAuthorization(ctx context.Context,
	authzURL string) (*acme.Authorization, error) {
	return &acme.Authorization{}, nil
}

type dummyRNG struct{}

func (z *dummyRNG) Read(b []byte) (int, error) {
	return rand.Read(b)
}

type shortRNG struct{}

func (z *shortRNG) Read(b []byte) (int, error) {
	k, _ := rand.Read(b[0 : len(b)/2])
	return k, fmt.Errorf("short read")
}
