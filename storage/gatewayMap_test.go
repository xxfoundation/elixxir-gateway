///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package storage

import (
	"gitlab.com/elixxir/primitives/id"
	"math/rand"
	"testing"
)

// Hidden function for one-time unit testing database implementation
//func TestDatabaseImpl(t *testing.T) {
//
//	jwalterweatherman.SetLogThreshold(jwalterweatherman.LevelTrace)
//	jwalterweatherman.SetStdoutThreshold(jwalterweatherman.LevelTrace)
//
//	db, _, err := NewDatabase("cmix", "", "cmix_gateway", "0.0.0.0", "5432")
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//
//	testBytes := []byte("test")
//	testClientId := []byte("client")
//	testRound := uint64(10)
//
//	testClient := id.NewIdFromBytes(testClientId, t)
//	testRecip := id.NewIdFromBytes(testBytes, t)
//	testRoundId := id.Round(testRound)
//
//
//	err = db.InsertClient(&Client{
//		Id:      testClient.Marshal(),
//		Key:     testBytes,
//	})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	err = db.UpsertRound(&Round{
//		Id:       testRound,
//		UpdateId: 71,
//		InfoBlob: testBytes,
//	})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	err = db.InsertBloomFilter(&BloomFilter{
//		ClientId:    testClient.Marshal(),
//		Count:       10,
//		Filter:      testBytes,
//		DateCreated: time.Now(),
//	})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	err = db.InsertBloomFilter(&BloomFilter{
//		ClientId:    testClient.Marshal(),
//		Count:       20,
//		Filter:      testBytes,
//		DateCreated: time.Now(),
//	})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	err = db.InsertEphemeralBloomFilter(&EphemeralBloomFilter{
//		RecipientId: testRecip.Marshal(),
//		Count:       20,
//		Filter:      testBytes,
//	})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	err = db.InsertEphemeralBloomFilter(&EphemeralBloomFilter{
//		RecipientId: testRecip.Marshal(),
//		Count:       40,
//		Filter:      testBytes,
//	})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	err = db.InsertMixedMessage(&MixedMessage{
//		RoundId:         testRound,
//		RecipientId:     testClient.Marshal(),
//		MessageContents: testBytes,
//	})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	err = db.InsertMixedMessage(&MixedMessage{
//		RoundId:         testRound,
//		RecipientId:     testClient.Marshal(),
//		MessageContents: []byte("Test24"),
//	})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//
//	client, err := db.GetClient(testClient)
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	jww.INFO.Printf("%+v", client)
//	round, err := db.GetRound(&testRoundId)
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	jww.INFO.Printf("%+v", round)
//	messages, err := db.GetMixedMessages(testClient, &testRoundId)
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	jww.INFO.Printf("%+v", messages)
//	filters, err := db.GetBloomFilters(testClient)
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	jww.INFO.Printf("%+v", filters)
//	ephFilters, err := db.GetEphemeralBloomFilters(testRecip)
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	jww.INFO.Printf("%+v", ephFilters)
//
//	err = db.DeleteMixedMessage(messages[0].Id)
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	err = db.DeleteBloomFilter(filters[0].Id)
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	err = db.DeleteEphemeralBloomFilter(ephFilters[0].Id)
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//}

// Happy path
func TestMapImpl_GetClient(t *testing.T) {
	testKey := *id.NewIdFromString("testKey1", id.User, t)
	testClient := &Client{Id: testKey.Marshal()}
	m := &MapImpl{
		clients: map[id.ID]*Client{testKey: testClient},
	}

	client, err := m.GetClient(&testKey)
	if err != nil || client != testClient {
		t.Errorf("Failed to get client: %v", err)
	}
}

// Error Path: Client not in map.
func TestMapImpl_GetClient_NoClientError(t *testing.T) {
	testKey := id.NewIdFromString("testKey1", id.User, t)
	m := &MapImpl{
		clients: map[id.ID]*Client{},
	}

	client, err := m.GetClient(testKey)
	if err == nil || client != nil {
		t.Errorf("No error returned when client does not exist.")
	}
}

