package cmd

import (
	"crypto"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/crypto/hash"
	rsa2 "gitlab.com/elixxir/crypto/rsa"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/crypto/csprng"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gorm.io/gorm"
	"io"
	"strings"
	"time"
)

const CertificateStateKey = "https_certificate"
const DnsTemplate = "%s.mainnet.cmix.rip"

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

	return gw.SetGatewayTlsCertificate(cert)
}

// getHttpsCreds is a helper for getting the tls certificate and key to pass
// into protocomms.  It will attempt to load from storage, or get a cert
// via zerossl if one is not found
func (gw *Instance) getHttpsCreds() ([]byte, []byte, error) {
	// Check states table for cert
	loadedCert, loadedKey, err := loadHttpsCreds(gw.storage)
	if err != nil &&
		!strings.Contains(err.Error(), gorm.ErrRecordNotFound.Error()) &&
		!strings.Contains(err.Error(), "Unable to locate state for key") {
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

	// TODO there has to be a better way to do this
	pk, err := rsa2.GetScheme().UnmarshalPrivateKeyPEM(
		rsa.CreatePrivateKeyPem(gw.Comms.GetPrivateKey()))
	if err != nil {
		return nil, nil, err
	}
	err = gw.autoCert.Register(pk, eabCredResp.KeyId, eabCredResp.Key,
		gw.Params.HttpsEmail)
	if err != nil {
		return nil, nil, err
	}

	// Generate DNS name
	dnsName := fmt.Sprintf(DnsTemplate, gw.Comms.GetId().String())

	// Get ACME token
	_, acmeToken, err := gw.autoCert.Request(dnsName) // TODO : do we need the key for anything?
	if err != nil {
		return nil, nil, err
	}

	ts := uint64(time.Now().UnixNano())

	// Sign ACME token
	rng := csprng.NewSystemRNG()
	sig, err := signAcmeToken(rng, gw.Comms.GetPrivateKey(),
		gw.Params.PublicAddress, acmeToken, ts)
	if err != nil {
		return nil, nil, err
	}

	// Send ACME token to name server
	_, err = gw.Comms.SendAuthorizerCertRequest(authHost,
		&mixmessages.AuthorizerCertRequest{
			GwID:      gw.Comms.GetId().Bytes(),
			Timestamp: ts,
			ACMEToken: acmeToken,
			Signature: sig,
		})
	if err != nil {
		return nil, nil, err
	}

	// TODO : do we need the der for something?
	csrPem, _, err := gw.autoCert.CreateCSR(dnsName, gw.Params.HttpsEmail,
		gw.Params.HttpsCountry, gw.Comms.GetId().String(), rng)
	if err != nil {
		return nil, nil, err
	}

	// Get issued certificate and key from autoCert
	issuedCert, issuedKey, err := gw.autoCert.Issue(csrPem)
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
	if err != nil {
		return nil, nil, err
	}
	loaded := &storedHttpsCreds{}
	err = json.Unmarshal([]byte(val), loaded)
	if err != nil {
		return nil, nil, err
	}
	return loaded.Cert, loaded.Key, nil
}

// signAcmeToken creates the signature sent with an AuthorizerCertRequest
func signAcmeToken(rng io.Reader, gwRsa *rsa.PrivateKey, ipAddress,
	acmeToken string, timestamp uint64) ([]byte, error) {
	hashType := hash.CMixHash
	h := hashType.New()
	h.Write([]byte(ipAddress))
	h.Write([]byte(acmeToken))
	tsBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBytes, timestamp)
	h.Write(tsBytes)
	return gwRsa.Sign(rng, h.Sum(nil), crypto.SignerOpts(hashType))
}

func (gw *Instance) SetGatewayTlsCertificate(cert []byte) error {
	rng := csprng.NewSystemRNG()
	hashType := hash.CMixHash
	h := hashType.New()
	h.Write(cert)
	sig, err := rsa.Sign(rng, gw.Comms.GetPrivateKey(), hashType, h.Sum(nil), rsa.NewDefaultOptions())
	if err != nil {
		return err
	}
	gw.gatewayCert = &mixmessages.GatewayCertificate{
		Certificate: cert,
		Signature:   sig,
	}
	return nil
}
