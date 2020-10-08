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
		err := gw.UpsertFilter(recipient.Bytes(), roundId)
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
func (gw *Instance) UpsertFilter(recipient []byte, roundId id.Round) error {
	// Marshal the id
	recipientId, err := id.Unmarshal(recipient)
	if err != nil {
		return err
	}

	// See if we recognize this user
	retrievedClient, err := gw.database.GetClient(recipientId)
	if err != nil || retrievedClient == nil {
		// If we do not recognize the client, create an ephemeral filter
		return gw.upsertEphemeralFilter(recipientId, roundId)
	}

	// If we do recognize the client, create a filter on the client
	return gw.upsertUserFilter(recipientId, roundId)

}

// Helper function which updates the clients bloom filter
func (gw *Instance) upsertUserFilter(clientId *id.ID, roundId id.Round) error {
	// Get the filters for the associated client
	filters, err := gw.database.GetBloomFilters(clientId)
	if err != nil {
		newUserFilter, err := generateNewUserFilter(clientId, roundId)
		if err != nil {
			return err
		}
		return gw.database.InsertBloomFilter(newUserFilter)
	}

	// Go through the filters and update each one
	for _, filter := range filters {
		bloomFilter, err := bloom.InitByParameters(bloomFilterSize, bloomFilterHashes)
		if err != nil {
			return err
		}
		err = bloomFilter.UnmarshalBinary(filter.Filter)
		if err != nil {
			return err
		}

		// fixme: better way to do this? look into internals of bloom filter
		//  consider adding this as a marshal function internal to bloomFilter
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(roundId))

		bloomFilter.Add(b)

		marshaledFilter, err := bloomFilter.MarshalBinary()
		if err != nil {
			return err
		}

		// fixme: is this right? I don't see an upsert. Would it make more sense to
		//  call insert client with a list of updated filters?
		// fixme: Likely to change due to DB restructure
		err = gw.database.InsertBloomFilter(&storage.BloomFilter{
			ClientId:    clientId.Bytes(),
			Count:       filter.Count,
			Filter:      marshaledFilter,
			DateCreated: filter.DateCreated,
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// Helper function which updates the ephemeral bloom filter
func (gw *Instance) upsertEphemeralFilter(clientId *id.ID, roundId id.Round) error {
	filters, err := gw.database.GetEphemeralBloomFilters(clientId)
	if err != nil {
		newEphemeralFilter, err := generateNewEphemeralFilter(clientId, roundId)
		if err != nil {
			return err
		}
		return gw.database.InsertEphemeralBloomFilter(newEphemeralFilter)
	}

	// Iterate through the filters, updating each one
	for _, filter := range filters {
		bloomFilter, err := bloom.InitByParameters(bloomFilterSize, bloomFilterHashes)
		if err != nil {
			return err
		}
		err = bloomFilter.UnmarshalBinary(filter.Filter)
		if err != nil {
			return err
		}

		// Add the round to the bloom filter
		// fixme: better way to do this? look into internals of bloom filter
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(roundId))

		bloomFilter.Add(b)

		// Marshal the bloom filter back
		marshaledBloom, err := bloomFilter.MarshalBinary()
		if err != nil {
			return err
		}

		// fixme: Likely to change due to DB restructure
		err = gw.database.InsertEphemeralBloomFilter(&storage.EphemeralBloomFilter{
			Id:           filter.Id,
			RecipientId:  clientId.Bytes(),
			Filter:       marshaledBloom,
			DateModified: time.Now(),
		})
		if err != nil {
			return err
		}
	}

	return nil

}

// Helper function which generates a bloom filter with the round hashed into it
func generateNewEphemeralFilter(recipient *id.ID, roundId id.Round) (*storage.EphemeralBloomFilter, error) {
	// Initialize a new bloom filter
	newBloom, err := bloom.InitByParameters(bloomFilterSize, bloomFilterHashes)
	if err != nil {
		return &storage.EphemeralBloomFilter{},
			errors.Errorf("Unable to generate new bloom filter: %s", err)
	}

	// Add the round to the bloom filter
	// fixme: better way to do this? look into internals of bloom filter
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(roundId))

	newBloom.Add(b)

	// Add the round to the bloom filter
	// Marshal the new bloom filter
	marshaledBloom, err := newBloom.MarshalBinary()
	if err != nil {
		return &storage.EphemeralBloomFilter{},
			errors.Errorf("Unable to marshal new bloom filter: %s", err)
	}

	return &storage.EphemeralBloomFilter{
		RecipientId:  recipient.Bytes(),
		Filter:       marshaledBloom,
		DateModified: time.Now(),
	}, nil

}

// Helper function which generates a bloom filter with the round hashed into it
func generateNewUserFilter(recipient *id.ID, roundId id.Round) (*storage.BloomFilter, error) {
	// Initialize a new bloom filter
	newBloom, err := bloom.InitByParameters(bloomFilterSize, bloomFilterHashes)
	if err != nil {
		return &storage.BloomFilter{},
			errors.Errorf("Unable to generate new bloom filter: %s", err)
	}

	// Add the round to the bloom filter
	// fixme: better way to do this? look into internals of bloom filter
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(roundId))

	newBloom.Add(b)

	// Add the round to the bloom filter
	// Marshal the new bloom filter
	marshaledBloom, err := newBloom.MarshalBinary()
	if err != nil {
		return &storage.BloomFilter{},
			errors.Errorf("Unable to marshal new bloom filter: %s", err)
	}

	return &storage.BloomFilter{
		ClientId:    recipient.Bytes(),
		Count:       0,
		Filter:      marshaledBloom,
		DateCreated: time.Now(),
	}, nil

}
