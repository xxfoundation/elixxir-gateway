package cmd

import (
	"bytes"
	"gitlab.com/elixxir/gateway/storage"
	"testing"
	"time"
)

func TestStoreHttpsCreds(t *testing.T) {
	db, err := storage.NewStorage("", "", "", "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	creds := storedHttpsCreds{
		Key:  []byte("TestKey"),
		Cert: []byte("TestCert"),
	}

	err = storeHttpsCreds(creds.Cert, creds.Key, db)
	if err != nil {
		t.Fatal(err)
	}

	cert, key, err := loadHttpsCreds(db)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(cert, creds.Cert) || !bytes.Equal(key, creds.Key) {
		t.Fatalf("Did not receive expected creds\n\tExpected: "+
			"\n\t\tKey: %+v\n\t\tCert: %+v\n\tReceived: \n\t\t"+
			"Key: %+v\n\t\tCert: %+v\n", creds.Key, creds.Cert, key, cert)
	}
}

func TestGetReplaceAt(t *testing.T) {
	day := time.Hour * 24
	notAfter := time.Now().Add(90 * day)
	t.Log(notAfter)
	replaceAt := getReplaceAt(notAfter, time.Hour*24*30, time.Hour*24*7)
	t.Log(replaceAt)
	if replaceAt.After(notAfter) {
		t.Fatalf("Replaceat %s should be before notAfter %s", replaceAt, notAfter)
	}
}
