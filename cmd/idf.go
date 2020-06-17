///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"github.com/pkg/errors"
	"gitlab.com/elixxir/primitives/id"
	"gitlab.com/elixxir/primitives/id/idf"
	"gitlab.com/elixxir/primitives/ndf"
)

// writeIDF writes the identity file for the gateway into the given location
func writeIDF(ndf *ndf.NetworkDefinition, index int, idfPath string) error {
	// Create IDF based on NDF ID
	zeroSalt := make([]byte, 32)
	gwID, err := id.Unmarshal(ndf.Gateways[index].ID)
	// Save new ID to file
	if err == nil {
		err = idf.LoadIDF(idfPath, zeroSalt, gwID)
	}
	if err != nil {
		errors.Errorf("Failed to save IDF: %+v", err)
	}
	return nil
}
