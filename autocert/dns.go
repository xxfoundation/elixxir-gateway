////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Package autocert requests an ACME certificate using EAB credentials
// and provides helper functions to wait until the certificate is issued.
package autocert

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"

	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/crypto/rsa"
	"golang.org/x/crypto/acme"
	"golang.org/x/net/context"
)

const ZeroSSLACMEURL = "https://acme.zerossl.com/v2/DV90"

type dnsClient struct {
	*acme.Client
	AuthzURL         string
	AuthzFinalizeURL string
	Domain           string
	PrivateKey       rsa.PrivateKey
}

// GenerateCertKey generates a 4096 bit RSA Private Key that can be used in
// the certificate request.
func GenerateCertKey(csprng io.Reader) (rsa.PrivateKey, error) {
	pKey, err := rsa.GetScheme().Generate(csprng, 4096)
	if err != nil {
		return nil, err
	}
	return pKey, nil
}

func (d *dnsClient) CreateCSR(domain, email, country, nodeID string,
	rng io.Reader) (csrPEM, csrDER []byte, err error) {

	subject := pkix.Name{
		Country:            []string{country},
		Organization:       []string{"xx network"},
		OrganizationalUnit: []string{nodeID},
		CommonName:         domain,
	}

	csrTemplate := &x509.CertificateRequest{
		SignatureAlgorithm: x509.SHA512WithRSA,
		PublicKeyAlgorithm: x509.RSA,
		PublicKey:          d.PrivateKey.GetGoRSA().PublicKey,
		Subject:            subject,
	}
	csrDER, err = x509.CreateCertificateRequest(
		rng,
		csrTemplate,
		d.PrivateKey.GetGoRSA(),
	)

	csrPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST",
		Bytes: csrDER})

	// return csrPEM, err
	return csrPEM, csrDER, err
}

func getDefaultContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 60*time.Second)

}

// NewDNS creates a new empty DNS Client object
func NewDNS() Client {
	return &dnsClient{
		Client: &acme.Client{
			DirectoryURL: ZeroSSLACMEURL,
		},
		AuthzURL:         "",
		AuthzFinalizeURL: "",
		Domain:           "",
	}
}

// LoadDNS recreates a DNS client object based on the private key PEM file
func LoadDNS(privateKeyPEM []byte) (Client, error) {
	d := &dnsClient{
		Client: &acme.Client{
			DirectoryURL: ZeroSSLACMEURL,
		},
		AuthzURL:         "",
		AuthzFinalizeURL: "",
		Domain:           "",
	}
	privateKey, err := rsa.GetScheme().UnmarshalPrivateKeyPEM(privateKeyPEM)
	if err != nil {
		return nil, err
	}

	d.Key = privateKey.GetGoRSA()
	d.PrivateKey = privateKey

	ctx, cancelFn := getDefaultContext()
	defer cancelFn()
	acct, err := d.GetReg(ctx, "")
	if err == nil {
		jww.DEBUG.Printf("looked up acct: %v", acct)
	}
	return d, err
}

func (d *dnsClient) Register(privateKey rsa.PrivateKey,
	eabKeyID, eabKey, email string) error {
	// Let's rule out dumb mistakes and decode/create the external account
	// binding first.
	eabHMAC, err := base64.RawURLEncoding.DecodeString(eabKey)
	if err != nil {
		return err
	}

	d.Key = privateKey.GetGoRSA()

	acctReq := &acme.Account{
		ExternalAccountBinding: &acme.ExternalAccountBinding{
			KID: eabKeyID,
			Key: eabHMAC,
		},
		Contact: []string{fmt.Sprintf("mailto:%s", email)},
	}

	// Note: this is wonky, because the account object sent is not modified
	// and a new one gets returned. A review of the internals shows that
	// only the ExternalAccountBinding and Contact objects are used
	ctx, cancelFn := getDefaultContext()
	defer cancelFn()
	acct, err := d.Client.Register(ctx, acctReq, acme.AcceptTOS)
	if err != nil {
		return err
	}

	d.PrivateKey = privateKey

	jww.DEBUG.Printf("Account Registered: %v", acct)
	return nil
}

