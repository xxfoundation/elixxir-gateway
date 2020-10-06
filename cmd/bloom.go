///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////
package cmd

import (
	"github.com/pkg/errors"
	bloom "gitlab.com/elixxir/bloomfilter"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/xx_network/primitives/id"
	"time"
)

// This file will handle the logistics of maintaining, creating and deleting user bloom filters

// Constants for constructing a bloom filter
const bloomFilterSize = 71888 // In Bits
const bloomFilterHashes = 8

var LastBloomFilterClearance time.Time

//func (gw *Instance) CleanBloomFilters() {
//	// todo: implement me
//	//scheduler := gocron.NewScheduler()
//	//j, _ := scheduler.NextRun()
//	//j.From(&LastBloomFilterClearance)
//	//j.Do(gw.cleanBloomFilters)
//}
//
//// Globally clears out all but one of the existing bloom filters
//// Happens on a weekly basis
//func (gw *Instance) cleanBloomFilters() error {
//	//gw.database.DeleteBloomFilterBYDate()
//
//	// Once the bloom filters have been clear out
//	//  reset the time
//	LastBloomFilterClearance = time.Now()
//	// Marshal the clearance time to a string
//	marshaledClearanceTime, err := LastBloomFilterClearance.MarshalText()
//	if err != nil {
//		return errors.Errorf("Could not marshal time into a string: %s", err)
//	}
//
//	// Write this new clearance time to file
//	err = utils.WriteFile(gw.Params.BloomFilterTrackerPath, marshaledClearanceTime, utils.FilePerms, utils.DirPerms)
//	if err != nil {
//		return errors.Errorf("Could not write bloom filter clearance to file: %s", err)
//	}
//	return nil
//}

// Initializes a  bloom filter on a client ID, and inserts into database
func (gw *Instance) CreateBloomFilter(clientId *id.ID) error {

	// See if we recognize this user
	retrievedClient, err := gw.database.GetClient(clientId)
	if err != nil || retrievedClient == nil {
		// If we do not recognize the client, create an ephemeral filter
		return gw.createEphemeralBloomFilter(clientId)
	}

	// If we do recognize the client, create a filter on the client
	return gw.createUserBloomFilter(clientId, retrievedClient)

}

// Creates a bloom filter on a known client,
func (gw *Instance) createUserBloomFilter(clientId *id.ID, retrievedClient *storage.Client) error {

	// Initialize a new bloom filter
	newBloom, err := bloom.InitByParameters(bloomFilterSize, bloomFilterHashes)
	if err != nil {
		return errors.Errorf("Could not initialize new bloom filter: %s", err)
	}

	// Marshal the new bloom filter
	marshaledBloom, err := newBloom.MarshalBinary()
	if err != nil {
		return errors.Errorf("Could not marshal new bloom filter: %s", err)
	}

	countOfBloomFilters := uint64(len(retrievedClient.Filters))

	// Insert the bloom filter into the database
	// fixme: Likely to change due to DB restructure
	err = gw.database.InsertBloomFilter(&storage.BloomFilter{
		ClientId:    clientId.Bytes(),
		Count:       countOfBloomFilters + 1,
		Filter:      marshaledBloom,
		DateCreated: time.Now(),
	})

	if err != nil {
		return errors.Errorf("Could not insert User Bloom Filter into database: %s", err)
	}

	return nil
}

// Creates a bloom filter on a ephemeral user
func (gw *Instance) createEphemeralBloomFilter(clientId *id.ID) error {
	// Initialize a new bloom filter
	newBloom, err := bloom.InitByParameters(bloomFilterSize, bloomFilterHashes)
	if err != nil {
		return errors.Errorf("Could not initialize new bloom filter: %s", err)
	}

	// Marshal the new bloom filter
	marshaledBloom, err := newBloom.MarshalBinary()
	if err != nil {
		return errors.Errorf("Could not marshal new bloom filter: %s", err)
	}
	// fixme: Likely to change due to DB restructure
	err = gw.database.InsertEphemeralBloomFilter(&storage.EphemeralBloomFilter{
		RecipientId:  clientId.Bytes(),
		Filter:       marshaledBloom,
		DateModified: time.Now(),
	})

	if err != nil {
		return errors.Errorf("Could not insert Ephemeral Bloom Filter into database: %s", err)
	}

	return nil

}
