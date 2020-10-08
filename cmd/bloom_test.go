///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////
package cmd

import (
	"encoding/binary"
	bloom "gitlab.com/elixxir/bloomfilter"
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

	// Create a mock client
	testClientId := id.NewIdFromString("0", id.User, t)

	// Pull a bloom filter from the database on the client ID BEFORE INSERTION
	retrievedFilters, err := gw.database.GetBloomFilters(testClientId)

	// Check that this filter is nil
	if err == nil || retrievedFilters != nil {
		t.Errorf("Should not get test client from storage prior to insertion.")
	}

	// Create a bloom filter on this client ID
	err = gw.upsertUserFilter(testClientId, 0)
	if err != nil {
		t.Errorf("Failed to create user bloom filter: %s", err)
	}

	// Pull a bloom filter from the database on the client ID AFTER INSERTION
	retrievedFilters, err = gw.database.GetBloomFilters(testClientId)
	if err != nil {
		t.Errorf("Could not get filters from storage: %s", err)
	}

	// Check that it is of the expected length and not nil
	if retrievedFilters == nil || len(retrievedFilters) != 1 {
		t.Errorf("Retrieved client did not store new bloom filter")
	}

}

// Happy path
func TestInstance_upsertEphemeralFilter(t *testing.T) {
	// Create gateway instance
	params := Params{
		NodeAddress:    NODE_ADDRESS,
		ServerCertPath: testkeys.GetNodeCertPath(),
		CertPath:       testkeys.GetGatewayCertPath(),
		MessageTimeout: 10 * time.Minute,
	}
	gw := NewGatewayInstance(params)

	// Create a mock client
	testClientId := id.NewIdFromString("0", id.User, t)

	retrievedFilters, err := gw.database.GetEphemeralBloomFilters(testClientId)
	// Check that this filter is nil
	if err == nil || retrievedFilters != nil {
		t.Errorf("Should not get test client from storage prior to insertion.")
	}

	err = gw.upsertEphemeralFilter(testClientId, 0)
	if err != nil {
		t.Errorf("Failed to create ephemeral bloom filter: %s", err)
	}

	retrievedFilters, err = gw.database.GetEphemeralBloomFilters(testClientId)
	if err != nil {
		t.Errorf("Could not get filters from storage: %s", err)
	}

	if retrievedFilters == nil || len(retrievedFilters) != 1 {
		t.Errorf("Retrieved ehphemeral filter was not expected. Should be non-nil an dlength of 1")
	}

	err = gw.upsertEphemeralFilter(testClientId, 1)
	if err != nil {
		t.Errorf("Failed to create ephemeral bloom filter: %s", err)

	}

	retrievedFilters, err = gw.database.GetEphemeralBloomFilters(testClientId)
	if err != nil {
		t.Errorf("Could not get filters from storage: %s", err)
	}

	bloomFilter, err := bloom.InitByParameters(bloomFilterSize, bloomFilterHashes)
	if err != nil {
		t.Errorf("Failed to create bloom filter: %s", err)
	}
	err = bloomFilter.UnmarshalBinary(retrievedFilters[0].Filter)
	if err != nil {
		t.Errorf("Could not get unmarshal bloom filter: %s", err)
	}

	// fixme: better way to do this? look into internals of bloom filter
	roundOne := make([]byte, 8)
	binary.LittleEndian.PutUint64(roundOne, uint64(1))

	roundTwo := make([]byte, 8)
	binary.LittleEndian.PutUint64(roundTwo, uint64(1))

	if bloomFilter.Test(roundOne) && bloomFilter.Test(roundTwo) {
		t.Error("Does not have expected rounds")
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

	// Create a mock client
	testClientId := id.NewIdFromString("0", id.User, t)

	// Check that the databases are empty for ephemeral filters
	retrievedEphFilters, err := gw.database.GetEphemeralBloomFilters(testClientId)
	// Check that this filter is nil
	if err == nil || retrievedEphFilters != nil {
		t.Errorf("Should not get test client from storage prior to insertion.")
	}

	// Check that the databases are empty for ephemeral filters
	retrievedUserFilters, err := gw.database.GetBloomFilters(testClientId)

	// Check that this filter is nil
	if err == nil || retrievedUserFilters != nil {
		t.Errorf("Should not get test client from storage prior to insertion.")
	}

	// This should result in an ephemeral bloom filter being created
	err = gw.UpsertFilter(testClientId, 0)
	if err != nil {
		t.Errorf("Could not create a bloom filter: %v", err)
	}

	// Check that an ephemeral bloom filter has been created
	retrievedEphFilters, err = gw.database.GetEphemeralBloomFilters(testClientId)
	if retrievedEphFilters == nil || len(retrievedEphFilters) != 1 {
		t.Errorf("Retrieved ehphemeral filter was not expected. Should be non-nil an dlength of 1")
	}

	// Insert client into database
	err = gw.database.InsertClient(&storage.Client{
		Id: testClientId.Bytes(),
	})
	if err != nil {
		t.Errorf("Failed to insert client into storage: %s", err)
	}

	// This should create a user bloom filter
	err = gw.UpsertFilter(testClientId, 0)
	if err != nil {
		t.Errorf("Could not create a bloom filter: %v", err)
	}

	// Check that a user bloom filter has been created
	retrievedUserFilters, err = gw.database.GetBloomFilters(testClientId)
	if retrievedUserFilters == nil || len(retrievedUserFilters) != 1 {
		t.Errorf("Retrieved user filter was not expected. Should be non-nil an dlength of 1")
	}

	// Check that an ephemeral bloom filter has not been modified by the insertion
	retrievedEphFilters, err = gw.database.GetEphemeralBloomFilters(testClientId)
	if retrievedEphFilters == nil || len(retrievedEphFilters) != 1 {
		t.Errorf("Retrieved ehphemeral filter was not expected. Should be non-nil an dlength of 1")
	}

}
