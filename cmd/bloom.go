///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////
package cmd

import (
	"encoding/binary"
	"github.com/pkg/errors"
	bloom "gitlab.com/elixxir/bloomfilter"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/xx_network/primitives/id"
	"strings"
)

// This file will handle the logistics of maintaining, creating and deleting user bloom filters

// Constants for constructing a bloom filter
const bloomFilterSize = 71888 // In Bits
const bloomFilterHashes = 8

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
// TODO: will handle higher business logic when complex filter design
//  is ready
func (gw *Instance) UpsertFilter(recipientId *id.ID, roundId id.Round) error {
	return gw.upsertFilter(recipientId, roundId)
}

// Helper function which updates the clients bloom filter
func (gw *Instance) upsertFilter(recipientId *id.ID, roundId id.Round) error {
	// Get the latest epoch value
	epoch, err := gw.storage.GetLatestEpoch()
	if err != nil {
		return errors.Errorf("Unable to get latest epoch: %s", err)
	}

	// Get the filters for the associated client
	filters, err := gw.storage.GetBloomFilters(recipientId, roundId)
	if err != nil || filters == nil {
		// Generate a new filter
		newUserFilter, err := generateNewFilter(recipientId, roundId)
		if err != nil {
			return errors.Errorf("Unable to generate a new user filter: %v", err)
		}

		// Update the epoch it was created in
		newUserFilter.EpochId = epoch.Id

		// Upsert the filter to storage
		return gw.storage.UpsertBloomFilter(newUserFilter)
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
	err = gw.storage.UpsertBloomFilter(&storage.BloomFilter{
		RecipientId: recipientId.Bytes(),
		Filter:      marshaledFilter,
		EpochId:     epoch.Id,
	})

	if err != nil {
		return errors.Errorf("Unable to insert user filter into database: %v", err)
	}

	return nil
}

// Helper function which generates a bloom filter with the round hashed into it
func generateNewFilter(recipientId *id.ID, roundId id.Round) (*storage.BloomFilter, error) {
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
		RecipientId: recipientId.Bytes(),
		Filter:      marshaledBloom,
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
