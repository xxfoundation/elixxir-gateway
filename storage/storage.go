////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles the high level storage API.
// This layer merges the business logic layer and the database layer

package storage

import "gitlab.com/xx_network/primitives/id"

// API for the storage layer
type Storage struct {
	// Stored database interface
	db database
}

//
func NewStorage(username, password, dbName, address, port string) (*Storage, func() error, error) {
	db, closeFunc, err := NewDatabase(username, password, dbName, address, port)
	storage := &Storage{db: db}
	return storage, closeFunc, err
}

//
func (s *Storage) InsertMixedMessage(msg *MixedMessage) error {
	return s.db.InsertMixedMessage(msg)
}

//
func (s *Storage) GetClient(id *id.ID) (*Client, error) {
	return s.db.GetClient(id)
}

//
func (s *Storage) InsertClient(client *Client) error {
	return s.db.InsertClient(client)
}

//
func (s *Storage) GetRounds(ids []id.Round) ([]*Round, error) {
	return s.db.GetRounds(ids)
}

//
func (s *Storage) GetMixedMessages(recipientId *id.ID, roundId id.Round) ([]*MixedMessage, error) {
	return s.GetMixedMessages(recipientId, roundId)
}
