package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/comms/mixmessages"
	crypto "gitlab.com/elixxir/crypto/authorize"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/crypto/rsa"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/crypto/csprng"
	rsa2 "gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gorm.io/gorm"
	"strings"
	"time"
)

const CertificateStateKey = "https_certificate"
const DnsTemplate = "%s.mainnet.cmix.rip"
const httpsEmail = "admins@xx.network"
const httpsCountry = "US"

// StartHttpsServer gets a well-formed tls certificate and provides it to
// protocomms so it can start to listen for HTTPS
func (gw *Instance) StartHttpsServer() error {
	// Get tls certificate and key
	cert, key, err := gw.getHttpsCreds()
	if err != nil {
		return err
	}

	// Pass the issued cert & key to protocomms so it can start serving https
	err = gw.Comms.ProtoComms.ProvisionHttps(cert, key)
	if err != nil {
		return err
	}

	return gw.setGatewayTlsCertificate(cert)
}

// getHttpsCreds is a helper for getting the tls certificate and key to pass
// into protocomms.  It will attempt to load from storage, or get a cert
// via zerossl if one is not found
func (gw *Instance) getHttpsCreds() ([]byte, []byte, error) {
	// Check states table for cert
	loadedCert, loadedKey, err := loadHttpsCreds(gw.storage)
	if err != nil {
		return nil, nil, err
	}
	if loadedCert != nil && loadedKey != nil {
		return loadedCert, loadedKey, nil
	}

	// Get Authorizer host
	authHost, ok := gw.Comms.GetHost(&id.Authorizer)
	if !ok {
		if gw.Params.AuthorizerAddress == "" {
			return nil, nil, errors.New("authorizer address is blank; cannot create host")
		} else {
			authHost, err = gw.Comms.AddHost(&id.Authorizer, gw.Params.AuthorizerAddress,
				[]byte(gw.NetInf.GetFullNdf().Get().Registration.TlsCertificate), connect.GetDefaultHostParams())
			if err != nil {
				return nil, nil, errors.WithMessage(err, "Failed to add authorizer host")
			}
		}
	}

	// Request EAB credentials
	eabCredResp, err := gw.Comms.SendEABCredentialRequest(authHost,
		&mixmessages.EABCredentialRequest{})
	if err != nil {
		return nil, nil, err
	}

	pk := rsa.GetScheme().Convert(&gw.Comms.GetPrivateKey().PrivateKey)
	err = gw.autoCert.Register(pk, eabCredResp.KeyId, eabCredResp.Key,
		httpsEmail)
	if err != nil {
		return nil, nil, err
	}

	// Generate DNS name
	dnsName := fmt.Sprintf(DnsTemplate, base64.URLEncoding.EncodeToString(gw.Comms.GetId().Marshal()))

	// Get ACME token
	chalDomain, challenge, err := gw.autoCert.Request(dnsName)
	if err != nil {
		return nil, nil, err
	}

	jww.INFO.Printf("ADD TXT RECORD: %s\t%s\n", chalDomain, challenge)

	ts := time.Now()

	// Sign ACME token
	rng := csprng.NewSystemRNG()
	sig, err := crypto.SignCertRequest(rng, gw.Comms.GetPrivateKey(), challenge, ts)
	if err != nil {
		return nil, nil, err
	}

	// Send ACME token to name server
	_, err = gw.Comms.SendAuthorizerCertRequest(authHost,
		&mixmessages.AuthorizerCertRequest{
			GwID:      gw.Comms.GetId().Bytes(),
			Timestamp: ts.UnixNano(),
			ACMEToken: challenge,
			Signature: sig,
		})
	if err != nil {
		return nil, nil, err
	}

	csrPem, csrDer, err := gw.autoCert.CreateCSR(dnsName, httpsEmail,
		httpsCountry, gw.Comms.GetId().String(), rng)
	if err != nil {
		return nil, nil, err
	}

	jww.INFO.Printf("Received CSR from autocert:\n\t%s", string(csrPem))

	// Get issued certificate and key from autoCert
	issuedCert, issuedKey, err := gw.autoCert.Issue(csrDer)
	if err != nil {
		return nil, nil, err
	}

	// Store the issued credentials in the states table
	err = storeHttpsCreds(issuedCert, issuedKey, gw.storage)
	if err != nil {
		return nil, nil, err
	}

	return issuedCert, issuedKey, nil
}

/* HTTPS helpers */

// storedHttpsCreds is an internal type, used to encapsulate the
// key and cert issued for https
type storedHttpsCreds struct {
	Key  []byte
	Cert []byte
}

// storeHttpCreds attempts to store a cert and key in the states table
func storeHttpsCreds(cert, key []byte, db *storage.Storage) error {
	internal := &storedHttpsCreds{
		Cert: cert,
		Key:  key,
	}
	marshalled, err := json.Marshal(internal)
	if err != nil {
		return err
	}

	return db.UpsertState(&storage.State{
		Key:   CertificateStateKey,
		Value: string(marshalled),
	})
}

// loadHttpsCreds attempts to load a cert and key from the states table
func loadHttpsCreds(db *storage.Storage) ([]byte, []byte, error) {
	val, err := db.GetStateValue(CertificateStateKey)
	if err != nil && !strings.Contains(err.Error(), gorm.ErrRecordNotFound.Error()) &&
		!strings.Contains(err.Error(), "Unable to locate state for key") {
		return nil, nil, err
	}
	if val == "" {
		return nil, nil, nil
	}
	loaded := &storedHttpsCreds{}
	err = json.Unmarshal([]byte(val), loaded)
	if err != nil {
		return nil, nil, err
	}
	return loaded.Cert, loaded.Key, nil
}

// Helper function which accepts the certificate used for https, signs it,
// and sets the GatewayCertificate on the Instance object to be sent when
// clients request it
func (gw *Instance) setGatewayTlsCertificate(cert []byte) error {
	h, err := hash.NewCMixHash()
	if err != nil {
		return err
	}
	h.Write(cert)
	sig, err := rsa2.Sign(csprng.NewSystemRNG(), gw.Comms.GetPrivateKey(), hash.CMixHash, h.Sum(nil), rsa2.NewDefaultOptions())
	if err != nil {
		return err
	}
	gw.gatewayCert = &mixmessages.GatewayCertificate{
		Certificate: cert,
		Signature:   sig,
	}
	return nil
}