// Happy path
func TestMapImpl_InsertClient(t *testing.T) {
	testKey := id.NewIdFromString("testKey1", id.User, t)
	testClient := &Client{Id: testKey.Marshal()}
	m := &MapImpl{
		clients: make(map[id.ID]*Client),
	}

	err := m.InsertClient(testClient)
	if err != nil || m.clients[*testKey] == nil {
		t.Errorf("Failed to insert client: %v", err)
	}
}

// Error Path: Client already exists in map.
func TestMapImpl_InsertClient_ClientAlreadyExistsError(t *testing.T) {
	testKey := *id.NewIdFromString("testKey1", id.User, t)
	testClient := &Client{Id: testKey.Marshal()}
	m := &MapImpl{
		clients: map[id.ID]*Client{testKey: testClient},
	}

	err := m.InsertClient(testClient)
	if err == nil {
		t.Errorf("Did not error when attempting to insert a client that " +
			"already exists.")
	}
}

// Error Path: Client has an invalid ID.
func TestMapImpl_InsertClient_InvalidIdError(t *testing.T) {
	testClient := &Client{Id: []byte{1, 2, 3}}
	m := &MapImpl{}

	err := m.InsertClient(testClient)
	if err == nil {
		t.Errorf("Did not error when provided client with invalid ID.")
	}
}

// Happy path.
func TestMapImpl_GetRound(t *testing.T) {
	testKey := id.Round(rand.Uint64())
	testRound := &Round{Id: uint64(testKey)}
	m := &MapImpl{
		rounds: map[id.Round]*Round{testKey: testRound},
	}

	round, err := m.GetRound(&testKey)
	if err != nil || round != testRound {
		t.Errorf("Failed to get round: %v", err)
	}
}

// Error Path: Round not in map.
func TestMapImpl_GetRound_NoRoundError(t *testing.T) {
	testKey := id.Round(rand.Uint64())
	m := &MapImpl{
		rounds: make(map[id.Round]*Round),
	}

	round, err := m.GetRound(&testKey)
	if err == nil || round != nil {
		t.Errorf("No error returned when round does not exist.")
	}
}

// Happy path.
func TestMapImpl_InsertRound(t *testing.T) {
	testKey := id.Round(rand.Uint64())
	testRound := &Round{Id: uint64(testKey)}
	m := &MapImpl{
		rounds: make(map[id.Round]*Round),
	}

	err := m.UpsertRound(testRound)
	if err != nil || m.rounds[testKey] == nil {
		t.Errorf("Failed to insert round: %v", err)
	}
}

// Error Path: Round already exists in map.
func TestMapImpl_InsertRound_RoundAlreadyExistsError(t *testing.T) {
	testKey := id.Round(rand.Uint64())
	testRound := &Round{Id: uint64(testKey)}
	m := &MapImpl{
		rounds: map[id.Round]*Round{testKey: testRound},
	}

	err := m.UpsertRound(testRound)
	if err == nil {
		t.Errorf("Did not error when attempting to insert a round that " +
			"already exists.")
	}
}

