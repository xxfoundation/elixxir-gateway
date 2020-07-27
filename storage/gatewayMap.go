////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles the Map backend for gateway storage

package storage

import "gitlab.com/elixxir/primitives/id"

func (m *MapImpl) GetClient(id *id.ID) (*Client, error) {
	panic("implement me")
}

func (m *MapImpl) InsertClient(client *Client) error {
	panic("implement me")
}

func (m *MapImpl) GetRound(id *id.Round) (*Round, error) {
	panic("implement me")
}

func (m *MapImpl) InsertRound(round *Round) error {
	panic("implement me")
}

func (m *MapImpl) GetMixedMessages(recipientId *id.ID, roundId *id.Round) ([]*MixedMessage, error) {
	panic("implement me")
}

func (m *MapImpl) InsertMixedMessage(msg *MixedMessage) error {
	panic("implement me")
}

func (m *MapImpl) DeleteMixedMessage(id uint64) error {
	panic("implement me")
}

func (m *MapImpl) GetBloomFilters(clientId *id.ID) ([]*BloomFilter, error) {
	panic("implement me")
}

func (m *MapImpl) InsertBloomFilter(filter *BloomFilter) error {
	panic("implement me")
}

func (m *MapImpl) DeleteBloomFilter(id uint64) error {
	panic("implement me")
}

func (m *MapImpl) GetEphemeralBloomFilters(recipientId *id.ID) ([]*EphemeralBloomFilter, error) {
	panic("implement me")
}

func (m *MapImpl) InsertEphemeralBloomFilter(filter *EphemeralBloomFilter) error {
	panic("implement me")
}

func (m *MapImpl) DeleteEphemeralBloomFilter(id uint64) error {
	panic("implement me")
}
