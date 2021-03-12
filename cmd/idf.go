///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"bytes"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/idf"
	"gitlab.com/xx_network/primitives/ndf"
)

// Helper that updates parses the NDF in order to create our IDF
func (gw *Instance) setupIDF(nodeId []byte, ourNdf *ndf.NetworkDefinition) (err error) {

	// Determine the index of this gateway
	for i, node := range ourNdf.Nodes {
		// Find our node in the ndf
		if bytes.Compare(node.ID, nodeId) == 0 {

			// Save the IDF to the idfPath
			err := writeIDF(ourNdf, i, idfPath)
			if err != nil {
				jww.WARN.Printf("Could not write ID File: %s",
					idfPath)
			}

			return nil
		}
	}

	return errors.Errorf("Unable to locate ID %v in NDF!", nodeId)
}

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