// Happy path.
func TestMapImpl_GetMixedMessages(t *testing.T) {
	testMsgID := rand.Uint64()
	testRoundID := id.Round(rand.Uint64())
	testRecipientID := id.NewIdFromUInt(rand.Uint64(), id.User, t)
	testMixedMessage := &MixedMessage{
		Id:          testMsgID,
		RoundId:     uint64(testRoundID),
		RecipientId: testRecipientID.Marshal(),
	}
	m := &MapImpl{
		mixedMessages: map[uint64]*MixedMessage{
			testMsgID: testMixedMessage,
		},
	}

	// Get list of 1 item
	mixedMsgs, err := m.GetMixedMessages(testRecipientID, &testRoundID)
	if err != nil {
		t.Errorf("Unexpected error retrieving mixedMessage: %v", err)
	}
	if len(mixedMsgs) != 1 {
		t.Errorf("Received unexpected number of MixedMessages: %v", mixedMsgs)
	}

	// Add more messages with same recipient and round IDs.
	testMsgID = rand.Uint64()
	testMixedMessage = &MixedMessage{
		Id:          testMsgID,
		RoundId:     uint64(testRoundID),
		RecipientId: testRecipientID.Marshal(),
	}
	m.mixedMessages[testMsgID] = testMixedMessage
	testMsgID = rand.Uint64()
	testMixedMessage = &MixedMessage{
		Id:          testMsgID,
		RoundId:     uint64(testRoundID),
		RecipientId: testRecipientID.Marshal(),
	}
	m.mixedMessages[testMsgID] = testMixedMessage

	// Get list of 3 items
	mixedMsgs, err = m.GetMixedMessages(testRecipientID, &testRoundID)
	if err != nil {
		t.Errorf("Unexpected error retrieving mixedMessage: %v", err)
	}
	if len(mixedMsgs) != 3 {
		t.Errorf("Received unexpected number of MixedMessages: %v", mixedMsgs)
	}

	// Add more messages with different recipient and round IDs.
	testMsgID = rand.Uint64()
	testMixedMessage = &MixedMessage{
		Id:          testMsgID,
		RoundId:     rand.Uint64(),
		RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(),
	}
	m.mixedMessages[testMsgID] = testMixedMessage
	testMsgID = rand.Uint64()
	testMixedMessage = &MixedMessage{
		Id:          testMsgID,
		RoundId:     rand.Uint64(),
		RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(),
	}
	m.mixedMessages[testMsgID] = testMixedMessage

	// Get list of 3 items
	mixedMsgs, err = m.GetMixedMessages(testRecipientID, &testRoundID)
	if err != nil {
		t.Errorf("Unexpected error retrieving mixedMessage: %v", err)
	}
	if len(mixedMsgs) != 3 {
		t.Errorf("Received unexpected number of MixedMessages: %v", mixedMsgs)
	}
}

// Error Path: No matching messages exist in the map.
func TestMapImpl_GetMixedMessages_NoMessageError(t *testing.T) {
	testRoundID := id.Round(rand.Uint64())
	testRecipientID := id.NewIdFromUInt(rand.Uint64(), id.User, t)
	m := &MapImpl{
		mixedMessages: map[uint64]*MixedMessage{
			rand.Uint64(): {
				RoundId:     rand.Uint64(),
				RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(),
			},
			rand.Uint64(): {
				RoundId:     rand.Uint64(),
				RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(),
			},
		},
	}

	// Attempt to get message that is not in map
	mixedMsgs, err := m.GetMixedMessages(testRecipientID, &testRoundID)
	if err == nil {
		t.Errorf("Expected an error when mixedMessage is not found in map.")
	}
	if mixedMsgs != nil {
		t.Errorf("Expected nil mixedMessages. Received: %v", mixedMsgs)
	}
}

// Happy path.
func TestMapImpl_InsertMixedMessage(t *testing.T) {
	testMsgID := uint64(0)
	testMixedMessage := &MixedMessage{
		RoundId:     rand.Uint64(),
		RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(),
	}
	m := &MapImpl{
		mixedMessages: make(map[uint64]*MixedMessage),
	}

	err := m.InsertMixedMessage(testMixedMessage)
	if err != nil || m.mixedMessages[testMsgID] == nil {
		t.Errorf("Failed to insert MixedMessage: %v", err)
	}
}

// Error Path: MixedMessage already exists in map.
func TestMapImpl_InsertMixedMessage_MessageAlreadyExistsError(t *testing.T) {
	testMsgID := uint64(0)
	testMixedMessage := &MixedMessage{
		RoundId:     rand.Uint64(),
		RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(),
	}
	m := &MapImpl{
		mixedMessages: map[uint64]*MixedMessage{testMsgID: testMixedMessage},
	}

	err := m.InsertMixedMessage(testMixedMessage)
	if err == nil {
		t.Errorf("Did not error when attempting to insert a mixedMessage that " +
			"already exists.")
	}
}

// Happy path.
func TestMapImpl_DeleteMixedMessage(t *testing.T) {
	testMsgID := rand.Uint64()
	testRoundID := id.Round(rand.Uint64())
	testRecipientID := id.NewIdFromUInt(rand.Uint64(), id.User, t)
	testMixedMessage := &MixedMessage{
		Id:          testMsgID,
		RoundId:     uint64(testRoundID),
		RecipientId: testRecipientID.Marshal(),
	}
	m := &MapImpl{
		mixedMessages: map[uint64]*MixedMessage{
			testMsgID: testMixedMessage,
		},
	}

	err := m.DeleteMixedMessage(testMsgID)

	if err != nil || m.mixedMessages[testMsgID] != nil {
		t.Errorf("Failed to delete MixedMessage: %v", err)
	}
}

