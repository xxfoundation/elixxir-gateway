///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// Handles the Map backend for gateway storage

package storage

import (
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"time"
)

const initialClientBloomFilterListSize = 100

// Inserts the given State into Database if it does not exist
// Or updates the Database State if its value does not match the given State
func (m *MapImpl) UpsertState(state *State) error {
	m.Lock()
	defer m.Unlock()

	m.states[state.Key] = state.Value
	return nil
}

// Returns a State's value from Database with the given key
// Or an error if a matching State does not exist
func (m *MapImpl) GetStateValue(key string) (string, error) {
	m.Lock()
	defer m.Unlock()

	if val, ok := m.states[key]; ok {
		return val, nil
	} else {
		return "", errors.Errorf("Unable to locate state for key %s", key)
	}
}

// Returns a Client from database with the given id
// Or an error if a matching Client does not exist
func (m *MapImpl) GetClient(id *id.ID) (*Client, error) {
	m.RLock()
	client := m.clients[*id]
	m.RUnlock()

	// Return an error if the Client was not found in the map
	if client == nil {
		return nil, errors.Errorf("Could not find Client with ID %v in map.",
			id)
	}

	return client, nil
}

// Upsert client into the database - replace key field if it differs so interrupted reg doesn't fail
func (m *MapImpl) UpsertClient(client *Client) error {
	cid, err := id.Unmarshal(client.Id)
	if err != nil {
		return err
	}
	if _, ok := m.clients[*cid]; ok {
		copy(m.clients[*cid].Key, client.Key)
	} else {
		m.clients[*cid] = client
	}
	return nil
}

// Returns a Round from database with the given id
// Or an error if a matching Round does not exist
func (m *MapImpl) GetRound(id id.Round) (*Round, error) {
	m.RLock()
	round := m.rounds[id]
	m.RUnlock()

	// Return an error if the Round was not found in the map
	if round == nil {
		return nil, errors.Errorf("Could not find Round with ID %v in map.", id)
	}

	return round, nil
}

// Returns multiple Rounds from database with the given ids
// Or an error if no matching Rounds exist
func (m *MapImpl) GetRounds(ids []id.Round) ([]*Round, error) {
	m.RLock()
	defer m.RUnlock()

	results := make([]*Round, 0)
	for _, roundId := range ids {
		if round := m.rounds[roundId]; round != nil {
			results = append(results, round)
		}
	}

	if len(results) == 0 {
		return nil, errors.Errorf("Could not find matching Rounds in map.")
	}
	return results, nil
}

// Inserts the given Round into database if it does not exist
// Or updates the given Round if the provided Round UpdateId is greater
func (m *MapImpl) UpsertRound(round *Round) error {
	roundID := id.Round(round.Id)

	m.Lock()
	defer m.Unlock()

	// Insert the round if it does not exist or if it does exist, update it if
	// the update ID provided is greater
	if m.rounds[roundID] == nil || round.UpdateId > m.rounds[roundID].UpdateId {
		round.LastUpdated = time.Now()
		m.rounds[roundID] = round
	}

	return nil
}

// Deletes all Round objects before the given timestamp from database
func (m *MapImpl) deleteRound(ts time.Time) error {
	m.Lock()
	defer m.Unlock()
	for r := range m.rounds {
		if m.rounds[r].LastUpdated.Before(ts) {
			delete(m.rounds, r)
		}
	}
	return nil
}

// Count the number of MixedMessage in the database for the given roundId
func (m *MapImpl) countMixedMessagesByRound(roundId id.Round) (uint64, error) {
	m.mixedMessages.RLock()
	defer m.mixedMessages.RUnlock()

	return m.mixedMessages.RoundIdCount[roundId], nil
}

// Returns a slice of MixedMessages from database
// with matching recipientId and roundId
// Or an error if a matching Round does not exist
func (m *MapImpl) getMixedMessages(recipientId *ephemeral.Id, roundId id.Round) ([]*MixedMessage, error) {
	m.mixedMessages.RLock()
	defer m.mixedMessages.RUnlock()

	jww.INFO.Printf("Dumping RecipientMap: %+v", m.mixedMessages.RecipientId)
	msgCount := len(m.mixedMessages.RecipientId[recipientId.Int64()][roundId])

	// Return an error if no matching messages are in the map
	if msgCount == 0 {
		return nil, errors.Errorf("Could not find any MixedMessages with the "+
			"recipient ID %d and the round ID %v in map.", recipientId.Int64(), roundId)
	}

	// Build list of matching messages
	msgs := make([]*MixedMessage, msgCount)
	var i int
	for _, msg := range m.mixedMessages.RecipientId[recipientId.Int64()][roundId] {
		msgs[i] = msg
		i++
	}

	return msgs, nil
}

