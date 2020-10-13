///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package storage

import (
	"bytes"
	"gitlab.com/xx_network/primitives/id"
	"math/rand"
	"testing"
)

// Hidden function for one-time unit testing database implementation
//func TestDatabaseImpl(t *testing.T) {
//
//	jwalterweatherman.SetLogThreshold(jwalterweatherman.LevelTrace)
//	jwalterweatherman.SetStdoutThreshold(jwalterweatherman.LevelTrace)
//
//	db, _, err := newDatabase("cmix", "", "cmix_gateway", "0.0.0.0", "5432")
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//
//	testBytes := []byte("test")
//	testBytes2 := []byte("words")
//	testClientId := []byte("client")
//	testRound := uint64(10)
//	testRound2 := uint64(11)
//	testRound3 := uint64(12)
//
//	testClient := id.NewIdFromBytes(testClientId, t)
//	testRecip := id.NewIdFromBytes(testBytes, t)
//	testRoundId := id.Round(testRound)
//	testRoundId3 := id.Round(testRound3)
//	testEpoch, err := db.InsertEpoch(testRoundId)
//	if err != nil {
//		t.Errorf("%+v", err)
//	}
//	testEpoch2, err := db.InsertEpoch(testRoundId)
//	if err != nil {
//		t.Errorf("%+v", err)
//	}
//
//	rtnEpoch, err := db.GetEpoch(testEpoch.Id)
//	if err != nil || rtnEpoch == nil {
//		t.Errorf("%+v, %+v", rtnEpoch, err)
//	}
//
//	latestEpoch, err := db.GetLatestEpoch()
//	if err != nil {
//		t.Errorf("%+v", err)
//	}
//
//	if testEpoch2.Id != latestEpoch.Id {
//		t.Errorf("Expected epoch ids to match!")
//	}
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
//		UpdateId: 50,
//		InfoBlob: testBytes,
//	})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	err = db.UpsertRound(&Round{
//		Id:       testRound2,
//		UpdateId: 51,
//		InfoBlob: testBytes,
//	})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	err = db.UpsertRound(&Round{
//		Id:       testRound3,
//		UpdateId: 52,
//		InfoBlob: testBytes,
//	})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	err = db.UpsertBloomFilter(&BloomFilter{
//		RecipientId:    testClient.Marshal(),
//		Filter:      testBytes,
//		EpochId: 1,
//	})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	err = db.UpsertBloomFilter(&BloomFilter{
//		RecipientId:    testClient.Marshal(),
//		Filter:      testBytes2,
//		EpochId: testEpoch.Id,
//	})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	err = db.InsertMixedMessages([]*MixedMessage{{
//		RoundId:         testRound,
//		RecipientId:     testClient.Marshal(),
//		MessageContents: testBytes,
//	}, {
//		RoundId:         testRound,
//		RecipientId:     testClient.Marshal(),
//		MessageContents: testBytes,
//	}, {
//		RoundId:         testRound + 1,
//		RecipientId:     testClient.Marshal(),
//		MessageContents: testBytes,
//	}})
//	count, err := db.countMixedMessagesByRound(testRoundId)
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	if count != 2 {
//		t.Errorf("Unexpected count! Got %d", count)
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
//	err = db.DeleteMixedMessageByRound(testRoundId)
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
//	jwalterweatherman.INFO.Printf("%+v", client)
//	round, err := db.GetRound(testRoundId)
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	jwalterweatherman.INFO.Printf("%+v", round)
//	rounds, err := db.GetRounds([]id.Round{testRoundId, testRoundId3})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	jwalterweatherman.INFO.Printf("%+v", rounds[1])
//	messages, err := db.GetMixedMessages(testClient, testRoundId)
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	jwalterweatherman.INFO.Printf("%+v", messages)
//	filters, err := db.getBloomFilters(testClient)
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//	jwalterweatherman.INFO.Printf("%+v", filters)
//
//	err = db.DeleteBloomFilterByEpoch(testEpoch.Id)
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//}

// Happy path
func TestNewMixedMessage(t *testing.T) {
	testBytes := []byte("test1234")
	testBytes1 := []byte("test")
	testBytes2 := []byte("1234")
	testRound := uint64(10)
	testRecip := id.NewIdFromBytes(testBytes, t)
	testRoundId := id.Round(testRound)

	mm := NewMixedMessage(&testRoundId, testRecip, testBytes1, testBytes2)

	if mm.Id != 0 {
		t.Errorf("Invalid Id: %d", mm.Id)
	}
	if mm.RoundId != testRound {
		t.Errorf("Invalid Round Id: %d", mm.RoundId)
	}
	if bytes.Compare(mm.RecipientId, testRecip.Marshal()) != 0 {
		t.Errorf("Invalid Recipient Id: %v", mm.RecipientId)
	}
	if bytes.Compare(mm.MessageContents, testBytes) != 0 {
		t.Errorf("Invalid Message Contents: %v", mm.MessageContents)
	}
}

// Happy path
func TestMixedMessage_GetMessageContents(t *testing.T) {
	testBytes := []byte("test1234")
	testBytes1 := []byte("test")
	testBytes2 := []byte("1234")
	testRound := uint64(10)
	testRecip := id.NewIdFromBytes(testBytes, t)
	testRoundId := id.Round(testRound)

	mm := NewMixedMessage(&testRoundId, testRecip, testBytes1, testBytes2)
	messageContentsA, messageContentsB := mm.GetMessageContents()

	if bytes.Compare(testBytes1, messageContentsA) != 0 {
		t.Errorf("Invalid message contents A: %v", string(messageContentsA))
	}
	if bytes.Compare(testBytes2, messageContentsB) != 0 {
		t.Errorf("Invalid message contents B: %v", string(messageContentsB))
	}
}

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

	round, err := m.GetRound(testKey)
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

	round, err := m.GetRound(testKey)
	if err == nil || round != nil {
		t.Errorf("No error returned when round does not exist.")
	}
}

