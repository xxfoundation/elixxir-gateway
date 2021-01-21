////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles the database ORM for gateways

package storage

import (
	"bytes"
	"github.com/jinzhu/gorm"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
)

// Inserts the given State into Database if it does not exist
// Or updates the Database State if its value does not match the given State
func (d *DatabaseImpl) UpsertState(state *State) error {
	// Build a transaction to prevent race conditions
	return d.db.Transaction(func(tx *gorm.DB) error {
		// Make a copy of the provided state
		newState := *state

		// Attempt to insert state into the Database,
		// or if it already exists, replace state with the Database value
		err := tx.FirstOrCreate(state, &State{Key: state.Key}).Error
		if err != nil {
			return err
		}

		// If state is already present in the Database, overwrite it with newState
		if newState.Value != state.Value {
			return tx.Save(newState).Error
		}

		// Commit
		return nil
	})
}

// Returns a State's value from Database with the given key
// Or an error if a matching State does not exist
func (d *DatabaseImpl) GetStateValue(key string) (string, error) {
	result := &State{Key: key}
	err := d.db.Take(result).Error
	return result.Value, err
}

// Returns a Client from database with the given id
// Or an error if a matching Client does not exist
func (d *DatabaseImpl) GetClient(id *id.ID) (*Client, error) {
	result := &Client{}
	err := d.db.Take(&result, "id = ?", id.Marshal()).Error
	return result, err
}

// Inserts the given Client into database
// Returns an error if a Client with a matching Id already exists
func (d *DatabaseImpl) InsertClient(client *Client) error {
	return d.db.Create(client).Error
}

// Upsert client into the database - replace key field if it differs so interrupted reg doesn't fail
func (d *DatabaseImpl) UpsertClient(client *Client) error {
	// Make a copy of the provided client
	newClient := *client

	// Build a transaction to prevent race conditions
	return d.db.Transaction(func(tx *gorm.DB) error {
		// Attempt to insert the client into the database,
		// or if it already exists, replace client with the database value
		err := tx.FirstOrCreate(client, &Client{Id: client.Id}).Error
		if err != nil {
			return err
		}

		// If the provided client has a different Key than the database value,
		// overwrite the database value with the provided client
		if !bytes.Equal(client.Key, newClient.Key) {
			return tx.Save(&newClient).Error
		}

		// Commit
		return nil
	})
}

// Returns a Round from database with the given id
// Or an error if a matching Round does not exist
func (d *DatabaseImpl) GetRound(id id.Round) (*Round, error) {
	result := &Round{}
	err := d.db.Take(&result, "id = ?", uint64(id)).Error
	return result, err
}

// Returns multiple Rounds from database with the given ids
// Or an error if no matching Rounds exist
func (d *DatabaseImpl) GetRounds(ids []id.Round) ([]*Round, error) {
	// Convert IDs to plain numbers
	plainIds := make([]uint64, len(ids))
	for i, v := range ids {
		plainIds[i] = uint64(v)
	}

	// Execute the query
	results := make([]*Round, 0)
	err := d.db.Where("id IN (?)", plainIds).Find(&results).Error

	return results, err
}

// Inserts the given Round into database if it does not exist
// Or updates the given Round if the provided Round UpdateId is greater
func (d *DatabaseImpl) UpsertRound(round *Round) error {
	// Make a copy of the provided round
	newRound := *round

	// Build a transaction to prevent race conditions
	return d.db.Transaction(func(tx *gorm.DB) error {

		// Attempt to insert the round into the database,
		// or if it already exists, replace round with the database value
		err := tx.FirstOrCreate(round, &Round{Id: round.Id}).Error
		if err != nil {
			return err
		}

		// If the provided round has a greater UpdateId than the database value,
		// overwrite the database value with the provided round
		if round.UpdateId < newRound.UpdateId {
			return tx.Save(&newRound).Error
		}

		// Commit
		return nil
	})
}

// Count the number of MixedMessage in the database for the given roundId
func (d *DatabaseImpl) countMixedMessagesByRound(roundId id.Round) (uint64, error) {
	var count uint64
	err := d.db.Model(&MixedMessage{}).Where("round_id = ?", uint64(roundId)).Count(&count).Error
	return count, err
}

// Returns a slice of MixedMessages from database
// with matching recipientId and roundId
// Or an error if a matching Round does not exist
func (d *DatabaseImpl) getMixedMessages(recipientId *ephemeral.Id, roundId id.Round) ([]*MixedMessage, error) {
	results := make([]*MixedMessage, 0)
	err := d.db.Find(&results,
		&MixedMessage{RecipientId: recipientId.Int64(),
			RoundId: uint64(roundId)}).Error
	return results, err
}

// Inserts the given list of MixedMessage into database
// NOTE: Do not specify Id attribute, it is autogenerated
func (d *DatabaseImpl) InsertMixedMessages(msgs []*MixedMessage) error {
	for _, msg := range msgs {
		err := d.db.Create(msg).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// Deletes all MixedMessages with the given roundId from database
func (d *DatabaseImpl) DeleteMixedMessageByRound(roundId id.Round) error {
	return d.db.Where("round_id = ?", uint64(roundId)).Delete(MixedMessage{}).Error
}

// Returns ClientBloomFilter from database with the given recipientId
// and an Epoch between startEpoch and endEpoch (inclusive)
// Or an error if no matching ClientBloomFilter exist
func (d *DatabaseImpl) GetClientBloomFilters(recipientId *ephemeral.Id, startEpoch, endEpoch uint32) ([]*ClientBloomFilter, error) {
	jww.DEBUG.Printf("Getting filters for client [%v]", recipientId)

	results := make([]*ClientBloomFilter, 0)
	err := d.db.Find(&results, &ClientBloomFilter{RecipientId: recipientId.Int64()}).
		Where("epoch BETWEEN ? AND ?", startEpoch, endEpoch).Error
	jww.DEBUG.Printf("Returning filters [%v] for client [%v]", results, recipientId)

	return results, err
}

// Inserts the given ClientBloomFilter into database if it does not exist
// Or updates the ClientBloomFilter in the database if the ClientBloomFilter already exists
func (d *DatabaseImpl) upsertClientBloomFilter(filter *ClientBloomFilter) error {
	jww.DEBUG.Printf("Upserting filter for client [%v]: %v", filter.RecipientId, filter)

	// Build a transaction to prevent race conditions
	return d.db.Transaction(func(tx *gorm.DB) error {
		// Initialize variable for returning existing value from the database
		oldFilter := &ClientBloomFilter{}

		// Attempt to insert filter into the database,
		// or if it already exists, replace oldFilter with the database value
		err := tx.FirstOrCreate(oldFilter, &ClientBloomFilter{
			Epoch:       filter.Epoch,
			RecipientId: filter.RecipientId,
		}).Error
		if err != nil {
			return err
		}

		// Combine oldFilter with filter
		filter.combine(oldFilter)

		// Commit to the database
		return tx.Save(filter).Error
	})
}

// Deletes all ClientBloomFilter with Epoch <= the given epoch
// Returns an error if no matching ClientBloomFilter exist
func (d *DatabaseImpl) DeleteClientFiltersBeforeEpoch(epoch uint32) error {
	return d.db.Delete(ClientBloomFilter{}, "epoch <= ?", epoch).Error
}
