////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Handles the database ORM for gateways

package storage

import (
	"bytes"
	"context"
	"database/sql"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gorm.io/gorm"
	"strings"
	"time"
)

// Helper for forcing panics in the event of a CDE, otherwise acts as a pass-through
func catchErrors(err error) error {
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			jww.FATAL.Panicf("Database call timed out: %+v", err.Error())
		}
		if strings.Contains(err.Error(), "No space left on device") {
			jww.FATAL.Panicf("Storage device full: %+v", err.Error())
		}
	}
	return err
}

// newContext builds a context for database operations.
func newContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), dbTimeout)
}

// Inserts the given State into Database if it does not exist
// Or updates the Database State if its value does not match the given State
func (d *DatabaseImpl) UpsertState(state *State) error {
	ctx, cancel := newContext()
	defer cancel()

	// Build a transaction to prevent race conditions
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
	return catchErrors(err)
}

// Returns a State's value from Database with the given key
// Or an error if a matching State does not exist
func (d *DatabaseImpl) GetStateValue(key string) (string, error) {
	ctx, cancel := newContext()
	defer cancel()

	result := &State{Key: key}
	err := d.db.WithContext(ctx).Take(result).Error
	return result.Value, catchErrors(err)
}

// Returns a Client from database with the given id
// Or an error if a matching Client does not exist
func (d *DatabaseImpl) GetClient(id *id.ID) (*Client, error) {
	ctx, cancel := newContext()
	defer cancel()

	result := &Client{}
	err := d.db.WithContext(ctx).Take(&result, "id = ?", id.Marshal()).Error
	return result, catchErrors(err)
}

// Upsert client into the database - replace key field if it differs so interrupted reg doesn't fail
func (d *DatabaseImpl) UpsertClient(client *Client) error {
	ctx, cancel := newContext()
	defer cancel()

	// Make a copy of the provided client
	newClient := *client

	// Build a transaction to prevent race conditions
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
	return catchErrors(err)
}

// Returns a Round from database with the given id
// Or an error if a matching Round does not exist
func (d *DatabaseImpl) GetRound(id id.Round) (*Round, error) {
	result := &Round{}
	err := d.db.Take(&result, "id = ?", uint64(id)).Error
	return result, catchErrors(err)
}

// GetRounds returns multiple Rounds from database with the given ids,
// or an error if no matching Rounds exist.
func (d *DatabaseImpl) GetRounds(ids []id.Round) ([]*Round, error) {
	// Convert IDs to plain numbers
	plainIds := make([]uint64, len(ids))
	for i, v := range ids {
		plainIds[i] = uint64(v)
	}

	// Execute the query
	results := make([]*Round, 0)
	err := d.db.Where("id IN (?)", plainIds).Find(&results).Error

	return results, catchErrors(err)
}

// UpsertRound into database if it does not exist,
// or updates the given Round if the provided Round UpdateId is greater.
func (d *DatabaseImpl) UpsertRound(round *Round) error {
	// Build a transaction to prevent race conditions
	err := d.db.Transaction(func(tx *gorm.DB) error {
		oldRound := &Round{
			LastUpdated: time.Now(),
		}

		// Attempt to insert the round into the database,
		// or if it already exists, replace round with the database value
		err := tx.Where(&Round{Id: round.Id}).FirstOrCreate(oldRound).Error
		if err != nil {
			return err
		}

		// If the provided round has a greater UpdateId than the database value,
		// overwrite the database value with the provided round
		if oldRound.UpdateId < round.UpdateId {
			round.LastUpdated = time.Now()
			return tx.Save(&round).Error
		}

		// Commit
		return nil
	})
	return catchErrors(err)
}

// Deletes all Round objects before the given timestamp from database
func (d *DatabaseImpl) deleteRound(ts time.Time) error {
	return catchErrors(d.db.Where("last_updated <= ?", ts).Delete(
		Round{}).Error)
}

// Count the number of MixedMessage in the database for the given roundId
func (d *DatabaseImpl) countMixedMessagesByRound(roundId id.Round) (uint64, bool, error) {
	ctx, cancel := newContext()
	defer cancel()

	var roundCount int64
	err := d.db.WithContext(ctx).Model(&ClientRound{}).Where(
		"id = ?", uint64(roundId)).Count(&roundCount).Error
	if err != nil {
		return 0, false, catchErrors(err)
	}
	hasRound := roundCount > 0

	var count int64
	if hasRound {
		err = d.db.WithContext(ctx).Model(&MixedMessage{}).Where(
			"round_id = ?", uint64(roundId)).Count(&count).Error
	}

	return uint64(count), hasRound, catchErrors(err)
}