// Happy path.
func TestMapImpl_GetRounds(t *testing.T) {
	testKey := id.Round(40)
	testRound := &Round{Id: uint64(testKey)}
	testKey2 := id.Round(50)
	testRound2 := &Round{Id: uint64(testKey2)}
	m := &MapImpl{
		rounds: map[id.Round]*Round{testKey: testRound, testKey2: testRound2},
	}

	rounds, err := m.GetRounds([]id.Round{testKey, testKey2})
	if err != nil || len(rounds) != 2 {
		t.Errorf("Failed to get rounds: %v", err)
	}
}

// Error Path: Rounds not in map.
func TestMapImpl_GetRounds_NoRoundError(t *testing.T) {
	testKey := id.Round(40)
	testRound := &Round{Id: uint64(testKey)}
	testKey2 := id.Round(50)
	testRound2 := &Round{Id: uint64(testKey2)}
	invalidKey := id.Round(30)
	invalidKey2 := id.Round(20)
	m := &MapImpl{
		rounds: map[id.Round]*Round{testKey: testRound, testKey2: testRound2},
	}

	rounds, err := m.GetRounds([]id.Round{invalidKey, invalidKey2})
	if err == nil || rounds != nil {
		t.Errorf("No error returned when rounds do not exist.")
	}
}

// Happy path.
func TestMapImpl_UpsertRound(t *testing.T) {
	testKey := id.Round(rand.Uint64())
	testRounds := []*Round{
		{Id: uint64(testKey), UpdateId: 0},
		{Id: uint64(testKey), UpdateId: 1},
	}
	m := &MapImpl{
		rounds: make(map[id.Round]*Round),
	}

	err := m.UpsertRound(testRounds[0])
	if err != nil || m.rounds[testKey] == nil {
		t.Errorf("Failed to insert round: %v", err)
	}

	err = m.UpsertRound(testRounds[1])
	if err != nil || m.rounds[testKey] == nil {
		t.Errorf("Failed to insert round: %v", err)
	}
}

// Neutral path: round exists but update ID is smaller than the one in the map.
func TestMapImpl_UpsertRound_RoundAlreadyExists(t *testing.T) {
	testKey := id.Round(rand.Uint64())
	testRounds := []*Round{
		{Id: uint64(testKey), UpdateId: 2},
		{Id: uint64(testKey), UpdateId: 0},
	}
	m := &MapImpl{
		rounds: map[id.Round]*Round{testKey: testRounds[0]},
	}

	err := m.UpsertRound(testRounds[1])
	if err != nil || m.rounds[testKey].UpdateId != testRounds[0].UpdateId {
		t.Errorf("Round updated in map even though update ID is greater.")
	}
}

// Happy path.
func TestMapImpl_getMixedMessages(t *testing.T) {
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
	mixedMsgs, err := m.getMixedMessages(testRecipientID, testRoundID)
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
	mixedMsgs, err = m.getMixedMessages(testRecipientID, testRoundID)
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
	mixedMsgs, err = m.getMixedMessages(testRecipientID, testRoundID)
	if err != nil {
		t.Errorf("Unexpected error retrieving mixedMessage: %v", err)
	}
	if len(mixedMsgs) != 3 {
		t.Errorf("Received unexpected number of MixedMessages: %v", mixedMsgs)
	}
}