// Error Path: MixedMessage does not exists in map.
func TestMapImpl_DeleteMixedMessage_NoMessageError(t *testing.T) {
	testMsgID := rand.Uint64()
	m := &MapImpl{
		mixedMessages: make(map[uint64]*MixedMessage),
	}

	err := m.DeleteMixedMessage(testMsgID)

	if err == nil {
		t.Errorf("No error received when attemting to delete message that " +
			"does not exist in map.")
	}
}

// Happy path.
func TestMapImpl_GetBloomFilters(t *testing.T) {
	testClientID := id.NewIdFromUInt(rand.Uint64(), id.User, t)
	m := &MapImpl{
		bloomFilters: map[uint64]*BloomFilter{
			rand.Uint64(): {ClientId: testClientID.Marshal()},
			rand.Uint64(): {ClientId: testClientID.Marshal()},
			rand.Uint64(): {ClientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal()},
			rand.Uint64(): {ClientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal()},
			rand.Uint64(): {ClientId: testClientID.Marshal()},
		},
	}

	bloomFilters, err := m.GetBloomFilters(testClientID)
	if err != nil {
		t.Errorf("Unexpected error retrieving bloom filters: %v", err)
	}
	if len(bloomFilters) != 3 {
		t.Errorf("Received unexpected number of bloom filters: %v", bloomFilters)
	}
}

// Error Path: No matching bloom filters exist in the map.
func TestMapImpl_GetBloomFilters_NoFiltersError(t *testing.T) {
	testClientID := id.NewIdFromUInt(rand.Uint64(), id.User, t)
	m := &MapImpl{
		bloomFilters: map[uint64]*BloomFilter{
			rand.Uint64(): {ClientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal()},
			rand.Uint64(): {ClientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal()},
		},
	}

	bloomFilters, err := m.GetBloomFilters(testClientID)
	if err == nil {
		t.Errorf("Expected an error when bloom filters is not in map.")
	}
	if bloomFilters != nil {
		t.Errorf("Expected nil bloom filters returned. Received: %v",
			bloomFilters)
	}
}

// Happy path.
func TestMapImpl_InsertBloomFilter(t *testing.T) {
	testID := uint64(0)
	testBloomFilter := &BloomFilter{Id: testID}
	m := &MapImpl{
		bloomFilters: make(map[uint64]*BloomFilter),
	}

	err := m.InsertBloomFilter(testBloomFilter)
	if err != nil || m.bloomFilters[testID] == nil {
		t.Errorf("Failed to insert bloom filter: %v", err)
	}
}

// Error Path: Bloom filter already exists in map.
func TestMapImpl_InsertBloomFilter_FilterAlreadyExistsError(t *testing.T) {
	testID := uint64(0)
	testBloomFilter := &BloomFilter{Id: testID}
	m := &MapImpl{
		bloomFilters: map[uint64]*BloomFilter{testID: testBloomFilter},
	}

	err := m.InsertBloomFilter(testBloomFilter)
	if err == nil {
		t.Errorf("Did not error when attempting to insert a bloom filter that " +
			"already exists.")
	}
}

// Happy path.
func TestMapImpl_DeleteBloomFilter(t *testing.T) {
	testID := rand.Uint64()
	testBloomFilter := &BloomFilter{Id: testID}
	m := &MapImpl{
		bloomFilters: map[uint64]*BloomFilter{testID: testBloomFilter},
	}

	err := m.DeleteBloomFilter(testID)

	if err != nil || m.bloomFilters[testID] != nil {
		t.Errorf("Failed to delete bloom filter: %v", err)
	}
}

// Error Path: The bloom filter does not exists in map.
func TestMapImpl_DeleteBloomFilter_NoFilterError(t *testing.T) {
	testID := rand.Uint64()
	m := &MapImpl{
		bloomFilters: make(map[uint64]*BloomFilter),
	}

	err := m.DeleteBloomFilter(testID)

	if err == nil {
		t.Errorf("No error received when attemting to delete bloom filter " +
			"that does not exist in map.")
	}
}

