///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////
package cmd

import (
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	bloom "gitlab.com/elixxir/bloomfilter"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/xx_network/primitives/id"
	"strings"
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

// Upserts filters of passed in recipients, using the round ID
func (gw *Instance) UpsertFilters(recipients []*id.ID, roundId id.Round) error {
	var errReturn error
	var errs []string
	for _, recipient := range recipients {
		err := gw.UpsertFilter(recipient, roundId)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		errReturn = errors.New(strings.Join(errs, errorDelimiter))
	}

	return errReturn
}

// Update function which updates a recipient's bloom filter.
//  If the client is recognized, we update the user directly
//  If the client is not recognized, we update the ephemeral bloom filter
func (gw *Instance) UpsertFilter(recipientId *id.ID, roundId id.Round) error {
	// See if we recognize this user or if they are ephemeral
	retrievedClient, err := gw.database.GetClient(recipientId)
	if err != nil || retrievedClient == nil {

		// If we do not recognize the client, create an ephemeral filter
		return gw.upsertEphemeralFilter(recipientId, roundId)
	}

	// If we do recognize the client, create a filter on the client
	return gw.upsertUserFilter(recipientId, roundId)

}

// Helper function which updates the clients bloom filter
func (gw *Instance) upsertUserFilter(recipientId *id.ID, roundId id.Round) error {
	// Get the filters for the associated client
	filters, err := gw.database.GetBloomFilters(recipientId)
	if err != nil || filters == nil {
		newUserFilter, err := generateNewUserFilter(recipientId, roundId)
		if err != nil {
			return errors.Errorf("Unable to generate a new user filter: %v", err)
		}
		return gw.database.InsertBloomFilter(newUserFilter)
	}

	// Pull the most recent filter
	recentFilter := filters[len(filters)-1]
	// Unmarshal the most recent filter
	bloomFilter, err := bloom.InitByParameters(bloomFilterSize, bloomFilterHashes)
	if err != nil {
		return errors.Errorf("Unable to create a bloom filter: %v", err)
	}
	err = bloomFilter.UnmarshalBinary(recentFilter.Filter)
	if err != nil {
		return errors.Errorf("Unable to unmarshal filter from storage: %v", err)
	}

	// Add the round to the bloom filter
	serializedRound := serializeRound(roundId)
	bloomFilter.Add(serializedRound)

	// Marshal the bloom filter back for storage
	marshaledFilter, err := bloomFilter.MarshalBinary()
	if err != nil {
		return errors.Errorf("Unable to marshal user filter: %v", err)
	}

	// fixme: Likely to change due to DB restructure
	// Place filter back into database
	err = gw.database.InsertBloomFilter(&storage.BloomFilter{
		ClientId:    recipientId.Bytes(),
		Count:       recentFilter.Count + 1,
		Filter:      marshaledFilter,
		DateCreated: recentFilter.DateCreated,
	})

	if err != nil {
		return errors.Errorf("Unable to insert user filter into database: %v", err)
	}

	return nil
}

// Helper function which updates the ephemeral bloom filter
func (gw *Instance) upsertEphemeralFilter(recipientId *id.ID, roundId id.Round) error {
	filters, err := gw.database.GetEphemeralBloomFilters(recipientId)
	if err != nil || filters == nil {
		newEphemeralFilter, err := generateNewEphemeralFilter(recipientId, roundId)
		if err != nil {
			return errors.Errorf("Unable to generate a new ephemeral filter: %v", err)
		}

		return gw.database.InsertEphemeralBloomFilter(newEphemeralFilter)

	}

	// Pull the most recent filter
	recentFilter := filters[len(filters)-1]
	fmt.Printf("recognized that this isn't first time!\n")
	// Unmarshal the most recent filter
	bloomFilter, err := bloom.InitByParameters(bloomFilterSize, bloomFilterHashes)
	if err != nil {
		return errors.Errorf("Unable to create a bloom filter: %v", err)
	}
	err = bloomFilter.UnmarshalBinary(recentFilter.Filter)
	if err != nil {
		return errors.Errorf("Unable to unmarshal filter from storage: %v", err)
	}

	// Add the round to the bloom filter
	serializedRound := serializeRound(roundId)
	bloomFilter.Add(serializedRound)

	// Marshal the bloom filter back
	marshaledBloom, err := bloomFilter.MarshalBinary()
	if err != nil {
		return errors.Errorf("Unable to marshal ephemeral filter: %v", err)
	}

	// fixme: Likely to change due to DB restructure
	// Place filter back into database
	err = gw.database.InsertEphemeralBloomFilter(&storage.EphemeralBloomFilter{
		Id:           recentFilter.Id,
		RecipientId:  recipientId.Bytes(),
		Filter:       marshaledBloom,
		DateModified: time.Now(),
	})
	if err != nil {
		return errors.Errorf("Unable to insert ephemeral filter into database: %v", err)
	}

	return nil

}

// Helper function which generates a bloom filter with the round hashed into it
func generateNewEphemeralFilter(recipientId *id.ID, roundId id.Round) (*storage.EphemeralBloomFilter, error) {
	// Initialize a new bloom filter
	newBloom, err := bloom.InitByParameters(bloomFilterSize, bloomFilterHashes)
	if err != nil {
		return &storage.EphemeralBloomFilter{},
			errors.Errorf("Unable to generate new bloom filter: %s", err)
	}

	// Add the round to the bloom filter
	serializedRound := serializeRound(roundId)
	newBloom.Add(serializedRound)

	// Add the round to the bloom filter
	// Marshal the new bloom filter
	marshaledBloom, err := newBloom.MarshalBinary()
	if err != nil {
		return &storage.EphemeralBloomFilter{},
			errors.Errorf("Unable to marshal new bloom filter: %s", err)
	}

	return &storage.EphemeralBloomFilter{
		RecipientId:  recipientId.Bytes(),
		Filter:       marshaledBloom,
		DateModified: time.Now(),
	}, nil

}

// Helper function which generates a bloom filter with the round hashed into it
func generateNewUserFilter(recipientId *id.ID, roundId id.Round) (*storage.BloomFilter, error) {
	// Initialize a new bloom filter
	newBloom, err := bloom.InitByParameters(bloomFilterSize, bloomFilterHashes)
	if err != nil {
		return &storage.BloomFilter{},
			errors.Errorf("Unable to generate new bloom filter: %s", err)
	}

	// Add the round to the bloom filter
	serializedRound := serializeRound(roundId)
	newBloom.Add(serializedRound)

	// Add the round to the bloom filter
	// Marshal the new bloom filter
	marshaledBloom, err := newBloom.MarshalBinary()
	if err != nil {
		return &storage.BloomFilter{},
			errors.Errorf("Unable to marshal new bloom filter: %s", err)
	}

	return &storage.BloomFilter{
		ClientId:    recipientId.Bytes(),
		Count:       0,
		Filter:      marshaledBloom,
		DateCreated: time.Now(),
	}, nil

}

// Serializes a round into a byte array.
// fixme: Used as bloom filters requires insertion
//  of a byte array into the data structure
//  better way to do this? look into internals of bloom filter
//  likely a marshal function internal to the filter
func serializeRound(roundId id.Round) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(roundId))
	return b
}
