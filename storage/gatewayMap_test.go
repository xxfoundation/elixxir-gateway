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
	"reflect"
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
//
//	testClientId2 := []byte("testclient2")
//	testClient2 := id.NewIdFromBytes(testClientId2, t)
//	// testRecip := id.NewIdFromBytes(testBytes, t)
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
//
//	err = db.UpsertClient(&Client{
//		Id:      testClient2.Marshal(),
//		Key:     []byte("keystring1"),
//	})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//
//	err = db.UpsertClient(&Client{
//		Id:      testClient2.Marshal(),
//		Key:     []byte("keystring2"),
//	})
//	if err != nil {
//		t.Errorf(err.Error())
//		return
//	}
//
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
//	err = db.InsertMixedMessages([]*MixedMessage{{
//		RoundId:         testRound,
//		RecipientId:     testClient.Marshal(),
//		MessageContents: []byte("Test24"),
//	},},)
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
//	messages, err := db.getMixedMessages(testClient, testRoundId)
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

	mm := NewMixedMessage(testRoundId, testRecip, testBytes1, testBytes2)

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

	mm := NewMixedMessage(testRoundId, testRecip, testBytes1, testBytes2)
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
func TestMapImpl_countMixedMessagesByRound(t *testing.T) {
	testRoundID := rand.Uint64()
	m := &MapImpl{
		mixedMessages: MixedMessageMap{
			RoundId:      map[id.Round]map[id.ID]map[uint64]*MixedMessage{},
			RecipientId:  map[id.ID]map[id.Round]map[uint64]*MixedMessage{},
			RoundIdCount: map[id.Round]uint64{},
		},
	}

	// Add more messages with different recipient and round IDs.
	_ = m.InsertMixedMessages([]*MixedMessage{{
		Id:          rand.Uint64(),
		RoundId:     testRoundID,
		RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(),
	}, {
		Id:          rand.Uint64(),
		RoundId:     testRoundID,
		RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(),
	}, {
		Id:          rand.Uint64(),
		RoundId:     testRoundID,
		RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(),
	}})

	count, err := m.countMixedMessagesByRound(id.Round(testRoundID))
	if err != nil {
		t.Errorf("countMixedMessagesByRound() produced an error: %v", err)
	}

	if count != 3 {
		t.Errorf("countMixedMessagesByRound() returned incorrect count."+
			"\n\texpected: %v\n\treceived: %v", 3, count)
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
		mixedMessages: MixedMessageMap{
			RoundId:      map[id.Round]map[id.ID]map[uint64]*MixedMessage{testRoundID: {*testRecipientID: {testMsgID: testMixedMessage}}},
			RecipientId:  map[id.ID]map[id.Round]map[uint64]*MixedMessage{*testRecipientID: {testRoundID: {testMsgID: testMixedMessage}}},
			RoundIdCount: map[id.Round]uint64{testRoundID: 1},
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
	testMixedMessage = &MixedMessage{
		Id:          rand.Uint64(),
		RoundId:     uint64(testRoundID),
		RecipientId: testRecipientID.Marshal(),
	}
	_ = m.InsertMixedMessages([]*MixedMessage{testMixedMessage})

	testMixedMessage = &MixedMessage{
		Id:          rand.Uint64(),
		RoundId:     uint64(testRoundID),
		RecipientId: testRecipientID.Marshal(),
	}
	_ = m.InsertMixedMessages([]*MixedMessage{testMixedMessage})

	// Get list of 3 items
	mixedMsgs, err = m.getMixedMessages(testRecipientID, testRoundID)
	if err != nil {
		t.Errorf("Unexpected error retrieving mixedMessage: %v", err)
	}
	if len(mixedMsgs) != 3 {
		t.Errorf("Received unexpected number of MixedMessages: %v", mixedMsgs)
	}

	// Add more messages with different recipient and round IDs.
	testMixedMessage = &MixedMessage{
		Id:          rand.Uint64(),
		RoundId:     rand.Uint64(),
		RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(),
	}
	_ = m.InsertMixedMessages([]*MixedMessage{testMixedMessage})
	testMixedMessage = &MixedMessage{
		Id:          rand.Uint64(),
		RoundId:     rand.Uint64(),
		RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(),
	}
	_ = m.InsertMixedMessages([]*MixedMessage{testMixedMessage})

	// Get list of 3 items
	mixedMsgs, err = m.getMixedMessages(testRecipientID, testRoundID)
	if err != nil {
		t.Errorf("Unexpected error retrieving mixedMessage: %v", err)
	}
	if len(mixedMsgs) != 3 {
		t.Errorf("Received unexpected number of MixedMessages: %v", mixedMsgs)
	}
}

// m.mixedMessages.insert(t, testMixedMessage)
// Error Path: No matching messages exist in the map.
func TestMapImpl_getMixedMessages_NoMessageError(t *testing.T) {
	testRoundID := id.Round(rand.Uint64())
	testRecipientID := id.NewIdFromUInt(rand.Uint64(), id.User, t)
	m := &MapImpl{
		mixedMessages: MixedMessageMap{
			RoundId:      map[id.Round]map[id.ID]map[uint64]*MixedMessage{},
			RecipientId:  map[id.ID]map[id.Round]map[uint64]*MixedMessage{},
			RoundIdCount: map[id.Round]uint64{},
		},
	}

	_ = m.InsertMixedMessages([]*MixedMessage{
		{
			RoundId:     rand.Uint64(),
			RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(),
		}, {
			RoundId:     rand.Uint64(),
			RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(),
		},
	})

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
	roundID := id.Round(rand.Uint64())
	recipientId := id.NewIdFromUInt(rand.Uint64(), id.User, t)
	testMixedMessage := &MixedMessage{
		RoundId:     uint64(roundID),
		RecipientId: recipientId.Marshal(),
	}
	m := &MapImpl{
		mixedMessages: MixedMessageMap{
			RoundId:      map[id.Round]map[id.ID]map[uint64]*MixedMessage{},
			RecipientId:  map[id.ID]map[id.Round]map[uint64]*MixedMessage{},
			RoundIdCount: map[id.Round]uint64{},
		},
	}

	err := m.InsertMixedMessages([]*MixedMessage{testMixedMessage})
	if err != nil || m.mixedMessages.RecipientId[*recipientId][roundID] == nil ||
		m.mixedMessages.RoundId[roundID][*recipientId] == nil {
		t.Errorf("Failed to insert MixedMessage: %v", err)
	}

	if m.mixedMessages.RoundIdCount[roundID] != 1 {
		t.Errorf("Mixed message count incorrect: %d", m.mixedMessages.RoundIdCount[roundID])
	}
}

// Error Path: MixedMessage already exists in map.
func TestMapImpl_InsertMixedMessages_MessageAlreadyExistsError(t *testing.T) {
	roundId := id.Round(rand.Uint64())
	recipientId := *id.NewIdFromUInt(rand.Uint64(), id.User, t)
	testMixedMessage := &MixedMessage{
		RoundId:     uint64(roundId),
		RecipientId: recipientId.Marshal(),
	}
	m := &MapImpl{
		mixedMessages: MixedMessageMap{
			RoundId:      map[id.Round]map[id.ID]map[uint64]*MixedMessage{roundId: {recipientId: {testMixedMessage.Id: testMixedMessage}}},
			RecipientId:  map[id.ID]map[id.Round]map[uint64]*MixedMessage{recipientId: {roundId: {testMixedMessage.Id: testMixedMessage}}},
			RoundIdCount: map[id.Round]uint64{roundId: 1},
		},
	}

	err := m.InsertMixedMessages([]*MixedMessage{testMixedMessage})
	if err == nil {
		t.Errorf("Did not error when attempting to insert a mixedMessage that " +
			"already exists.")
	}
}

// Happy path
func TestMapImpl_DeleteMixedMessageByRound(t *testing.T) {
	testRoundId := id.Round(100)
	testRoundId2 := id.Round(2)
	testRecipientId := *id.NewIdFromUInt(5, id.User, t)
	m := &MapImpl{
		mixedMessages: MixedMessageMap{
			RoundId:      map[id.Round]map[id.ID]map[uint64]*MixedMessage{},
			RecipientId:  map[id.ID]map[id.Round]map[uint64]*MixedMessage{},
			RoundIdCount: map[id.Round]uint64{},
		},
	}

	// Insert message not to be deleted
	_ = m.InsertMixedMessages([]*MixedMessage{{
		RoundId:     uint64(testRoundId2),
		RecipientId: testRecipientId.Bytes(),
	}})

	// Insert two messages to be deleted
	_ = m.InsertMixedMessages([]*MixedMessage{
		{
			RoundId:     uint64(testRoundId),
			RecipientId: testRecipientId.Bytes(),
		}, {
			RoundId:     uint64(testRoundId),
			RecipientId: testRecipientId.Bytes(),
		},
	})

	// Delete the two messages
	err := m.DeleteMixedMessageByRound(testRoundId)
	if err != nil {
		t.Errorf("Unable to delete mixed messages by round: %+v", err)
	}

	// Ensure both messages were deleted
	if m.mixedMessages.RoundId[testRoundId][testRecipientId][1] != nil ||
		m.mixedMessages.RecipientId[testRecipientId][testRoundId][1] != nil {
		t.Errorf("Expected to delete message with id %d from map", 1)
	}
	if m.mixedMessages.RoundId[testRoundId][testRecipientId][2] != nil ||
		m.mixedMessages.RecipientId[testRecipientId][testRoundId][2] != nil {
		t.Errorf("Expected to delete message with id %d from map", 2)
	}

	// Ensure other message remains
	if m.mixedMessages.RoundId[testRoundId2][testRecipientId][0] == nil ||
		m.mixedMessages.RecipientId[testRecipientId][testRoundId2][0] == nil {
		t.Errorf("Incorrectly deleted message with id %d", 0)
	}
}

// Happy path.
func TestMapImpl_GetBloomFilters(t *testing.T) {
	testClientID := id.NewIdFromUInt(rand.Uint64(), id.User, t)
	m := &MapImpl{
		bloomFilters: BloomFilterMap{
			RecipientId: map[id.ID]map[uint64]*BloomFilter{},
			EpochId:     map[uint64]map[id.ID]*BloomFilter{},
		},
	}

	_ = m.UpsertBloomFilter(&BloomFilter{RecipientId: testClientID.Marshal(), EpochId: rand.Uint64()})
	_ = m.UpsertBloomFilter(&BloomFilter{RecipientId: testClientID.Marshal(), EpochId: rand.Uint64()})
	_ = m.UpsertBloomFilter(&BloomFilter{RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(), EpochId: rand.Uint64()})
	_ = m.UpsertBloomFilter(&BloomFilter{RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(), EpochId: rand.Uint64()})
	_ = m.UpsertBloomFilter(&BloomFilter{RecipientId: testClientID.Marshal(), EpochId: rand.Uint64()})

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
		bloomFilters: BloomFilterMap{
			RecipientId: map[id.ID]map[uint64]*BloomFilter{},
			EpochId:     map[uint64]map[id.ID]*BloomFilter{},
		},
	}
	_ = m.UpsertBloomFilter(&BloomFilter{RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(), EpochId: rand.Uint64()})
	_ = m.UpsertBloomFilter(&BloomFilter{RecipientId: id.NewIdFromUInt(rand.Uint64(), id.User, t).Marshal(), EpochId: rand.Uint64()})

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
	testRecipientId := *id.NewIdFromUInt(rand.Uint64(), id.User, t)
	testEpochId := rand.Uint64()
	testBloomFilter := &BloomFilter{
		RecipientId: testRecipientId.Marshal(),
		EpochId:     testEpochId,
	}
	m := &MapImpl{
		bloomFilters: BloomFilterMap{
			RecipientId: map[id.ID]map[uint64]*BloomFilter{},
			EpochId:     map[uint64]map[id.ID]*BloomFilter{},
		},
	}

	err := m.UpsertBloomFilter(testBloomFilter)
	if err != nil || m.bloomFilters.RecipientId[testRecipientId][testEpochId] == nil ||
		m.bloomFilters.EpochId[testEpochId][testRecipientId] == nil {
		t.Errorf("Failed to insert BloomFilter: %v", err)
	}
}

// Happy path.
func TestMapImpl_DeleteBloomFilterByEpoch(t *testing.T) {
	epochId := rand.Uint64()
	recipientId := *id.NewIdFromUInt(rand.Uint64(), id.User, t)
	vals := []struct {
		recipientId id.ID
		epochId     uint64
	}{
		{*id.NewIdFromUInt(rand.Uint64(), id.User, t), rand.Uint64()},
		{recipientId, rand.Uint64()},
		{recipientId, epochId},
		{*id.NewIdFromUInt(rand.Uint64(), id.User, t), epochId},
	}

	m := &MapImpl{
		bloomFilters: BloomFilterMap{
			RecipientId: map[id.ID]map[uint64]*BloomFilter{},
			EpochId:     map[uint64]map[id.ID]*BloomFilter{},
		},
	}

	// Insert the messages
	for _, val := range vals {
		_ = m.UpsertBloomFilter(&BloomFilter{
			RecipientId: val.recipientId.Marshal(),
			EpochId:     val.epochId,
		})
	}

	// Delete two of the filters
	err := m.DeleteBloomFilterByEpoch(epochId)
	if err != nil {
		t.Errorf("Unable to delete bloom filters by epochId: %+v", err)
	}

	// Ensure both filters were deleted
	for i, val := range vals {
		if val.epochId == epochId {
			if m.bloomFilters.RecipientId[val.recipientId][val.epochId] != nil ||
				m.bloomFilters.EpochId[val.epochId][val.recipientId] != nil {
				t.Errorf("Expected to delete bloom filter %d from map", i)
			}
		}
	}

	// Ensure the other filter remains
	for i, val := range vals {
		if val.epochId != epochId {
			if m.bloomFilters.RecipientId[val.recipientId][val.epochId] == nil ||
				m.bloomFilters.EpochId[val.epochId][val.recipientId] == nil {
				t.Errorf("Incorrectly deleted bloom filter %d: %+v", i, val)
			}
		}
	}
}

// Error Path: The bloom filter does not exists in map.
func TestMapImpl_DeleteBloomFilterByEpoch_NoFilterError(t *testing.T) {
	epochId := rand.Uint64()
	recipientId := *id.NewIdFromUInt(rand.Uint64(), id.User, t)
	vals := []struct {
		recipientId id.ID
		epochId     uint64
	}{
		{*id.NewIdFromUInt(rand.Uint64(), id.User, t), rand.Uint64()},
		{recipientId, rand.Uint64()},
		{recipientId, rand.Uint64()},
		{*id.NewIdFromUInt(rand.Uint64(), id.User, t), rand.Uint64()},
	}

	m := &MapImpl{
		bloomFilters: BloomFilterMap{
			RecipientId: map[id.ID]map[uint64]*BloomFilter{},
			EpochId:     map[uint64]map[id.ID]*BloomFilter{},
		},
	}

	// Insert the messages
	for _, val := range vals {
		_ = m.UpsertBloomFilter(&BloomFilter{
			RecipientId: val.recipientId.Marshal(),
			EpochId:     val.epochId,
		})
	}

	// Attempt to delete
	err := m.DeleteBloomFilterByEpoch(epochId)
	if err == nil {
		t.Errorf("No error ocurred when bloom filters do not exist.")
	}

	// Ensure all filters still exist
	for i, val := range vals {
		if m.bloomFilters.RecipientId[val.recipientId][val.epochId] == nil ||
			m.bloomFilters.EpochId[val.epochId][val.recipientId] == nil {
			t.Errorf("Incorrectly deleted bloom filter %d: %+v", i, val)
		}
	}
}

// Happy path.
func TestMapImpl_InsertEpoch(t *testing.T) {
	testRid := id.Round(rand.Uint64())
	m := &MapImpl{
		epochs: EpochMap{
			M:       map[uint64]*Epoch{},
			IdTrack: 0,
		},
	}

	_, err := m.InsertEpoch(testRid)
	if err != nil || m.epochs.M[0] == nil || m.epochs.M[0].RoundId != uint64(testRid) {
		t.Errorf("Failed to insert epoch: %+v", err)
	}
}

// Happy path.
func TestMapImpl_GetEpoch(t *testing.T) {
	testRid := id.Round(rand.Uint64())
	m := &MapImpl{
		epochs: EpochMap{
			M:       map[uint64]*Epoch{},
			IdTrack: 0,
		},
	}
	_, _ = m.InsertEpoch(testRid)

	epoch, err := m.GetEpoch(0)
	if err != nil || epoch.RoundId != uint64(testRid) {
		t.Errorf("Failed to get epoch: %+v", err)
	}
}

// Error path: no matching epoch exists.
func TestMapImpl_GetEpoch_NoMatchingEpochError(t *testing.T) {
	m := &MapImpl{
		epochs: EpochMap{
			M:       map[uint64]*Epoch{},
			IdTrack: 0,
		},
	}

	epoch, err := m.GetEpoch(0)
	if err == nil || epoch != nil {
		t.Errorf("Retrieved epoch when one should not exist")
	}
}

// Happy path.
func TestMapImpl_GetLatestEpoch(t *testing.T) {
	m := &MapImpl{
		epochs: EpochMap{
			M:       map[uint64]*Epoch{},
			IdTrack: 0,
		},
	}
	_, _ = m.InsertEpoch(id.Round(rand.Uint64()))
	_, _ = m.InsertEpoch(id.Round(rand.Uint64()))
	_, _ = m.InsertEpoch(id.Round(rand.Uint64()))
	_, _ = m.InsertEpoch(id.Round(rand.Uint64()))
	epoch, _ := m.InsertEpoch(id.Round(rand.Uint64()))

	testEpoch, err := m.GetLatestEpoch()
	if err != nil || !reflect.DeepEqual(epoch, testEpoch) {
		t.Errorf("Failed to get correct latest epoch: %+v"+
			"\n\texpected: %+v\n\treceived: %+v", err, epoch, testEpoch)
	}
}

// Error path: no epochs exist in map
func TestMapImpl_GetLatestEpoch_NoEpochsInMapError(t *testing.T) {
	m := &MapImpl{
		epochs: EpochMap{
			M:       map[uint64]*Epoch{},
			IdTrack: 0,
		},
	}

	_, err := m.GetLatestEpoch()
	if err == nil {
		t.Errorf("Expected error when epoch map is empty.")
	}
}

func TestMapImpl_UpsertClient(t *testing.T) {
	testKey := id.NewIdFromString("testKey1", id.User, t)
	testClient := &Client{Id: testKey.Marshal(), Key: []byte("testkey1")}
	m := &MapImpl{
		clients: make(map[id.ID]*Client),
	}

	err := m.UpsertClient(testClient)
	if err != nil || m.clients[*testKey] == nil {
		t.Errorf("Failed to insert client: %v", err)
	}

	testClient.Key = []byte("testkey2")

	err = m.UpsertClient(testClient)
	if err != nil || !bytes.Equal(m.clients[*testKey].Key, []byte("testkey2")) {
		t.Errorf("Failed to upsert client: %v", err)
	}
}
