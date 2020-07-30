///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// Handles the Map backend for gateway storage

package storage

import (
	"bytes"
	"github.com/pkg/errors"
	"gitlab.com/elixxir/primitives/id"
)

// Returns a Client from Storage with the given id
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

// Inserts the given Client into Storage
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

// Returns a Round from Storage with the given id
// Or an error if a matching Round does not exist
func (m *MapImpl) GetRound(id *id.Round) (*Round, error) {
	m.RLock()
	round := m.rounds[*id]
	m.RUnlock()

	// Return an error if the Round was not found in the map
	if round == nil {
		return nil, errors.Errorf("Could not find Round with ID %v in map.", id)
	}

	return round, nil
}

// Inserts the given Round into Storage if it does not exist
// Or updates the given Round if the provided Round UpdateId is greater
func (m *MapImpl) UpsertRound(round *Round) error {
	roundID := id.Round(round.Id)

	m.Lock()
	defer m.Unlock()

	// Return an error if a Round with the ID already exists in the map
	if m.rounds[roundID] != nil {
		return errors.Errorf("Could not insert Round. Round with ID %v "+
			"already exists in map.", roundID)
	}

	m.rounds[roundID] = round

	return nil
}

// Returns a slice of MixedMessages from Storage
// with matching recipientId and roundId
// Or an error if a matching Round does not exist
func (m *MapImpl) GetMixedMessages(recipientId *id.ID, roundId *id.Round) ([]*MixedMessage, error) {
	roundID := uint64(*roundId)
	var mixedMessages []*MixedMessage

	m.RLock()
	// Search map for all MixedMessages with matching recipient ID and round ID
	for _, msg := range m.mixedMessages {
		if bytes.Equal(msg.RecipientId, recipientId.Marshal()) && msg.RoundId == roundID {
			mixedMessages = append(mixedMessages, msg)
		}
	}
	m.RUnlock()

	// Return an error if no MixedMessages were found.
	if len(mixedMessages) == 0 {
		return nil, errors.Errorf("Could not find any MixedMessages with the "+
			"recipient ID %v and the round ID %v in map.", recipientId, roundId)
	}

	return mixedMessages, nil
}

// Inserts the given MixedMessage into Storage
// NOTE: Do not specify Id attribute, it is autogenerated
func (m *MapImpl) InsertMixedMessage(msg *MixedMessage) error {
	m.Lock()
	defer m.Unlock()

	// Return an error if a MixedMessage with the ID already exists in the map
	if m.mixedMessages[m.mixedMessagesCount] != nil {
		return errors.Errorf("Could not insert MixedMessage. MixedMessage "+
			"with ID %v already exists in map.", m.mixedMessagesCount)
	}

	m.mixedMessages[m.mixedMessagesCount] = msg

	m.mixedMessagesCount++

	return nil
}

// Deletes a MixedMessage with the given id from Storage
// Returns an error if a matching MixedMessage does not exist
func (m *MapImpl) DeleteMixedMessage(id uint64) error {
	m.Lock()
	defer m.Unlock()

	// Return an error if a MixedMessage with the ID does not exists in the map
	if m.mixedMessages[id] == nil {
		return errors.Errorf("Could not delete MixedMessage. MixedMessage "+
			"with ID %v does not exists in map.", id)
	}

	delete(m.mixedMessages, id)

	return nil
}

// Returns a BloomFilter from Storage with the given clientId
// Or an error if a matching BloomFilter does not exist
func (m *MapImpl) GetBloomFilters(clientId *id.ID) ([]*BloomFilter, error) {
	var bloomFilters []*BloomFilter

	m.RLock()
	// Search map for all BloomFilters with matching client ID
	for _, bf := range m.bloomFilters {
		if bytes.Equal(bf.ClientId, clientId.Marshal()) {
			bloomFilters = append(bloomFilters, bf)
		}
	}
	m.RUnlock()

	// Return an error if no BloomFilters were found.
	if len(bloomFilters) == 0 {
		return nil, errors.Errorf("Could not find any BloomFilters with the "+
			"client ID %v in map.", clientId)
	}

	return bloomFilters, nil
}

// Inserts the given BloomFilter into Storage
// NOTE: Do not specify Id attribute, it is autogenerated
func (m *MapImpl) InsertBloomFilter(filter *BloomFilter) error {
	m.Lock()
	defer m.Unlock()

	// Return an error if a BloomFilter with the ID already exists in the map
	if m.bloomFilters[m.bloomFiltersCount] != nil {
		return errors.Errorf("Could not insert BloomFilter. BloomFilter with "+
			"ID %v already exists in map.", m.bloomFiltersCount)
	}

	m.bloomFilters[m.bloomFiltersCount] = filter

	m.bloomFiltersCount++

	return nil
}

// Deletes a BloomFilter with the given id from Storage
// Returns an error if a matching BloomFilter does not exist
func (m *MapImpl) DeleteBloomFilter(id uint64) error {
	m.Lock()
	defer m.Unlock()

	// Return an error if a BloomFilter with the ID does not exists in the map
	if m.bloomFilters[id] == nil {
		return errors.Errorf("Could not delete BloomFilter. BloomFilter with "+
			"ID %v does not exists in map.", id)
	}

	delete(m.bloomFilters, id)

	return nil
}

// Returns a EphemeralBloomFilter from Storage with the given recipientId
// Or an error if a matching EphemeralBloomFilter does not exist
func (m *MapImpl) GetEphemeralBloomFilters(recipientId *id.ID) ([]*EphemeralBloomFilter, error) {
	var ephemeralBloomFilter []*EphemeralBloomFilter

	m.RLock()
	// Search map for all EphemeralBloomFilters with matching recipient ID
	for _, ebf := range m.ephemeralBloomFilters {
		if bytes.Equal(ebf.RecipientId, recipientId.Marshal()) {
			ephemeralBloomFilter = append(ephemeralBloomFilter, ebf)
		}
	}
	m.RUnlock()

	// Return an error if no EphemeralBloomFilters were found.
	if len(ephemeralBloomFilter) == 0 {
		return nil, errors.Errorf("Could not find any EphemeralBloomFilters "+
			"with the recipient ID %v in map.", recipientId)
	}

	return ephemeralBloomFilter, nil
}

// Inserts the given EphemeralBloomFilter into Storage
// NOTE: Do not specify Id attribute, it is autogenerated
func (m *MapImpl) InsertEphemeralBloomFilter(filter *EphemeralBloomFilter) error {
	m.Lock()
	defer m.Unlock()

	// Return an error if a BloomFilter with the ID already exists in the map
	if m.ephemeralBloomFilters[m.ephemeralBloomFiltersCount] != nil {
		return errors.Errorf("Could not insert EphemeralBloomFilter."+
			"EphemeralBloomFilter with ID %v already exists in map.",
			m.ephemeralBloomFiltersCount)
	}

	m.ephemeralBloomFilters[m.ephemeralBloomFiltersCount] = filter

	m.ephemeralBloomFiltersCount++

	return nil
}

// Deletes a EphemeralBloomFilter with the given id from Storage
// Returns an error if a matching EphemeralBloomFilter does not exist
func (m *MapImpl) DeleteEphemeralBloomFilter(id uint64) error {
	m.Lock()
	defer m.Unlock()

	// Return an error if a BloomFilter with the ID does not exists in the map
	if m.ephemeralBloomFilters[id] == nil {
		return errors.Errorf("Could not delete EphemeralBloomFilter. "+
			"EphemeralBloomFilter with ID %v does not exists in map.", id)
	}

	delete(m.ephemeralBloomFilters, id)

	return nil
}