// Happy path.
func TestMapImpl_GetEphemeralBloomFilters(t *testing.T) {
	testRecipientIdID := id.NewIdFromUInt(rand.Uint64(), id.User, t)
	m := &MapImpl{
		ephemeralBloomFilters: map[uint64]*EphemeralBloomFilter{
			rand.Uint64(): {RecipientId: testRecipientIdID.Marshal()},
			rand.Uint64(): {RecipientId: testRecipientIdID.Marshal()},
			rand.Uint64(): {RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal()},
			rand.Uint64(): {RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal()},
			rand.Uint64(): {RecipientId: testRecipientIdID.Marshal()},
		},
	}

	ephemeralBloomFilters, err := m.GetEphemeralBloomFilters(testRecipientIdID)
	if err != nil {
		t.Errorf("Unexpected error retrieving ephemeral bloom filterss: %v", err)
	}
	if len(ephemeralBloomFilters) != 3 {
		t.Errorf("Received unexpected number of ephemeral bloom filterss: %v", ephemeralBloomFilters)
	}
}

// Error Path: No matching ephemeral bloom filters exist in the map.
func TestMapImpl_GetEphemeralBloomFilters_NoFiltersError(t *testing.T) {
	testClientID := id.NewIdFromUInt(rand.Uint64(), id.User, t)
	m := &MapImpl{
		ephemeralBloomFilters: map[uint64]*EphemeralBloomFilter{
			rand.Uint64(): {RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal()},
			rand.Uint64(): {RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal()},
		},
	}

	ephemeralBloomFilters, err := m.GetEphemeralBloomFilters(testClientID)
	if err == nil {
		t.Errorf("Expected an error when ephemeral bloom filterss is not in map.")
	}
	if ephemeralBloomFilters != nil {
		t.Errorf("Expected nil ephemeral bloom filterss returned. Received: %v",
			ephemeralBloomFilters)
	}
}

// Happy path.
func TestMapImpl_InsertEphemeralBloomFilter(t *testing.T) {
	testID := uint64(0)
	testEphemeralBloomFilter := &EphemeralBloomFilter{Id: testID}
	m := &MapImpl{
		ephemeralBloomFilters: make(map[uint64]*EphemeralBloomFilter),
	}

	err := m.InsertEphemeralBloomFilter(testEphemeralBloomFilter)
	if err != nil || m.ephemeralBloomFilters[testID] == nil {
		t.Errorf("Failed to insert ephemeral bloom filters: %v", err)
	}
}

// Error Path: Bloom filter already exists in map.
func TestMapImpl_InsertEphemeralBloomFilter_FilterAlreadyExistsError(t *testing.T) {
	testID := uint64(0)
	testEphemeralBloomFilter := &EphemeralBloomFilter{Id: testID}
	m := &MapImpl{
		ephemeralBloomFilters: map[uint64]*EphemeralBloomFilter{testID: testEphemeralBloomFilter},
	}

	err := m.InsertEphemeralBloomFilter(testEphemeralBloomFilter)
	if err == nil {
		t.Errorf("Did not error when attempting to insert a ephemeral bloom filters that " +
			"already exists.")
	}
}

// Happy path.
func TestMapImpl_DeleteEphemeralBloomFilter(t *testing.T) {
	testID := rand.Uint64()
	testEphemeralBloomFilter := &EphemeralBloomFilter{Id: testID}
	m := &MapImpl{
		ephemeralBloomFilters: map[uint64]*EphemeralBloomFilter{testID: testEphemeralBloomFilter},
	}

	err := m.DeleteEphemeralBloomFilter(testID)

	if err != nil || m.ephemeralBloomFilters[testID] != nil {
		t.Errorf("Failed to delete ephemeral bloom filters: %v", err)
	}
}

// Error Path: The ephemeral bloom filters does not exists in map.
func TestMapImpl_DeleteEphemeralBloomFilter_NoFilterError(t *testing.T) {
	testID := rand.Uint64()
	m := &MapImpl{
		ephemeralBloomFilters: make(map[uint64]*EphemeralBloomFilter),
	}

	err := m.DeleteEphemeralBloomFilter(testID)

	if err == nil {
		t.Errorf("No error received when attemting to delete ephemeral bloom filters " +
			"that does not exist in map.")
	}
}