// Inserts the given list of MixedMessage into database
// NOTE: Do not specify Id attribute, it is autogenerated
func (m *MapImpl) InsertMixedMessages(cr *ClientRound) error {
	m.mixedMessages.Lock()
	msgs := cr.Messages

	for _, msg := range msgs {
		// Generate  map keys
		roundId := id.Round(msg.RoundId)

		// Initialize inner maps if they do not already exist
		if m.mixedMessages.RoundId[roundId] == nil {
			m.mixedMessages.RoundId[roundId] = make(map[int64]map[uint64]*MixedMessage)
		}
		if m.mixedMessages.RoundId[roundId][msg.RecipientId] == nil {
			m.mixedMessages.RoundId[roundId][msg.RecipientId] = make(map[uint64]*MixedMessage)
		}
		if m.mixedMessages.RecipientId[msg.RecipientId] == nil {
			m.mixedMessages.RecipientId[msg.RecipientId] = make(map[id.Round]map[uint64]*MixedMessage)
		}
		if m.mixedMessages.RecipientId[msg.RecipientId][roundId] == nil {
			m.mixedMessages.RecipientId[msg.RecipientId][roundId] = make(map[uint64]*MixedMessage)
		}

		// Return an error if the message already exists
		if m.mixedMessages.RoundId[roundId][msg.RecipientId][m.mixedMessages.IdTrack] != nil {
			return errors.Errorf("Message with ID %d already exists in the map.", m.mixedMessages.IdTrack)
		}

		msg.Id = m.mixedMessages.IdTrack
		m.mixedMessages.IdTrack++

		// Insert into maps
		m.mixedMessages.RoundId[roundId][msg.RecipientId][msg.Id] = &msg
		m.mixedMessages.RecipientId[msg.RecipientId][roundId][msg.Id] = &msg

		// Update the count of the number of mixed messages in map
		m.mixedMessages.RoundIdCount[roundId]++
	}
	jww.INFO.Printf("Dumping RecipientMap: %+v", m.mixedMessages.RecipientId)
	m.mixedMessages.Unlock()

	m.Lock()
	m.clientRounds[cr.Id] = cr
	m.Unlock()

	return nil
}

// Deletes all MixedMessages before the given timestamp from database
func (m *MapImpl) deleteMixedMessages(ts time.Time) error {
	m.mixedMessages.Lock()
	defer m.mixedMessages.Unlock()
	m.Lock()
	defer m.Unlock()
	for cr := range m.clientRounds {
		if m.clientRounds[cr].Timestamp.Before(ts) {
			for _, msg := range m.clientRounds[cr].Messages {
				roundId := id.Round(msg.RoundId)
				// Delete all messages from the RecipientId map
				for recipientId := range m.mixedMessages.RoundId[roundId] {
					delete(m.mixedMessages.RecipientId[recipientId], roundId)
				}

				// Update the count of the number of mixed messages in map
				delete(m.mixedMessages.RoundIdCount, roundId)

				// Delete all messages from the RoundId map
				delete(m.mixedMessages.RoundId, roundId)

				delete(m.clientRounds, cr)
			}
		}
	}

	return nil
}

