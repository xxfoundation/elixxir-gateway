////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Handles the high level storage API.
// This layer merges the business logic layer and the database layer

package storage

import (
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"time"
)

const (
	// Determines maximum runtime (in seconds) of specific DB queries
	dbTimeout = 3 * time.Second
	// Determines maximum number of uses for a BloomFilter in a given period.
	maxBloomUses = 100
)

// API for the storage layer
type Storage struct {
	// Stored database interface
	database
}

// Create a new Storage object wrapping a database interface
// Returns a Storage object and error
func NewStorage(username, password, dbName, address, port string, devmode bool) (*Storage, error) {
	db, err := newDatabase(username, password, dbName, address, port, devmode)
	storage := &Storage{db}
	return storage, err
}

// Clears certain data from Storage older than the given timestamp
// This includes Round and MixedMessage information
func (s *Storage) ClearOldStorage(ts time.Time) error {
	err := s.deleteRound(ts)
	if err != nil {
		return err
	}

	return s.deleteMixedMessages(ts)
}

// Builds a ClientBloomFilter with the given parameters, then stores it
func (s *Storage) HandleBloomFilter(recipientId ephemeral.Id, filterBytes []byte, roundId id.Round, epoch uint32) error {
	// Ignore zero-value recipient ID for now - this is a reserved address
	recipientIdInt := recipientId.Int64()
	if recipientIdInt == 0 {
		return nil
	}

	// Build a newly-initialized ClientBloomFilter to be stored
	validFilter := &ClientBloomFilter{
		RecipientId: &recipientIdInt,
		Epoch:       epoch,
		// FirstRound is input as CurrentRound for later calculation
		FirstRound: uint64(roundId),
		// RoundRange is empty for now as it can't be calculated yet
		RoundRange: 0,
		Filter:     filterBytes,
	}

	// Commit the new/updated ClientBloomFilter
	return s.upsertClientBloomFilter(validFilter)
}

// Returns a slice of MixedMessage from database with matching recipientId and roundId
// Also returns a boolean for whether the gateway contains other messages for the given Round
func (s *Storage) GetMixedMessages(recipientId ephemeral.Id, roundId id.Round) (msgs []*MixedMessage, hasRound bool, err error) {
	// Determine whether this gateway has any messages for the given roundId
	count, hasRound, err := s.countMixedMessagesByRound(roundId)
	if !hasRound || count == 0 {
		return
	}

	// If the gateway has messages, return messages relevant to the given recipientId and roundId
	msgs, err = s.getMixedMessages(recipientId, roundId)
	return
}

// Helper function for HandleBloomFilter
// Returns the bitwise OR of two byte slices
func or(existingBuffer, additionalBuffer []byte) []byte {
	if existingBuffer == nil {
		return additionalBuffer
	} else if additionalBuffer == nil {
		return existingBuffer
	} else if len(existingBuffer) != len(additionalBuffer) {
		jww.ERROR.Printf("Unable to perform bitwise OR: Slice lens invalid.")
		return existingBuffer
	}

	result := make([]byte, len(existingBuffer))
	for i := range existingBuffer {
		result[i] = existingBuffer[i] | additionalBuffer[i]
	}
	return result
}

// Combine with and update this filter using oldFilter
// Used in upsertFilter functionality in order to ensure atomicity
// Kept in business logic layer because functionality is shared
func (f *ClientBloomFilter) combine(oldFilter *ClientBloomFilter) {

	// Initialize FirstRound variable if needed
	if oldFilter.FirstRound == uint64(0) {
		oldFilter.FirstRound = f.FirstRound
	}

	// calculate what the first round should be
	firstRound := oldFilter.FirstRound
	if f.FirstRound < oldFilter.FirstRound {
		firstRound = f.FirstRound
	}

	// calculate what the last round should be
	lastRound := oldFilter.lastRound()
	if f.lastRound() > lastRound {
		lastRound = f.lastRound()
	}

	// set the first round
	// note this MUST be after last round is calculated
	// becasue the value in f is used in the last round calculation
	f.FirstRound = firstRound

	// calculate the round range based upon the first and last round
	f.RoundRange = uint32(lastRound - firstRound)

	// Combine the filters
	f.Filter = or(oldFilter.Filter, f.Filter)
	f.Uses = oldFilter.Uses + 1
}

func (f *ClientBloomFilter) lastRound() uint64 {
	return f.FirstRound + uint64(f.RoundRange)
}
