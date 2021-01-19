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
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
)

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

// Inserts the given Client into database
// Returns an error if a Client with a matching Id already exists
func (m *MapImpl) InsertClient(client *Client) error {
	// Convert Client's ID to an ID object
	clientID, err := id.Unmarshal(client.Id)
	if err != nil {
		return err
	}

	m.Lock()
	defer m.Unlock()

	// Return an error if a Client with the ID already exists in the map
	if m.clients[*clientID] != nil {
		return errors.Errorf("Could not insert Client. Client with ID %v "+
			"already exists in map.", clientID)
	}

	m.clients[*clientID] = client

	return nil
}

// mapimpl call for upserting clients
func (m *MapImpl) UpsertClient(client *Client) error {
	cid, err := id.Unmarshal(client.Id)
	if err != nil {
		return err
	}
	if _, ok := m.clients[*cid]; ok {
		copy(m.clients[*cid].Filters, client.Filters)
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
		m.rounds[roundID] = round
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
func (m *MapImpl) getMixedMessages(recipientId *id.ID, roundId id.Round) ([]*MixedMessage, error) {
	m.mixedMessages.RLock()
	defer m.mixedMessages.RUnlock()

	msgCount := len(m.mixedMessages.RecipientId[*recipientId][roundId])

	// Return an error if no matching messages are in the map
	if msgCount == 0 {
		return nil, errors.Errorf("Could not find any MixedMessages with the "+
			"recipient ID %v and the round ID %v in map.", recipientId, roundId)
	}

	// Build list of matching messages
	msgs := make([]*MixedMessage, msgCount)
	var i int
	for _, msg := range m.mixedMessages.RecipientId[*recipientId][roundId] {
		msgs[i] = msg
		i++
	}

	return msgs, nil
}

// Inserts the given list of MixedMessage into database
// NOTE: Do not specify Id attribute, it is autogenerated
func (m *MapImpl) InsertMixedMessages(msgs []*MixedMessage) error {
	m.mixedMessages.Lock()
	defer m.mixedMessages.Unlock()

	for _, msg := range msgs {
		// Generate  map keys
		roundId := id.Round(msg.RoundId)
		recipientId, err := id.Unmarshal(msg.RecipientId)
		if err != nil {
			return err
		}

		// Initialize inner maps if they do not already exist
		if m.mixedMessages.RoundId[roundId] == nil {
			m.mixedMessages.RoundId[roundId] = map[id.ID]map[uint64]*MixedMessage{}
		}
		if m.mixedMessages.RoundId[roundId][*recipientId] == nil {
			m.mixedMessages.RoundId[roundId][*recipientId] = map[uint64]*MixedMessage{}
		}
		if m.mixedMessages.RecipientId[*recipientId] == nil {
			m.mixedMessages.RecipientId[*recipientId] = map[id.Round]map[uint64]*MixedMessage{}
		}
		if m.mixedMessages.RecipientId[*recipientId][roundId] == nil {
			m.mixedMessages.RecipientId[*recipientId][roundId] = map[uint64]*MixedMessage{}
		}

		// Return an error if the message already exists
		if m.mixedMessages.RoundId[roundId][*recipientId][m.mixedMessages.IdTrack] != nil {
			return errors.Errorf("Message with ID %d already exists in the map.", m.mixedMessages.IdTrack)
		}

		msg.Id = m.mixedMessages.IdTrack
		m.mixedMessages.IdTrack++

		// Insert into maps
		m.mixedMessages.RoundId[roundId][*recipientId][msg.Id] = msg
		m.mixedMessages.RecipientId[*recipientId][roundId][msg.Id] = msg

		// Update the count of the number of mixed messages in map
		m.mixedMessages.RoundIdCount[roundId]++
	}

	return nil
}

// Deletes all MixedMessages with the given roundId from database
func (m *MapImpl) DeleteMixedMessageByRound(roundId id.Round) error {
	m.mixedMessages.Lock()
	defer m.mixedMessages.Unlock()

	// Delete all messages from the RecipientId map
	for recipientId := range m.mixedMessages.RoundId[roundId] {

		delete(m.mixedMessages.RecipientId[recipientId], roundId)
	}

	// Update the count of the number of mixed messages in map
	delete(m.mixedMessages.RoundIdCount, roundId)

	// Delete all messages from the RoundId map
	delete(m.mixedMessages.RoundId, roundId)

	return nil
}

// Returns ClientBloomFilter from database with the given recipientId
// and an Epoch between startEpoch and endEpoch (inclusive)
// Or an error if no matching ClientBloomFilter exist
func (m *MapImpl) GetClientBloomFilters(recipientId *ephemeral.Id, startEpoch, endEpoch uint32) ([]*ClientBloomFilter, error) {
	// TODO: Function needs rewritten given new query
	//m.bloomFilters.RLock()
	//defer m.bloomFilters.RUnlock()
	//
	//members := ""
	//for member := range m.bloomFilters.RecipientId {
	//	members += member.String() + ", "
	//}
	//
	//jww.INFO.Printf("Dump everyone on get in filter map: %#v", members)
	//filterCount := len(m.bloomFilters.RecipientId[*recipientId])
	//jww.INFO.Printf("Dump filter count for %s: %d", recipientId, filterCount)
	//
	//// Return an error if no BloomFilters were found
	//if filterCount == 0 {
	//	return nil, errors.Errorf("Could not find any BloomFilters with the "+
	//		"client ID %v in map.", recipientId)
	//}
	//
	//// Copy all matching bloom filters into slice
	//bloomFilters := make([]*ClientBloomFilter, filterCount)
	//var i int
	//for _, filter := range m.bloomFilters.RecipientId[*recipientId] {
	//	bloomFilters[i] = filter
	//	i++
	//}
	//
	return nil, nil
}

// Inserts the given ClientBloomFilter into database if it does not exist
// Or updates the ClientBloomFilter in the database if the ClientBloomFilter already exists
func (m *MapImpl) upsertClientBloomFilter(filter *ClientBloomFilter) error {
	// TODO: Function needs rewritten
	//jww.DEBUG.Printf("Upserting filter for client [%v]: %v", filter.RecipientId, filter)
	//
	//// Generate key for  RecipientId map
	//recipientId, err := id.Unmarshal(filter.RecipientId)
	//if err != nil {
	//	return err
	//}
	//
	//m.bloomFilters.Lock()
	//defer m.bloomFilters.Unlock()
	//
	//// Initialize inner slices if they do not already exist
	//if m.bloomFilters.RecipientId[*recipientId] == nil {
	//	m.bloomFilters.RecipientId[*recipientId] = make([]*ClientBloomFilter, 0)
	//}
	//
	//// Insert into maps
	//m.bloomFilters.RecipientId[*recipientId] = append(m.bloomFilters.RecipientId[*recipientId], filter)
	//
	//members := ""
	//for member := range m.bloomFilters.RecipientId {
	//	members += member.String() + ", "
	//}
	//
	//jww.INFO.Printf("Dump everyone on upsert in filter map: %#v", members)

	return nil
}

// Deletes all ClientBloomFilter with Epoch <= the given epoch
// Returns an error if no matching ClientBloomFilter exist
func (m *MapImpl) DeleteClientFiltersBeforeEpoch(epoch uint32) error {
	// TODO Write and test
	return nil
}
