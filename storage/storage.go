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
)

// API for the storage layer
type Storage struct {
	// Stored database interface
	database
}

// Return-type object for non-database representation of a BloomFilter
type ClientBloomFilter struct {
	Filter     []byte
	FirstRound id.Round
	LastRound  id.Round
}

// Create a new Storage object wrapping a database interface
// Returns a Storage object, close function, and error
func NewStorage(username, password, dbName, address, port string) (*Storage, func() error, error) {
	db, closeFunc, err := newDatabase(username, password, dbName, address, port)
	storage := &Storage{db}
	return storage, closeFunc, err
}

// Returns a slice of MixedMessage from database with matching recipientId and roundId
// Also returns a boolean for whether the gateway contains other messages for the given Round
func (s *Storage) GetMixedMessages(recipientId *id.ID, roundId id.Round) (msgs []*MixedMessage, isValidGateway bool, err error) {
	count, err := s.countMixedMessagesByRound(roundId)
	if err != nil {
		return
	}
	isValidGateway = count > 0
	msgs, err = s.getMixedMessages(recipientId, roundId)
	return
}

// Returns all of the ClientBloomFilter from Storage relevant to the given clientId
// latestRound is the most recent round in the network, used to populate fields of ClientBloomFilter
func (s *Storage) GetBloomFilters(clientId *id.ID, latestRound id.Round) ([]*ClientBloomFilter, error) {
	bloomFilters, err := s.getBloomFilters(clientId)
	if err != nil {
		return nil, err
	}

	result := make([]*ClientBloomFilter, 0)

	latestEpoch, err := s.GetLatestEpoch()
	if err != nil {
		return nil, err
	}

	for _, filter := range bloomFilters {
		clientFilter := &ClientBloomFilter{
			Filter: filter.Filter,
		}

		// Determine relevant rounds for the ClientBloomFilter
		if filter.EpochId == latestEpoch.Id {
			clientFilter.FirstRound = id.Round(latestEpoch.RoundId)
			clientFilter.LastRound = latestRound
		} else {

			epoch, err := s.GetEpoch(filter.EpochId)
			if err != nil {
				return nil, err
			}
			nextEpoch, err := s.GetEpoch(filter.EpochId + 1)
			if err != nil {
				return nil, err
			}

			clientFilter.FirstRound = id.Round(epoch.RoundId)
			clientFilter.LastRound = id.Round(nextEpoch.RoundId - 1)
		}

		result = append(result, clientFilter)
	}

	return result, nil
}
