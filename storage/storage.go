////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles the high level storage API.
// This layer merges the business logic layer and the database layer

package storage

import "time"

// API for the storage layer
type Storage struct {
	// Stored database interface
	database
}

//
type BloomFilterReturned struct {
	RecipientId  []byte
	Filter       []byte
	FirstRound   uint64
	LastRound    uint64
	DateModified time.Time
}

// Create a new Storage object wrapping a database interface
// Returns a Storage object, close function, and error
func NewStorage(username, password, dbName, address, port string) (*Storage, func() error, error) {
	db, closeFunc, err := newDatabase(username, password, dbName, address, port)
	storage := &Storage{db}
	return storage, closeFunc, err
}

//
func (s *Storage) IncrementEpoch(firstRound uint64) (uint64, error) {

}

//
func (s *Storage) DeleteBloomsByEpoch(epochId uint64) error {
	err := s.deleteEphemeralBloomFilterByEpoch(epochId)
	if err != nil {
		return err
	}

	return s.deleteBloomFilterByEpoch(epochId)
}