// Returns a slice of MixedMessages from database
// with matching recipientId and roundId
// Or an error if a matching Round does not exist
func (d *DatabaseImpl) getMixedMessages(recipientId ephemeral.Id,
	roundId id.Round) ([]*MixedMessage, error) {
	ctx, cancel := newContext()
	defer cancel()

	results := make([]*MixedMessage, 0)
	err := d.db.WithContext(ctx).Find(&results,
		&MixedMessage{RecipientId: recipientId.Int64(),
			RoundId: uint64(roundId)}).Error
	return results, catchErrors(err)
}

// InsertMixedMessages into database.
// NOTE: Do not specify Id attribute for messages. It is autogenerated.
func (d *DatabaseImpl) InsertMixedMessages(cr *ClientRound) error {
	ctx, cancel := newContext()
	defer cancel()

	err := d.db.WithContext(ctx).Create(cr).Error
	return catchErrors(err)
}

// Deletes all MixedMessages before the given timestamp from database
func (d *DatabaseImpl) deleteMixedMessages(ts time.Time) error {
	return d.db.Where("timestamp <= ?", ts).Delete(ClientRound{}).Error
}

// GetClientBloomFilters returns ClientBloomFilter from database  with the
// given recipientId and an Epoch between startEpoch and endEpoch (inclusive).
// Returns an error if no matching ClientBloomFilter exist.
func (d *DatabaseImpl) GetClientBloomFilters(recipientId ephemeral.Id,
	startEpoch, endEpoch uint32) ([]*BloomFilter, error) {
	jww.DEBUG.Printf("Getting filters for client [%v]", recipientId)
	ctx, cancel := newContext()
	defer cancel()

	var results []*BloomFilter
	recipientIdInt := recipientId.Int64()
	err := d.db.WithContext(ctx).Where("epoch BETWEEN ? AND ?",
		startEpoch, endEpoch).Find(
		&results, &BloomFilter{RecipientId: &recipientIdInt}).Error

	jww.DEBUG.Printf("Returning filters [%v] for client [%v]",
		results, recipientId)
	return results, catchErrors(err)
}

// Inserts the given BloomFilter into database if it does not exist
// Or updates the BloomFilter in the database if the BloomFilter already exists
func (d *DatabaseImpl) upsertClientBloomFilter(filter *BloomFilter) error {
	jww.DEBUG.Printf("Upserting filter for client %d at epoch %d",
		*filter.RecipientId, filter.Epoch)
	ctx, cancel := newContext()
	defer cancel()

	// Build a transaction to prevent race conditions
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Initialize variable for returning existing value from the database
		oldFilter := &BloomFilter{
			Filter: make([]byte, len(filter.Filter)),
		}

		// Attempt to insert filter into the database,
		// or if it already exists, replace oldFilter with the database value
		err := tx.Where(&BloomFilter{
			Epoch:       filter.Epoch,
			RecipientId: filter.RecipientId,
		}).Where("uses < ?", maxBloomUses).FirstOrCreate(oldFilter).Error
		if err != nil {
			return err
		}

		// Combine oldFilter with filter
		filter.combine(oldFilter)

		// Commit to the database
		err = tx.Save(filter).Error
		if err != nil {
			return err
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
	return catchErrors(err)
}

// DeleteClientFiltersBeforeEpoch with Epoch <= the given epoch
// Returns an error if no matching ClientBloomFilter exist.
func (d *DatabaseImpl) DeleteClientFiltersBeforeEpoch(epoch uint32) error {
	ctx, cancel := newContext()
	err := d.db.WithContext(ctx).Delete(
		BloomFilter{}, "epoch <= ?", epoch).Error
	cancel()
	if err != nil {
		return catchErrors(err)
	}
	// TODO: Remove this code after migration to new table complete
	ctx, cancel = newContext()
	defer cancel()
	return catchErrors(d.db.WithContext(ctx).Delete(
		ClientBloomFilter{}, "epoch <= ?", epoch).Error)
}