// Error Path: No matching messages exist in the map.
func TestMapImpl_getMixedMessages_NoMessageError(t *testing.T) {
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
	mixedMsgs, err := m.getMixedMessages(testRecipientID, testRoundID)
	if err == nil {
		t.Errorf("Expected an error when mixedMessage is not found in map.")
	}
	if mixedMsgs != nil {
		t.Errorf("Expected nil mixedMessages. Received: %v", mixedMsgs)
	}
}

// Happy path.
func TestMapImpl_InsertMixedMessages(t *testing.T) {
	testMsgID := uint64(0)
	testMixedMessage := &MixedMessage{
		RoundId:     rand.Uint64(),
		RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(),
	}
	m := &MapImpl{
		mixedMessages: make(map[uint64]*MixedMessage),
	}

	err := m.InsertMixedMessages([]*MixedMessage{testMixedMessage})
	if err != nil || m.mixedMessages[testMsgID] == nil {
		t.Errorf("Failed to insert MixedMessage: %v", err)
	}
}

// Error Path: MixedMessage already exists in map.
func TestMapImpl_InsertMixedMessages_MessageAlreadyExistsError(t *testing.T) {
	testMsgID := uint64(0)
	testMixedMessage := &MixedMessage{
		RoundId:     rand.Uint64(),
		RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(),
	}
	m := &MapImpl{
		mixedMessages: map[uint64]*MixedMessage{testMsgID: testMixedMessage},
	}

	err := m.InsertMixedMessages([]*MixedMessage{testMixedMessage})
	if err == nil {
		t.Errorf("Did not error when attempting to insert a mixedMessage that " +
			"already exists.")
	}
}

// Happy path
func TestMapImpl_DeleteMixedMessageByRound(t *testing.T) {
	testMsgId1 := uint64(1000)
	testMsgId2 := uint64(2000)
	testMsgId3 := uint64(3000)
	testRoundId := uint64(100)
	m := &MapImpl{
		mixedMessages: make(map[uint64]*MixedMessage),
	}

	// Insert message not to be deleted
	m.mixedMessages[testMsgId3] = &MixedMessage{
		Id:      testMsgId3,
		RoundId: uint64(2),
	}

	// Insert two messages to be deleted
	m.mixedMessages[testMsgId1] = &MixedMessage{
		Id:      testMsgId1,
		RoundId: testRoundId,
	}
	m.mixedMessages[testMsgId2] = &MixedMessage{
		Id:      testMsgId2,
		RoundId: testRoundId,
	}

	// Delete the two messages
	err := m.DeleteMixedMessageByRound(id.Round(testRoundId))
	if err != nil {
		t.Errorf("Unable to delete mixed messages by round: %+v", err)
	}

	// Ensure both messages were deleted
	if _, exists := m.mixedMessages[testMsgId1]; exists {
		t.Errorf("Expected to delete message with id %d", testMsgId1)
	}
	if _, exists := m.mixedMessages[testMsgId2]; exists {
		t.Errorf("Expected to delete message with id %d", testMsgId2)
	}

	// Ensure other message remains
	if _, exists := m.mixedMessages[testMsgId3]; !exists {
		t.Errorf("Incorrectly deleted message with id %d", testMsgId3)
	}
}

// Happy path.
func TestMapImpl_GetBloomFilters(t *testing.T) {
	testClientID := id.NewIdFromUInt(rand.Uint64(), id.User, t)
	m := &MapImpl{
		bloomFilters: map[uint64]*BloomFilter{
			rand.Uint64(): {RecipientId: testClientID.Marshal()},
			rand.Uint64(): {RecipientId: testClientID.Marshal()},
			rand.Uint64(): {RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal()},
			rand.Uint64(): {RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal()},
			rand.Uint64(): {RecipientId: testClientID.Marshal()},
		},
	}

	bloomFilters, err := m.getBloomFilters(testClientID)
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
			rand.Uint64(): {RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal()},
			rand.Uint64(): {RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal()},
		},
	}

	bloomFilters, err := m.getBloomFilters(testClientID)
	if err == nil {
		t.Errorf("Expected an error when bloom filters is not in map.")
	}
	if bloomFilters != nil {
		t.Errorf("Expected nil bloom filters returned. Received: %v",
			bloomFilters)
	}
}

// Happy path.
func TestMapImpl_UpsertBloomFilter(t *testing.T) {
	// TODO
}

// Happy path.
func TestMapImpl_DeleteBloomFilterByEpoch(t *testing.T) {
	// TODO
}

// Error Path: The bloom filter does not exists in map.
func TestMapImpl_DeleteBloomFilterByEpoch_NoFilterError(t *testing.T) {
	// TODO
}
