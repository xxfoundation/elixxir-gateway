////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles the high level storage API.
// This layer merges the business logic layer and the database layer

package storage

import (
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
)

// API for the storage layer
type Storage struct {
	// Stored database interface
	database
}

// Create a new Storage object wrapping a database interface
// Returns a Storage object, close function, and error
func NewStorage(username, password, dbName, address, port string) (*Storage, func() error, error) {
	db, closeFunc, err := newDatabase(username, password, dbName, address, port)
	storage := &Storage{db}
	return storage, closeFunc, err
}

// Determines whether a new BloomFilter should be created or an existing BloomFilter
// should be updated based on the given parameters, then performs the Upsert operation
func (s *Storage) HandleBloomFilter(recipientId *ephemeral.Id, filterBytes []byte, roundId id.Round, epoch uint64) error {

	// Get BloomFilter for only the given epoch
	filters, err := s.GetBloomFilters(recipientId, epoch, epoch)
	if err != nil {
		return err
	}

	var validFilter *BloomFilter
	if len(filters) == 0 {
		// If a BloomFilter doesn't exist for the given epoch, create a new one
		validFilter = &BloomFilter{
			RecipientId: recipientId.UInt64(),
			Epoch:       epoch,
			FirstRound:  uint64(roundId),
			LastRound:   uint64(roundId),
			Filter:      filterBytes,
		}
	} else {
		// If a BloomFilter exists for the given epoch, update it
		validFilter = filters[0]
		validFilter.LastRound = uint64(roundId)
		// TODO validFilter.Filter |= filterBytes
	}

	// Commit the new/updated BloomFilter
	return s.upsertBloomFilter(validFilter)
}

// Returns a slice of MixedMessage from database with matching recipientId and roundId
// Also returns a boolean for whether the gateway contains other messages for the given Round
func (s *Storage) GetMixedMessages(recipientId *id.ID, roundId id.Round) (msgs []*MixedMessage, isValidGateway bool, err error) {
	// Determine whether this gateway has any messages for the given roundId
	count, err := s.countMixedMessagesByRound(roundId)
	isValidGateway = count > 0
	if err != nil || !isValidGateway {
		return
	}

	// If the gateway has messages, return messages relevant to the given recipientId and roundId
	msgs, err = s.getMixedMessages(recipientId, roundId)
	return
}
