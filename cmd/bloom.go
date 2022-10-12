////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"encoding/binary"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	bloom "gitlab.com/elixxir/bloomfilter"
	"gitlab.com/elixxir/primitives/states"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"strings"
)

// This file will handle the logistics of maintaining, creating and deleting user bloom filters

// Constants for constructing a bloom filter
const bloomFilterSize = 648 // In Bits
const bloomFilterHashes = 10

// Upserts filters of passed in recipients, using the round ID
func (gw *Instance) UpsertFilters(recipients map[ephemeral.Id]interface{}, roundId id.Round) error {
	var errReturn error
	var errs []string

	// Get epoch information
	round, err := gw.NetInf.GetRound(roundId)
	if err != nil {
		return err
	}
	roundTimestamp := round.Timestamps[states.QUEUED]
	epoch := GetEpoch(int64(roundTimestamp), gw.period)

	for recipient := range recipients {
		err := gw.UpsertFilter(recipient, roundId, epoch)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		errReturn = errors.New(strings.Join(errs, errorDelimiter))
	}

	return errReturn
}

// Helper function which updates the clients bloom filter
func (gw *Instance) UpsertFilter(recipientId ephemeral.Id, roundId id.Round, epoch uint32) error {
	jww.DEBUG.Printf("Adding bloom filter for client %d on round %d with epoch %d",
		recipientId.Int64(), roundId, epoch)

	// Generate a new filter
	// Initialize a new bloom filter
	newBloom, err := bloom.InitByParameters(bloomFilterSize, bloomFilterHashes)
	if err != nil {
		return errors.Errorf("Unable to generate new bloom filter: %s", err)
	}

	// Add the round to the bloom filter
	serializedRound := serializeRound(roundId)
	newBloom.Add(serializedRound)

	// Add the round to the bloom filter
	// Marshal the new bloom filter
	marshaledBloom, err := newBloom.MarshalBinary()
	if err != nil {
		return errors.Errorf("Unable to marshal new bloom filter: %s", err)
	}

	// Upsert the filter to storage
	return gw.storage.HandleBloomFilter(recipientId, marshaledBloom, roundId, epoch)
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
