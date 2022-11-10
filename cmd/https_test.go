package cmd

import (
	"bytes"
	"gitlab.com/elixxir/gateway/storage"
	"gorm.io/gorm"
	"strings"
	"testing"
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

	cert, key, err := loadHttpsCreds(db)
	if err == nil {
		t.Fatalf("Did not receive error loading https creds when none are stored")
	}
	if !strings.Contains(err.Error(), gorm.ErrRecordNotFound.Error()) &&
		!strings.Contains(err.Error(), "Unable to locate state for key") {
		t.Fatalf("Did not receive expected error: %+v", err)
	}

	err = storeHttpsCreds(creds.Cert, creds.Key, db)
	if err != nil {
		t.Fatal(err)
	}

	cert, key, err = loadHttpsCreds(db)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(cert, creds.Cert) || !bytes.Equal(key, creds.Key) {
		t.Fatalf("Did not receive expected creds\n\tExpected: "+
			"\n\t\tKey: %+v\n\t\tCert: %+v\n\tReceived: \n\t\t"+
			"Key: %+v\n\t\tCert: %+v\n", creds.Key, creds.Cert, key, cert)
	}
}
