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

// Returns all of the ClientBloomFilter from Storage relevant to the given clientId
// latestRound is the most recent round in the network, used to populate fields of ClientBloomFilter
func (s *Storage) GetBloomFilters(recipientId *id.ID, latestRound id.Round) ([]*ClientBloomFilter, error) {
	result := make([]*ClientBloomFilter, 0)

	// Get all BloomFilter from Storage for the given recipientId
	bloomFilters, err := s.getBloomFilters(recipientId)
	if err != nil {
		return nil, err
	}

	// Get the latest epoch for determining how current the filters are
	/*latestEpoch, err := s.GetLatestEpoch()
	if err != nil {
		return nil, err
	}*/
	//latestEpoch := 0

	for _, filter := range bloomFilters {
		clientFilter := &ClientBloomFilter{
			Filter: filter.Filter,
		}
		clientFilter.FirstRound = id.Round(0)
		clientFilter.LastRound = latestRound
		/*
		// Determine relevant rounds for the ClientBloomFilter
		if filter.EpochId == latestEpoch.Id {
			// If the BloomFilter is current, use the current round
			clientFilter.FirstRound = id.Round(latestEpoch.RoundId)
			clientFilter.LastRound = latestRound
		} else {
			// If the BloomFilter is not current, infer the LastRound from the next Epoch
			epoch, err := s.GetEpoch(filter.EpochId)
			if err != nil {
				return nil, err
			}
			nextEpoch, err := s.GetEpoch(filter.EpochId + 1)
			if err != nil {
				return nil, err
			}

			clientFilter.FirstRound = id.Round(epoch.RoundId)
			// (Epoch n).LastRound = (Epoch n + 1).FirstRound - 1
			clientFilter.LastRound = id.Round(nextEpoch.RoundId - 1)
		}*/

		result = append(result, clientFilter)
	}

	return result, nil
}
