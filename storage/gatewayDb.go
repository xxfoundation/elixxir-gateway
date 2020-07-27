////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles the database ORM for gateways

package storage

import "gitlab.com/elixxir/primitives/id"

func (d *DatabaseImpl) GetClient(id *id.ID) (*Client, error) {
	result := &Client{}
	err := d.db.First(&result, "id = ?", id.Marshal()).Error
	return result, err
}

func (d *DatabaseImpl) InsertClient(client *Client) error {
	return d.db.Create(client).Error
}

func (d *DatabaseImpl) GetRound(id *id.Round) (*Round, error) {
	result := &Round{}
	err := d.db.First(&result, "id = ?", uint64(*id)).Error
	return result, err
}

func (d *DatabaseImpl) InsertRound(round *Round) error {
	return d.db.Create(round).Error
}

func (d *DatabaseImpl) GetMixedMessages(recipientId *id.ID, roundId *id.Round) ([]*MixedMessage, error) {
	results := make([]*MixedMessage, 0)
	err := d.db.Find(&results,
		&MixedMessage{RecipientId: recipientId.Marshal(),
			RoundId: uint64(*roundId)}).Error
	return results, err
}

func (d *DatabaseImpl) InsertMixedMessage(msg *MixedMessage) error {
	return d.db.Create(msg).Error
}

func (d *DatabaseImpl) DeleteMixedMessage(id uint64) error {
	return d.db.Delete(&MixedMessage{
		Id: id,
	}).Error
}

func (d *DatabaseImpl) GetBloomFilters(clientId *id.ID) ([]*BloomFilter, error) {
	results := make([]*BloomFilter, 0)
	err := d.db.Find(&results,
		&BloomFilter{ClientId: clientId.Marshal()}).Error
	return results, err
}

func (d *DatabaseImpl) InsertBloomFilter(filter *BloomFilter) error {
	return d.db.Create(filter).Error
}

func (d *DatabaseImpl) DeleteBloomFilter(id uint64) error {
	return d.db.Delete(&BloomFilter{
		Id: id,
	}).Error
}

func (d *DatabaseImpl) GetEphemeralBloomFilters(recipientId *id.ID) ([]*EphemeralBloomFilter, error) {
	results := make([]*EphemeralBloomFilter, 0)
	err := d.db.Find(&results,
		&EphemeralBloomFilter{RecipientId: recipientId.Marshal()}).Error
	return results, err
}

func (d *DatabaseImpl) InsertEphemeralBloomFilter(filter *EphemeralBloomFilter) error {
	return d.db.Create(filter).Error
}

func (d *DatabaseImpl) DeleteEphemeralBloomFilter(id uint64) error {
	return d.db.Delete(&EphemeralBloomFilter{
		Id: id,
	}).Error
}
