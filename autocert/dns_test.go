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
	"encoding/pem"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/net/context"
)

func TestMain(m *testing.M) {
	rand.Seed(42) // consistent answers, more or less
	dnsClientObj = func() acmeClient {
		return &dummyACMEClient{}
	}
	os.Exit(m.Run())
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
	err = n.Register(key, "eabKeyID", "dGVzdHN0cmluZwo",
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

func TestRequest(t *testing.T) {
	n := NewDNS()
	rng1 := &dummyRNG{}
	key, err := GenerateCertKey(rng1)
	if err != nil {
		t.Errorf("%+v", err)
	}

	// dGVzdHN0cmluZwo= = "teststring"
	err = n.Register(key, "eabKeyID", "dGVzdHN0cmluZwo",
		"email@example.com")
	if err != nil {
		t.Errorf("%+v", err)
	}
	k, v, err := n.Request("example.com")
	if err != nil {
		t.Errorf("%+v", err)
	}
	expK := "_acme-challenge.example.com"
	if k != expK {
		t.Errorf("unexpected key: expect %s, got %s", expK, k)
	}
	if v != "dnsChallengeString" {
		t.Errorf("unexpect chal: expected dnsChallengeString got %s",
			v)
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
	if actualN.AuthzURL != "url1" {
		t.Errorf("bad AuthzURL: %s", actualN.AuthzURL)
	}
	if actualN.Domain != "example.com" {
		t.Errorf("bad domain: %s", actualN.Domain)
	}
	if actualN.AuthzFinalizeURL != "finalme" {
		t.Errorf("bad final URL: %s", actualN.AuthzFinalizeURL)
	}
}

func TestCSR(t *testing.T) {
	n := NewDNS()
	rng1 := &dummyRNG{}
	key, err := GenerateCertKey(rng1)
	if err != nil {
		t.Errorf("%+v", err)
	}

	// dGVzdHN0cmluZwo= = "teststring"
	err = n.Register(key, "eabKeyID", "dGVzdHN0cmluZwo",
		"email@example.com")
	if err != nil {
		t.Errorf("%+v", err)
	}

	csrPEM, csrDER, err := n.CreateCSR("example.com", "test@example.com",
		"USA", "nodeID", rng1)
	if err != nil {
		t.Errorf("%+v", err)
	}

	expPEM := []byte{45, 45, 45, 45, 45, 66, 69, 71, 73, 78, 32, 67, 69,
		82, 84, 73, 70, 73, 67, 65}
	if bytes.Compare(expPEM, csrPEM[:len(expPEM)]) != 0 {
		t.Errorf("bad pem:\n\t%v\n\t%v", expPEM, csrPEM[:len(expPEM)])
	}
	expDER := []byte{48, 130, 4, 143, 48, 130, 2, 119, 2, 1, 0, 48, 74,
		49, 12, 48, 10, 6}
	if bytes.Compare(expDER, csrDER[:len(expDER)]) != 0 {
		t.Errorf("bad der:\n\t%v\n\t%v", expDER, csrDER[:len(expDER)])
	}
}

func TestIssue(t *testing.T) {
	n := NewDNS()
	rng1 := &dummyRNG{}
	key, err := GenerateCertKey(rng1)
	if err != nil {
		t.Errorf("%+v", err)
	}

	// dGVzdHN0cmluZwo= = "teststring"
	err = n.Register(key, "eabKeyID", "dGVzdHN0cmluZwo",
		"admins@elixxir.io")
	if err != nil {
		t.Errorf("%+v", err)
	}
	_, _, err = n.Request("rick1.xxn2.work")
	if err != nil {
		t.Errorf("%+v", err)
	}

	_, csrDER, err := n.CreateCSR("rick1.xxn2.work", "admins@elixxir.io",
		"USA", "nodeID", rng1)
	if err != nil {
		t.Errorf("%+v", err)
	}

	_, _, err = n.Issue(csrDER, time.Minute)
	if err != nil {
		t.Errorf("%+v", err)
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
	return &acme.Order{
		AuthzURLs: []string{
			"url1",
		},
		FinalizeURL: "finalme",
	}, nil
}
func (a *dummyACMEClient) CreateOrderCert(ctx context.Context,
	finalURL string, csr []byte, ty bool) ([][]byte, string, error) {
	validCert := `-----BEGIN CERTIFICATE-----
MIIHbjCCBVagAwIBAgIQSM+Uht+j3A/bOj7JPRBDrzANBgkqhkiG9w0BAQwFADBL
MQswCQYDVQQGEwJBVDEQMA4GA1UEChMHWmVyb1NTTDEqMCgGA1UEAxMhWmVyb1NT
TCBSU0EgRG9tYWluIFNlY3VyZSBTaXRlIENBMB4XDTIyMTEwNTAwMDAwMFoXDTIz
MDIwMzIzNTk1OVowGjEYMBYGA1UEAxMPcmljazEueHhuMi53b3JrMIICIjANBgkq
hkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAxn0SZaCiduE3RINmtOI2GsmO4jGputtF
RhV9c8SwbnHZvVsZGJpcF0Zp/ONhSbIiPJEzVoNUylwTNp4JFr1ePkigLOrR4akJ
ovmAE1GzhNuBvgyxjS05e/hgGzDno186r9WaxcDnuMqaXF3syGEGoyDkB4hSJmY3
YOya8KrxZNHw7IDGOIjoyWCz8XzBvXR5KGztSgixG28IYkB+wAyfHj9IrRf0/T3Y
u6CYbBgwD5JLEG2Mn5WLbDfcIDubNxB4ZrxoFKUfZGfoankrO3C7cPjn+JgC8ZZz
kNN1KlD1ZqkNDPI077UvVhhBhbPQehYcEbCwLBSoXQRvVn7Ij4OROIqgTJI5wBIo
FrmYfgbI4xjEgYBfLS3u+FUvM7KrCh9IaFC5gtJSxsqGHkxsAYYEyXNqys/yvCEb
fCBMH08zmoHhqLbvQNIrCY+L31C4jcvS/FaqGiWjusljUW5/1FKi8SnHCWrneAxN
oKlsWHTYbgppf9N42Yy25w/Yp4n7cpUoFmCio6L9KLsZsRcWGuX9kzdcklqVOsuF
Ej+XnOdAVujXbgAkvtlyxlcQky1n9PZ2jxOnyQQ22+9PuGVz9DMI6MIrkScSfWYO
BDN6Lb6O3CZ0DRl3h1c7KUlCssoNcqi67G7vpSuRYMQ7TvSaxZIpf4pIepufrUiV
FXJSdrupTwMCAwEAAaOCAn0wggJ5MB8GA1UdIwQYMBaAFMjZeGii2Rlo1T1y3l8K
Pty1hoamMB0GA1UdDgQWBBTvyeRInS6KVVyO9eK82Fy8Jl2o7TAOBgNVHQ8BAf8E
BAMCBaAwDAYDVR0TAQH/BAIwADAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUH
AwIwSQYDVR0gBEIwQDA0BgsrBgEEAbIxAQICTjAlMCMGCCsGAQUFBwIBFhdodHRw
czovL3NlY3RpZ28uY29tL0NQUzAIBgZngQwBAgEwgYgGCCsGAQUFBwEBBHwwejBL
BggrBgEFBQcwAoY/aHR0cDovL3plcm9zc2wuY3J0LnNlY3RpZ28uY29tL1plcm9T
U0xSU0FEb21haW5TZWN1cmVTaXRlQ0EuY3J0MCsGCCsGAQUFBzABhh9odHRwOi8v
emVyb3NzbC5vY3NwLnNlY3RpZ28uY29tMIIBBgYKKwYBBAHWeQIEAgSB9wSB9ADy
AHcArfe++nz/EMiLnT2cHj4YarRnKV3PsQwkyoWGNOvcgooAAAGERYbgFQAABAMA
SDBGAiEA/d+USH39vf8RvckdgiB+a+NyDEb/7xG4VIBZPYIXLQACIQD4XySXos55
o0wyE3XTzylkqoyupOaLtGpSkbDZlIKp2QB3AHoyjFTYty22IOo44FIe6YQWcDIT
hU070ivBOlejUutSAAABhEWG3+MAAAQDAEgwRgIhAOBKcYLCr3EhrTqyywMvCA3Z
OeYesLfOdpJJJgkGB9aVAiEA36DwB++0dEzJf6JiXsXpuUfjZ4KgtDWufl+FYNd3
LlgwGgYDVR0RBBMwEYIPcmljazEueHhuMi53b3JrMA0GCSqGSIb3DQEBDAUAA4IC
AQAon7p8PMR/dbGBzqG0zoNotB+BxqdJV1VtJk0XYKQl/i03fBr4fzvhnBEFvKe4
qjk2YUqB52muLl64n0o0yI96p74F5j+X1xbuX1JwT1hA3udGRNrS4vkFTUV/ymhe
CPWnZaSpIudtb5uO/nNCZ/+794NHzmPHbC4oTo83wRFoxZst50jvC2a5E9ewY41h
uZ0la/n6Q+2/F9CBYYPvLeLqPmRco9QhXl6CDndvetwNkOXh75Kt517Xu97TcdK2
S+UdpRcZWonxosNxLEFtu3otRmxzT/3PhMit3GqQxw3gFzqjNIgwmumF2buSGZeH
SVBh+HkqxyE286UtHanCPrYv4ev4WjV05PvRbwSZyx/d//Lvmo9lheXzKN4HVVbh
wZX1HDu+mkmeHquDAQYT3Wbn0f5rmXaCbCBtAOLDpJs8jEu/9Xsb8R9IgsBdUkeL
tPt8haNAF82yYKlcBFJhPsQZSSabMy1Ew6gXqDCLC+YyxY5ASuLk4fOBMqfGuVDS
9Po+PFUvdas/yypuMauZWazj2kXki06xGylTb5pDgResTuGMQ4X7+9COzoGLNedt
iz2njhHe7DWAQiOFOllO1mCUvzJ0oyYF4C9YHIT4j+Mzq31mmpLAcv/oI10Yaco9
ha+P46sxUx4cDvtzq29TynRlbNAXj27yripo/2Azn6IG/Q==
-----END CERTIFICATE-----`
	pemBlk, _ := pem.Decode([]byte(validCert))
	der := pemBlk.Bytes
	return [][]byte{der}, "certURL", nil
}
func (a *dummyACMEClient) GetAuthorization(ctx context.Context,
	authzURL string) (*acme.Authorization, error) {
	return &acme.Authorization{
		Challenges: []*acme.Challenge{
			{
				Type: "dns-01",
			},
		},
	}, nil
}
func (a *dummyACMEClient) Accept(ctx context.Context,
	chal *acme.Challenge) (*acme.Challenge, error) {
	return chal, nil
}
func (a *dummyACMEClient) WaitAuthorization(ctx context.Context,
	authzURL string) (*acme.Authorization, error) {
	return &acme.Authorization{
		Status: acme.StatusValid,
	}, nil
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
