////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles the database ORM for gateways

package storage

import "gitlab.com/elixxir/primitives/id"

func (d *DatabaseImpl) GetClient(id *id.ID) (*Client, error) {
	panic("implement me")
}

func (d *DatabaseImpl) InsertClient(client *Client) error {
	panic("implement me")
}

func (d *DatabaseImpl) GetRound(id *id.Round) (*Round, error) {
	panic("implement me")
}

func (d *DatabaseImpl) InsertRound(round *Round) error {
	panic("implement me")
}

func (d *DatabaseImpl) GetMixedMessages(recipientId *id.ID, roundId *id.Round) ([]*MixedMessage, error) {
	panic("implement me")
}

func (d *DatabaseImpl) InsertMixedMessage(msg *MixedMessage) error {
	panic("implement me")
}

func (d *DatabaseImpl) DeleteMixedMessage(id uint64) error {
	panic("implement me")
}

func (d *DatabaseImpl) GetBloomFilters(clientId *id.ID) ([]*BloomFilter, error) {
	panic("implement me")
}

func (d *DatabaseImpl) InsertBloomFilter(filter *BloomFilter) error {
	panic("implement me")
}

func (d *DatabaseImpl) DeleteMessage(id uint64) error {
	panic("implement me")
}

func (d *DatabaseImpl) GetEphemeralBloomFilters(clientId *id.ID) ([]*EphemeralBloomFilter, error) {
	panic("implement me")
}

func (d *DatabaseImpl) InsertEphemeralBloomFilter(filter *EphemeralBloomFilter) error {
	panic("implement me")
}

func (d *DatabaseImpl) DeleteEphemeralBloomFilter(id uint64) error {
	panic("implement me")
}
