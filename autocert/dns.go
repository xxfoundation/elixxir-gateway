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
	"crypto"
	"encoding/base64"
	"io"
	"time"

	"github.com/pkg/errors"

	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/crypto/rsa"
	"golang.org/x/crypto/acme"
	"golang.org/x/net/context"
)

const ZeroSSLACMEURL = "https://acme.zerossl.com/v2/DV90"

// GenerateCertKey generates a 4096 bit RSA Private Key that can be used in
// the certificate request.
func GenerateCertKey(csprng io.Reader) (crypto.Signer, error) {
	pKey, err := rsa.GetScheme().Generate(csprng, 4096)
	if err != nil {
		return nil, err
	}
	jww.INFO.Printf("%s", pKey.MarshalPem())

	return pKey.GetGoRSA(), nil
}

// NewClient returns a new acme client request client
func NewRequest(privateKey crypto.Signer, domain, eabKeyID, eabKey string) (*acme.Client, error) {

	// Let's rule out dumb mistakes and decode/create the external account
	// binding first.
	eabHMAC, err := base64.RawURLEncoding.DecodeString(eabKey)
	if err != nil {
		return nil, err
	}

	client := &acme.Client{
		Key:          privateKey,
		DirectoryURL: ZeroSSLACMEURL,
	}

	acctReq := &acme.Account{
		ExternalAccountBinding: &acme.ExternalAccountBinding{
			KID: eabKeyID,
			Key: eabHMAC,
		},
		Contact: []string{"mailto:admins@elixxir.io"},
	}

	// Note: this is wonky, because the account object sent is not modified
	// and a new one gets returned. A review of the internals shows that
	// only the ExternalAccountBinding and Contact objects are used
	ctx, cancelFn := context.WithTimeout(context.Background(),
		120*time.Second)
	defer cancelFn()
	acct, err := client.Register(ctx, acctReq, acme.AcceptTOS)
	if err != nil {
		return nil, err
	}
	jww.INFO.Print("Account Registered: %+v", acct)

	// ctx, cancelFn5 := context.WithTimeout(context.Background(),
	// 	120*time.Second)
	// defer cancelFn5()
	// discovery, err := client.Discover(ctx)
	// if err != nil {
	// 	jww.ERROR.Printf("Discovery failed: %+v", err)
	// 	return nil, err
	// }
	// jww.INFO.Printf("Discovery: %+v", discovery)

	authzIDs := []acme.AuthzID{
		acme.AuthzID{
			Type:  "dns",
			Value: domain,
		},
	}
	ctx, cancelFn2 := context.WithTimeout(context.Background(),
		120*time.Second)
	defer cancelFn2()
	order, err := client.AuthorizeOrder(ctx, authzIDs)
	if err != nil {
		jww.ERROR.Printf("Authorize failed: %+v", err)
		return nil, err
	}

	jww.INFO.Printf("Order: %+v", order)

	var dns01 *acme.Challenge
	var challenges []*acme.Challenge
	for i := 0; i < len(order.AuthzURLs); i++ {
		authzURL := order.AuthzURLs[i]
		authz, err := client.GetAuthorization(ctx, authzURL)
		if err != nil {
			return nil, err
		}
		challenges = append(challenges, authz.Challenges...)
	}
	for i := 0; i < len(challenges); i++ {
		challenge := challenges[i]
		jww.INFO.Printf("Challenge Type: %s", challenge.Type)
		if challenge.Type == "dns-01" {
			dns01 = challenge
			break
		}
	}

	if dns01 == nil {
		return nil, errors.Errorf("no dns challenge available")
	}

	jww.INFO.Printf("DNS Challenge: %+v", dns01)

	dns01, err = client.Accept(ctx, dns01)
	if err != nil {
		jww.ERROR.Printf("Accept DNS failed: %+v", err)
		return nil, err
	}

	dnsChallenge, err := client.DNS01ChallengeRecord(dns01.Token)
	if err != nil {
		jww.ERROR.Printf("DNS failed: %+v", err)
		return nil, err
	}

	jww.INFO.Printf("TXT Record:\n_acme-challenge\t%s", dnsChallenge)

	for {
		ctx, _ := context.WithTimeout(context.Background(),
			1*time.Minute)
		auth, err := client.GetAuthorization(ctx, order.AuthzURLs[0])
		if err == nil {
			jww.INFO.Print("Auth Status: %+v", auth)
		} else {
			jww.ERROR.Printf("Erro Auth Stat: %+v", err)
		}
		chal, err := client.GetChallenge(ctx, dns01.URI)
		if err == nil {
			jww.INFO.Print("chal Status: %+v", chal)
		} else {
			jww.ERROR.Printf("Erro Chall Stat: %+v", err)
		}
		time.Sleep(1 * time.Minute)
	}

	// jww.INFO.Printf("Auth Status: %+v", auth)

	return client, nil

}
