///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// Testing file for extendedRoundStorage.go functions

package storage

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/xx_network/primitives/id"
	"testing"
)

// Tests the ERS wrapper functions in one test
// Testing them all individually relies either on the Store function working
// or copy pasting the entire Store function code and embedding the full
// function into each test.
func TestERS(t *testing.T) {
	// Setup a database based on a map impl
	m := &MapImpl{
		rounds: map[id.Round]*Round{},
	}

	// Create a fake round info to store
	origR10 := pb.RoundInfo{
		ID:                         10,
		UpdateID:                   7,
		BatchSize:                  9,
		ResourceQueueTimeoutMillis: 18,
	}

	// Store a round
	ers := Storage{m}
	err := ers.Store(&origR10)
	if err != nil {
		t.Error(err)
	}

	// Grab that round
	grabR10, err := ers.Retrieve(id.Round(10))
	if err != nil {
		t.Error(err)
	}
	if grabR10.ID != origR10.ID && grabR10.UpdateID != origR10.UpdateID && grabR10.BatchSize != origR10.BatchSize &&
		grabR10.ResourceQueueTimeoutMillis != origR10.ResourceQueueTimeoutMillis {
		t.Error("Grabbed round object does not look to be the same as the stored one")
	}

	// Grab a round that doesn't exist and check it silently fails
	_, err = ers.Retrieve(id.Round(5))
	if err != nil {
		t.Error("Retrieve did not silently fail on getting non-existent round.", err)
	}

	// Create and store two more fake round infos
	origR8 := pb.RoundInfo{
		ID:                         8,
		UpdateID:                   2,
		BatchSize:                  5,
		ResourceQueueTimeoutMillis: 23,
	}
	origR7 := pb.RoundInfo{
		ID:                         7,
		UpdateID:                   43,
		BatchSize:                  2,
		ResourceQueueTimeoutMillis: 39,
	}
	err = ers.Store(&origR8)
	if err != nil {
		t.Error(err)
	}
	err = ers.Store(&origR7)
	if err != nil {
		t.Error(err)
	}

	// Test RetrieveMany
	rounds, err := ers.RetrieveMany([]id.Round{10, 9, 8, 7})
	if err != nil {
		t.Error(err)
	}
	if rounds[0].ID != origR10.ID && rounds[0].UpdateID != origR10.UpdateID && rounds[0].BatchSize != origR10.BatchSize &&
		rounds[0].ResourceQueueTimeoutMillis != origR10.ResourceQueueTimeoutMillis {
		t.Error("Grabbed round object does not look to be the same as the stored one")
	}
	if rounds[1] != nil {
		t.Error("Did not receive placeholder for rid 9")
	}
	if rounds[2].ID != origR8.ID && rounds[2].UpdateID != origR8.UpdateID && rounds[2].BatchSize != origR8.BatchSize &&
		rounds[2].ResourceQueueTimeoutMillis != origR10.ResourceQueueTimeoutMillis {
		t.Error("Grabbed round object does not look to be the same as the stored one")
	}
	if rounds[3].ID != origR7.ID && rounds[3].UpdateID != origR7.UpdateID && rounds[3].BatchSize != origR7.BatchSize &&
		rounds[3].ResourceQueueTimeoutMillis != origR7.ResourceQueueTimeoutMillis {
		t.Error("Grabbed round object does not look to be the same as the stored one")
	}

	// Test RetrieveRange
	rounds, err = ers.RetrieveRange(7, 10)
	if err != nil {
		t.Error(err)
	}
	if rounds[3].ID != origR10.ID && rounds[3].UpdateID != origR10.UpdateID && rounds[3].BatchSize != origR10.BatchSize &&
		rounds[3].ResourceQueueTimeoutMillis != origR10.ResourceQueueTimeoutMillis {
		t.Error("Grabbed round object does not look to be the same as the stored one")
	}
	if rounds[2] != nil {
		t.Error("Did not receive placeholder for rid 9")
	}
	if rounds[1].ID != origR8.ID && rounds[1].UpdateID != origR8.UpdateID && rounds[1].BatchSize != origR8.BatchSize &&
		rounds[1].ResourceQueueTimeoutMillis != origR10.ResourceQueueTimeoutMillis {
		t.Error("Grabbed round object does not look to be the same as the stored one")
	}
	if rounds[0].ID != origR7.ID && rounds[0].UpdateID != origR7.UpdateID && rounds[0].BatchSize != origR7.BatchSize &&
		rounds[0].ResourceQueueTimeoutMillis != origR7.ResourceQueueTimeoutMillis {
		t.Error("Grabbed round object does not look to be the same as the stored one")
	}

	rounds, err = ers.RetrieveRange(10, 7)
	if err == nil {
		t.Error("Should have received an error when first is greater than last")
	}
}
