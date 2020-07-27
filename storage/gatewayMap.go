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
	"fmt"
	"github.com/pkg/errors"
	"gitlab.com/elixxir/primitives/id"
)

func (m *MapImpl) GetClient(id *id.ID) (*Client, error) {
	m.RLock()
	client := m.clients[*id]
	m.RUnlock()

	// Return an error if the Client was not found in the map
	if client == nil {
		return client, errors.Errorf("Could not find Client with ID %v in map.",
			id)
	}

	return client, nil
}

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

	fmt.Println("TEST")

	m.clients[*clientID] = client

	return nil
}

func (m *MapImpl) GetRound(id *id.Round) (*Round, error) {
	m.RLock()
	round := m.rounds[*id]
	m.RUnlock()

	// Return an error if the Round was not found in the map
	if round == nil {
		return round, errors.Errorf("Could not find Round with ID %v in map.",
			id)
	}

	return round, nil
}

func (m *MapImpl) InsertRound(round *Round) error {
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

func (m *MapImpl) InsertMixedMessage(msg *MixedMessage) error {
	m.Lock()
	defer m.Unlock()

	// Return an error if a MixedMessage with the ID already exists in the map
	if m.mixedMessages[msg.Id] != nil {
		return errors.Errorf("Could not insert MixedMessage. MixedMessage "+
			"with ID %v already exists in map.", msg.Id)
	}

	m.mixedMessages[msg.Id] = msg

	return nil
}

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

func (m *MapImpl) InsertBloomFilter(filter *BloomFilter) error {
	m.Lock()
	defer m.Unlock()

	// Return an error if a BloomFilter with the ID already exists in the map
	if m.bloomFilters[filter.Id] != nil {
		return errors.Errorf("Could not insert BloomFilter. BloomFilter with "+
			"ID %v already exists in map.", filter.Id)
	}

	m.bloomFilters[filter.Id] = filter

	return nil
}

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

func (m *MapImpl) InsertEphemeralBloomFilter(filter *EphemeralBloomFilter) error {
	m.Lock()
	defer m.Unlock()

	// Return an error if a BloomFilter with the ID already exists in the map
	if m.ephemeralBloomFilters[filter.Id] != nil {
		return errors.Errorf("Could not insert EphemeralBloomFilter."+
			"EphemeralBloomFilter with ID %v already exists in map.", filter.Id)
	}

	m.ephemeralBloomFilters[filter.Id] = filter

	return nil
}

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
