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
	"io"

	"gitlab.com/elixxir/crypto/rsa"
)

// Client autocert interface provides a simplified ACME Client
// with the ability to create a new request,
//
// To Use, you generate a private key, then:
//  1. Register with the server using EAB credentials.
//  2. Challenge which accepts and returns the challenge information.
//  3. Cert which waits until the server accepts and authorizes
//     your challenge, then requests and returns your cert in PEM format.
type Client interface {
	// Register authorizes this private key with the server.
	// eabKeyID is the key ID for External Account Binding and is a string
	// eabKey is a base64 raw encoded string for External Account Binding
	// email is the e-mail address to use when registering.
	// when nil is returned, the Registration succeeded and can continue
	// onto the Request step.
	Register(privateKey rsa.PrivateKey, eabKeyID, eabKey, email string) error

	// Request retrieves and accepts the appropriate ACME challenge for this
	// Client, and returns the challenge string (e.g., DNS Token to set)
	Request(domain string) (key, value string, err error)

	// Issue blocks until the challenge is accepted by the remote server,
	// and returns a certificate based on the private key and the key in PEM
	// format.
	Issue(csr []byte) (cert, key []byte, err error)

	// CreateCSR generates an issuer compliant certificate signed request
	CreateCSR(domain, email, country, nodeID string, rng io.Reader) (csrPEM,
		csrDER []byte, err error)
}
