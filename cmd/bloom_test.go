///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////
package cmd

import (
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/xx_network/primitives/id"
	"testing"
)

// Happy path
func TestInstance_CreateUserBloomFilter(t *testing.T) {
	// Create a mock client
	testClientId := id.NewIdFromString("0", id.User, t)
	storedClient := &storage.Client{
		Id:      []byte("test"),
		Key:     nil,
		Filters: nil,
	}

	// Pull a bloom filter from the database on the client ID BEFORE INSERTION
	retrievedFilters, err := gatewayInstance.database.GetBloomFilters(testClientId)

	// Check that this filter is nil
	if err == nil || retrievedFilters != nil {
		t.Errorf("Should not get test client from storage prior to insertion.")
	}

	// Create a bloom filter on this client ID
	err = gatewayInstance.createUserBloomFilter(testClientId, storedClient)
	if err != nil {
		t.Errorf("Failed to create user bloom filter: %s", err)
	}

	// Pull a bloom filter from the database on the client ID AFTER INSERTION
	retrievedFilters, err = gatewayInstance.database.GetBloomFilters(testClientId)
	if err != nil {
		t.Errorf("Could not get test client from storage: %s", err)
	}

	// Check that it is of the expected length and not nil
	if retrievedFilters == nil || len(retrievedFilters) != 1 {
		t.Errorf("Retrieved client did not store new bloom filter")
	}

}
