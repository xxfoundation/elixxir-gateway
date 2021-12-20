///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package ipAddress

import (
	"github.com/pkg/errors"
	"strconv"
	"strings"
)

// StringToByte will convert an IP address into a byte slice.
// Example "1.2.3.4" -> []byte{1,2,3,4}.
func StringToByte(ipAddr string) ([]byte, error) {
	// Split IP values separated by the '.' delimiter
	addrVals := strings.Split(ipAddr, ".")

	// Check validity of address by ensuring 4 values
	if len(addrVals) != 4 {
		return nil, errors.Errorf("Invalid input, %s is not recognized as an IP", ipAddr)
	}

	// Initialize byte slice
	b := make([]byte, 4)

	// Iterate through each value
	for i, addrVal := range addrVals {
		// Convert to byte
		addr, err := strconv.Atoi(addrVal)
		if err != nil {
			return nil, errors.WithMessagef(err, "Could not convert IP address (%s) to byte data", ipAddr)
		}

		// Place in byte array
		b[i] = byte(addr)
	}

	// Return IP address as byte data
	return b, nil
}

// ByteToString converts a byte representation of an IP address to a string.
// Example: []byte{1,2,3,4} -> "1.2.3.4".
func ByteToString(ipAddr []byte) (string, error) {
	// Check validity of address by ensuring 4 values
	if len(ipAddr) != 4 {
		return "", errors.Errorf("Invalid input, %s is not recognized as an IP", ipAddr)
	}

	// Convert each value to a string, place in a slice of strings
	addrVals := make([]string, 4)
	for i, b := range ipAddr {
		addrVals[i] = strconv.Itoa(int(b))
	}

	// Return joined string slice by "." delimiter
	return strings.Join(addrVals, "."), nil

}
