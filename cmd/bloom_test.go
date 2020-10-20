///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////
package cmd

import (
	"gitlab.com/elixxir/comms/testkeys"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/xx_network/primitives/id"
	"testing"
	"time"
)

// Happy path
func TestInstance_upsertUserFilter(t *testing.T) {
	// Create gateway instance
	params := Params{
		NodeAddress:    NODE_ADDRESS,
		ServerCertPath: testkeys.GetNodeCertPath(),
		CertPath:       testkeys.GetGatewayCertPath(),
		MessageTimeout: 10 * time.Minute,
	}
	gw := NewGatewayInstance(params)
	rndId := id.Round(0)

	_, err := gw.storage.InsertEpoch(rndId)
	if err != nil {
		t.Errorf("Could not insert epoch: %v", err)
	}

	// Create a mock client
	testClientId := id.NewIdFromString("0", id.User, t)

	// Pull a bloom filter from the database on the client ID BEFORE INSERTION
	retrievedFilters, err := gw.storage.GetBloomFilters(testClientId, rndId)

	// Check that this filter is nil
	if err == nil || retrievedFilters != nil {
		t.Errorf("Should not get test client from storage prior to insertion.")
	}

	// Create a bloom filter on this client ID
	err = gw.upsertFilter(testClientId, rndId)
	if err != nil {
		t.Errorf("Failed to create user bloom filter: %s", err)
	}

	// Pull a bloom filter from the database on the client ID AFTER INSERTION
	retrievedFilters, err = gw.storage.GetBloomFilters(testClientId, rndId)
	if err != nil {
		t.Errorf("Could not get filters from storage: %s", err)
	}

	// Check that it is of the expected length and not nil
	if retrievedFilters == nil || len(retrievedFilters) != 1 {
		t.Errorf("Retrieved client did not store new bloom filter")
	}

	// Insert a client already
	err = gw.storage.InsertClient(&storage.Client{
		Id: testClientId.Bytes(),
	})
	if err != nil {
		t.Errorf("Could not load client into storage: %v", err)
	}

	// Create a bloom filter on this client ID
	err = gw.upsertFilter(testClientId, 1)
	if err != nil {
		t.Errorf("Failed to create user bloom filter: %s", err)
	}

	// Pull a bloom filter from the database on the client ID AFTER INSERTION
	retrievedFilters, err = gw.storage.GetBloomFilters(testClientId, rndId)
	if err != nil {
		t.Errorf("Could not get filters from storage: %s", err)
	}

	// Check that it is of the expected length and not nil
	if retrievedFilters == nil {
		t.Errorf("Retrieved client did not store new bloom filter")
	}

}

// Happy path
func TestInstance_UpsertFilters(t *testing.T) {
	// Create gateway instance
	params := Params{
		NodeAddress:    NODE_ADDRESS,
		ServerCertPath: testkeys.GetNodeCertPath(),
		CertPath:       testkeys.GetGatewayCertPath(),
		MessageTimeout: 10 * time.Minute,
	}
	gw := NewGatewayInstance(params)
	rndId := id.Round(0)

	// Create a mock client
	testClientId := id.NewIdFromString("0", id.User, t)

	// Check that the databases are empty for ephemeral filters
	retrievedFilter, err := gw.storage.GetBloomFilters(testClientId, rndId)
	// Check that this filter is nil
	if err == nil || retrievedFilter != nil {
		t.Errorf("Should not get test client from storage prior to insertion.")
	}

	gw.storage.InsertEpoch(rndId)

	// This should result in an ephemeral bloom filter being created
	err = gw.UpsertFilter(testClientId, 0)
	if err != nil {
		t.Errorf("Could not create a bloom filter: %v", err)
	}

	// Check that an ephemeral bloom filter has been created
	retrievedFilter, err = gw.storage.GetBloomFilters(testClientId, rndId)
	if retrievedFilter == nil || len(retrievedFilter) != 1 {
		t.Errorf("Retrieved ehphemeral filter was not expected. Should be non-nil an dlength of 1")
	}

}