func (d *dnsClient) Request(domain string) (key, value string, err error) {
	authzIDs := []acme.AuthzID{
		{
			Type:  "dns",
			Value: domain,
		},
	}

	order, err := getAuthOrder(d.Client, authzIDs)
	if err != nil {
		jww.ERROR.Printf("Authorize failed: %+v", err)
		return "", "", err
	}
	jww.DEBUG.Printf("Order Returned: %v", order)

	dns01, authzURL, err := getDNSChallenge(d.Client, order)
	if err != nil {
		jww.ERROR.Printf("DNS challenge failed: %+v", err)
		return "", "", err
	}

	d.AuthzURL = authzURL
	d.Domain = domain
	d.AuthzFinalizeURL = order.FinalizeURL

	if dns01 == nil {
		return "already validated", "none", nil
	}

	jww.DEBUG.Printf("DNS Challenge: %v", dns01)

	dns01, err = acceptDNSChallenge(d.Client, dns01)
	if err != nil {
		jww.ERROR.Printf("accept DNS failed: %+v", err)
		return "", "", err
	}

	dnsChallenge, err := d.DNS01ChallengeRecord(dns01.Token)
	if err != nil {
		jww.ERROR.Printf("DNS token challenge failed: %+v", err)
		return "", "", err
	}

	key = fmt.Sprintf("_acme-challenge.%s", domain)
	value = dnsChallenge

	jww.DEBUG.Printf("TXT Record:\n%s\t%s", key, value)

	return key, value, nil
}

func (d *dnsClient) Issue(csr []byte) (cert, key []byte, err error) {
	if d.AuthzURL == "" {
		return nil, nil, errors.Errorf("missing auth, call Request")
	}
	authz := waitForAuthorization(d.Client, d.AuthzURL)
	if authz.Status != acme.StatusValid {
		return nil, nil, errors.Errorf("invalid status object: %v",
			authz)
	}
	jww.DEBUG.Printf("Final Auth: %v", authz)

	ctx, cancelFn := getDefaultContext()
	defer cancelFn()
	der, certURL, err := d.CreateOrderCert(ctx, d.AuthzFinalizeURL, csr,
		false)
	if err != nil {
		jww.ERROR.Printf("cannot create cert: %+v", err)
		return nil, nil, err
	}

	jww.DEBUG.Printf("got cert from %s, parsing...", certURL)

	certObj, err := x509.ParseCertificate(der[0])
	if err != nil {
		jww.ERROR.Printf("cannot parse cert: %+v", err)
		return nil, nil, err
	}

	err = certObj.VerifyHostname(d.Domain)
	if err != nil {
		jww.ERROR.Printf("cannot verify cert hostname: %+v", err)
		return nil, nil, err
	}

	cert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE",
		Bytes: der[0]})

	return cert, d.PrivateKey.MarshalPem(), nil
}

func getAuthOrder(client *acme.Client,
	authzIDs []acme.AuthzID) (*acme.Order, error) {
	ctx, cancelFn := getDefaultContext()
	defer cancelFn()
	return client.AuthorizeOrder(ctx, authzIDs)
}

func getDNSChallenge(client *acme.Client, order *acme.Order) (*acme.Challenge,
	string, error) {
	for i := 0; i < len(order.AuthzURLs); i++ {
		authzURL := order.AuthzURLs[i]
		authz := getAuth(client, authzURL)
		if authz.Status == acme.StatusValid {
			return nil, authzURL, nil
		}
		c := findDNSChallenge(authz.Challenges)
		if c != nil {
			return c, order.AuthzURLs[i], nil
		}
	}
	return nil, "", errors.Errorf("no dns challenge available")
}

func getAuth(client *acme.Client, authzURL string) *acme.Authorization {
	ctx, cancelFn := getDefaultContext()
	defer cancelFn()
	authz, err := client.GetAuthorization(ctx, authzURL)
	if err != nil {
		jww.WARN.Printf("error retriving authz %s: %+v", authzURL, err)
		return nil
	}
	return authz
}

func findDNSChallenge(challenges []*acme.Challenge) *acme.Challenge {
	for i := 0; i < len(challenges); i++ {
		challenge := challenges[i]
		jww.DEBUG.Printf("Challenge Type: %s", challenge.Type)
		if challenge.Type == "dns-01" {
			return challenge
		}
	}
	return nil
}

func acceptDNSChallenge(client *acme.Client,
	dns01 *acme.Challenge) (*acme.Challenge, error) {
	ctx, cancelFn := getDefaultContext()
	defer cancelFn()
	return client.Accept(ctx, dns01)
}

func waitForAuthorization(client *acme.Client,
	authzURL string) *acme.Authorization {
	for {
		ctx, cancelFn := context.WithTimeout(context.Background(),
			1*time.Minute)
		authz, err := client.WaitAuthorization(ctx, authzURL)
		if err != nil {
			jww.DEBUG.Printf("WaitAuthorization: %s, continuing...",
				err.Error())
			continue
		}
		cancelFn()
		return authz
	}
}
