///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package ipAddress

import (
	"bytes"
	"testing"
)

// Happy path.
func TestStringToByte(t *testing.T) {
	ipAddr := "1.2.3.4"

	expected := []byte{1, 2, 3, 4}

	recieved, err := StringToByte(ipAddr)
	if err != nil {
		t.Fatalf(err.Error())
	}

	if !bytes.Equal(expected, recieved) {
		t.Fatalf("Unexpected output converting IP address from string to byte."+
			"\n\tExpected: %v"+
			"\n\tReceived: %v", expected, recieved)
	}
}

// Error path.
func TestStringToByte2(t *testing.T) {
	invalidIpAddr := "1a.2b.3c.4d"

	_, err := StringToByte(invalidIpAddr)
	if err == nil {
		t.Fatalf("Expected error case, should not be able to convert %s to a byte slice", invalidIpAddr)
	}

}

// Happy path.
func TestByteToString(t *testing.T) {
	expected := "1.2.3.4"

	ipAddr := []byte{1, 2, 3, 4}

	received, err := ByteToString(ipAddr)
	if err != nil {
		t.Fatalf(err.Error())
	}

	if expected != received {
		t.Fatalf("Unexpected output converting IP address from byte to string."+
			"\n\tExpected: %v"+
			"\n\tReceived: %v", expected, received)

	}
}