// Returns ClientBloomFilter from database with the given recipientId
// and an Epoch between startEpoch and endEpoch (inclusive)
// Or an error if no matching ClientBloomFilter exist
func (m *MapImpl) GetClientBloomFilters(recipientId *ephemeral.Id, startEpoch, endEpoch uint32) ([]*ClientBloomFilter, error) {
	m.bloomFilters.RLock()
	defer m.bloomFilters.RUnlock()

	// Copy all matching bloom filters into slice
	list, exists := m.bloomFilters.RecipientId[recipientId.Int64()]

	// Return an error if the start or end epoch are out of range of the list or
	// if no epochs exist for the given ID.
	if !exists || startEpoch > list.lastEpoch() || endEpoch < list.start {
		return nil, errors.Errorf("Could not find any BloomFilters with the "+
			"client ID %v in map.", recipientId.Int64())
	}

	// Calculate the index for the startEpoch
	startIndex := list.getIndex(startEpoch)
	if startIndex < 0 {
		startIndex = 0
	}

	// Calculate the index for the endEpoch
	endIndex := list.getIndex(endEpoch)
	if endIndex > len(list.list) {
		endIndex = len(list.list) - 1
	}

	// Build list of existing filters between the range
	var bloomFilters []*ClientBloomFilter
	for _, bf := range list.list[startIndex : endIndex+1] {
		if bf != nil {
			bloomFilters = append(bloomFilters, bf)
		}
	}

	// Return an error if no BloomFilters were found
	if len(bloomFilters) == 0 {
		return nil, errors.Errorf("Could not find any ClientBloomFilter with "+
			"the client ID %v in map.", recipientId.Int64())
	}

	return bloomFilters, nil
}

// Inserts the given ClientBloomFilter into database if it does not exist
// Or updates the ClientBloomFilter in the database if the ClientBloomFilter already exists
func (m *MapImpl) upsertClientBloomFilter(filter *ClientBloomFilter) error {
	m.bloomFilters.Lock()
	defer m.bloomFilters.Unlock()
	jww.DEBUG.Printf("Upserting filter for client %v at epoch %d", filter.RecipientId, filter.Epoch)

	// Initialize list if it does not exist
	list := m.bloomFilters.RecipientId[filter.RecipientId]
	if list == nil {
		m.bloomFilters.RecipientId[filter.RecipientId] = &ClientBloomFilterList{
			list:  make([]*ClientBloomFilter, initialClientBloomFilterListSize),
			start: filter.Epoch,
		}
		list = m.bloomFilters.RecipientId[filter.RecipientId]
	}

	// Expand the list if it is not large enough
	index := list.getIndex(filter.Epoch)
	if index >= len(list.list) {
		list.changeSize(index+initialClientBloomFilterListSize, 0, 0)
	} else if index < 0 {
		list.changeSize(len(list.list)-index, int(list.start-filter.Epoch), 0)
		list.start = filter.Epoch
		index = list.getIndex(filter.Epoch)
	}

	// Update the filter with the new one if one already exists in the list
	if oldFilter := list.list[index]; oldFilter != nil {
		filter.combine(oldFilter)
	}

	// Insert the filter into the list
	list.list[index] = filter

	return nil
}

// Deletes all ClientBloomFilter with Epoch <= the given epoch
// Returns an error if no matching ClientBloomFilter exist
func (m *MapImpl) DeleteClientFiltersBeforeEpoch(epoch uint32) error {
	m.bloomFilters.Lock()
	defer m.bloomFilters.Unlock()

	bfCount := 0

	for rid, list := range m.bloomFilters.RecipientId {
		// If the epoch occurred before the first filter, skip the list
		if list.start > epoch {
			continue
		} else {
			bfCount++
		}

		// If the epoch occurred after the last filter, then delete the list
		if epoch > list.lastEpoch() {
			delete(m.bloomFilters.RecipientId, rid)
			continue
		}

		// Delete epochs that occurred before
		list.changeSize(len(list.list)-int(epoch-list.start), 0, list.getIndex(epoch+1))
		list.start = epoch + 1
	}

	if bfCount == 0 {
		return errors.Errorf("Could not find any bloom filters that occurred "+
			"before epoch %d.", epoch)
	}

	return nil
}

// changeSize expands or shrinks the list. copyIndex specifies the location
// where to place the data in the modified array. cutIndex specifies the start
// location of data to be copied.
func (bfl *ClientBloomFilterList) changeSize(size, copyIndex, cutIndex int) {
	newList := make([]*ClientBloomFilter, size)
	copy(newList[copyIndex:], bfl.list[cutIndex:])
	bfl.list = newList
}

// lastEpoch returns the epoch of the last item in the list regardless if it is
// nil or not.
func (bfl *ClientBloomFilterList) lastEpoch() uint32 {
	return bfl.start + uint32(len(bfl.list))
}

// getIndex returns the index in the array of the epoch.
func (bfl *ClientBloomFilterList) getIndex(epoch uint32) int {
	return int(epoch) - int(bfl.start)
}
