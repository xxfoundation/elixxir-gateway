///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package storage

import (
	"bytes"
	"github.com/spf13/jwalterweatherman"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

// Hidden function for one-time unit testing database implementation
func TestDatabaseImpl(t *testing.T) {

	jwalterweatherman.SetLogThreshold(jwalterweatherman.LevelTrace)
	jwalterweatherman.SetStdoutThreshold(jwalterweatherman.LevelTrace)

	db, err := newDatabase("cmix", "", "cmix_gateway", "0.0.0.0", "5432")
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	testBytes := []byte("tests")
	testBytes2 := []byte("words")
	testClientId := []byte("client")
	testRound := uint64(10)
	//testRound2 := uint64(11)
	//testRound3 := uint64(12)

	testClient := id.NewIdFromBytes(testClientId, t)
	testEphem, err := ephemeral.GetId(testClient, 64, uint64(time.Now().UnixNano()))
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	//testClientId2 := []byte("testclient2")
	//testClient2 := id.NewIdFromBytes(testClientId2, t)
	//testRecip := id.NewIdFromBytes(testBytes, t)
	//testRoundId := id.Round(testRound)
	//testRoundId3 := id.Round(testRound3)
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
	//	err = db.upsertClientBloomFilter(&ClientBloomFilter{
	//		RecipientId:    1,
	//		Filter:      testBytes2,
	//		FirstRound: 5,
	//		Epoch: 1,
	//	})
	//	if err != nil {
	//		t.Errorf(err.Error())
	//		return
	//	}
	//	err = db.upsertClientBloomFilter(&ClientBloomFilter{
	//		RecipientId:    1,
	//		Filter:      testBytes,
	//		Epoch: 1,
	//		FirstRound: 10,
	//	})
	//	if err != nil {
	//		t.Errorf(err.Error())
	//		return
	//	}
	//	err = db.upsertClientBloomFilter(&ClientBloomFilter{
	//		RecipientId:    1,
	//		Filter:      testBytes,
	//		Epoch: 1,
	//		FirstRound: 7,
	//	})
	//	if err != nil {
	//		t.Errorf(err.Error())
	//		return
	//	}
	//	err = db.upsertClientBloomFilter(&ClientBloomFilter{
	//		RecipientId:    1,
	//		Filter:      testBytes2,
	//		Epoch: 1,
	//		FirstRound: 1,
	//	})
	//	if err != nil {
	//		t.Errorf(err.Error())
	//		return
	//	}
	//	err = db.upsertClientBloomFilter(&ClientBloomFilter{
	//		RecipientId:    1,
	//		Filter:      testBytes2,
	//		Epoch: 3,
	//		FirstRound: 15,
	//	})
	//	if err != nil {
	//		t.Errorf(err.Error())
	//		return
	//	}
	//	err = db.upsertClientBloomFilter(&ClientBloomFilter{
	//		RecipientId:    1,
	//		Filter:      []byte("birds"),
	//		Epoch: 3,
	//		FirstRound: 20,
	//	})
	//	if err != nil {
	//		t.Errorf(err.Error())
	//		return
	//	}
	//err = db.InsertMixedMessages(&ClientRound{
	//	Id:        testRound,
	//	Timestamp: time.Now(),
	//	Messages: []MixedMessage{{
	//		RoundId:         testRound,
	//		RecipientId:     testEphem.Int64(),
	//		MessageContents: testBytes,
	//	}, {
	//		RoundId:         testRound,
	//		RecipientId:     testEphem.Int64(),
	//		MessageContents: testBytes,
	//	}, {
	//		RoundId:         testRound,
	//		RecipientId:     testEphem.Int64(),
	//		MessageContents: testBytes2,
	//	}},
	//})
	//if err != nil {
	//	t.Errorf(err.Error())
	//	return
	//}
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
	//err = db.deleteMixedMessages(time.Now().Add(1 * time.Hour))
	//if err != nil {
	//	t.Errorf(err.Error())
	//	return
	//}
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
	//	filters, err := db.GetClientBloomFilters(&testEphem, 1, 5)
	//	if err != nil {
	//		t.Errorf(err.Error())
	//		return
	//	}
	//	jwalterweatherman.INFO.Printf("%+v", filters)
	//
	//	err = db.DeleteClientFiltersBeforeEpoch(3)
	//	if err != nil {
	//		t.Errorf(err.Error())
	//		return
	//	}
}

// Happy path
func TestNewMixedMessage(t *testing.T) {
	testBytes := []byte("test1234")
	testBytes1 := []byte("test")
	testBytes2 := []byte("1234")
	testRound := uint64(10)
	testRecip := &ephemeral.Id{1, 2, 3}
	testRoundId := id.Round(testRound)

	mm := NewMixedMessage(testRoundId, testRecip, testBytes1, testBytes2)

	if mm.Id != 0 {
		t.Errorf("Invalid Id: %d", mm.Id)
	}
	if mm.RoundId != testRound {
		t.Errorf("Invalid Round Id: %d", mm.RoundId)
	}
	if mm.RecipientId != testRecip.Int64() {
		t.Errorf("Invalid Recipient Id: %v", mm.RecipientId)
	}
	if !bytes.Equal(mm.MessageContents, testBytes) {
		t.Errorf("Invalid Message Contents: %v", mm.MessageContents)
	}
}

// Happy path
func TestMixedMessage_GetMessageContents(t *testing.T) {
	testBytes1 := []byte("test")
	testBytes2 := []byte("1234")
	testRound := uint64(10)
	testRecip := &ephemeral.Id{1, 2, 3}
	testRoundId := id.Round(testRound)

	mm := NewMixedMessage(testRoundId, testRecip, testBytes1, testBytes2)
	messageContentsA, messageContentsB := mm.GetMessageContents()

	if !bytes.Equal(testBytes1, messageContentsA) {
		t.Errorf("Invalid message contents A: %v", string(messageContentsA))
	}
	if !bytes.Equal(testBytes2, messageContentsB) {
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

// TODO: Fix tests broken below sorry fam
//// Happy path.
//func TestMapImpl_countMixedMessagesByRound(t *testing.T) {
//	testRoundID := rand.Uint64()
//	m := &MapImpl{
//		mixedMessages: MixedMessageMap{
//			RoundId:      map[id.Round]map[int64]map[uint64]*MixedMessage{},
//			RecipientId:  map[int64]map[id.Round]map[uint64]*MixedMessage{},
//			RoundIdCount: map[id.Round]uint64{},
//		},
//	}
//
//	// Add more messages with different recipient and round IDs.
//	_ = m.InsertMixedMessages([]*MixedMessage{{
//		Id:          rand.Uint64(),
//		RoundId:     testRoundID,
//		RecipientId: rand.Int63(),
//	}, {
//		Id:          rand.Uint64(),
//		RoundId:     testRoundID,
//		RecipientId: rand.Int63(),
//	}, {
//		Id:          rand.Uint64(),
//		RoundId:     testRoundID,
//		RecipientId: rand.Int63(),
//	}})
//
//	count, err := m.countMixedMessagesByRound(id.Round(testRoundID))
//	if err != nil {
//		t.Errorf("countMixedMessagesByRound() produced an error: %v", err)
//	}
//
//	if count != 3 {
//		t.Errorf("countMixedMessagesByRound() returned incorrect count."+
//			"\n\texpected: %v\n\treceived: %v", 3, count)
//	}
//}
//
//// Happy path.
//func TestMapImpl_getMixedMessages(t *testing.T) {
//	testMsgID := rand.Uint64()
//	testRoundID := id.Round(rand.Uint64())
//	testRecipientID := &ephemeral.Id{1, 2, 3}
//	testMixedMessage := &MixedMessage{
//		Id:          testMsgID,
//		RoundId:     uint64(testRoundID),
//		RecipientId: testRecipientID.Int64(),
//	}
//	m := &MapImpl{
//		mixedMessages: MixedMessageMap{
//			RoundId: map[id.Round]map[int64]map[uint64]*MixedMessage{
//				testRoundID: {testRecipientID.Int64(): {testMsgID: testMixedMessage}},
//			},
//			RecipientId: map[int64]map[id.Round]map[uint64]*MixedMessage{
//				testRecipientID.Int64(): {testRoundID: {testMsgID: testMixedMessage}},
//			},
//			RoundIdCount: map[id.Round]uint64{testRoundID: 1},
//		},
//	}
//
//	// Get list of 1 item
//	mixedMsgs, err := m.getMixedMessages(testRecipientID, testRoundID)
//	if err != nil {
//		t.Errorf("Unexpected error retrieving mixedMessage: %v", err)
//	}
//	if len(mixedMsgs) != 1 {
//		t.Errorf("Received unexpected number of MixedMessages: %v", mixedMsgs)
//	}
//
//	// Add more messages with same recipient and round IDs.
//	testMixedMessage = &MixedMessage{
//		Id:          rand.Uint64(),
//		RoundId:     uint64(testRoundID),
//		RecipientId: testRecipientID.Int64(),
//	}
//	_ = m.InsertMixedMessages([]*MixedMessage{testMixedMessage})
//
//	testMixedMessage = &MixedMessage{
//		Id:          rand.Uint64(),
//		RoundId:     uint64(testRoundID),
//		RecipientId: testRecipientID.Int64(),
//	}
//	_ = m.InsertMixedMessages([]*MixedMessage{testMixedMessage})
//
//	// Get list of 3 items
//	mixedMsgs, err = m.getMixedMessages(testRecipientID, testRoundID)
//	if err != nil {
//		t.Errorf("Unexpected error retrieving mixedMessage: %v", err)
//	}
//	if len(mixedMsgs) != 3 {
//		t.Errorf("Received unexpected number of MixedMessages: %v", mixedMsgs)
//	}
//
//	// Add more messages with different recipient and round IDs.
//	testMixedMessage = &MixedMessage{
//		Id:          rand.Uint64(),
//		RoundId:     rand.Uint64(),
//		RecipientId: rand.Int63(),
//	}
//	_ = m.InsertMixedMessages([]*MixedMessage{testMixedMessage})
//	testMixedMessage = &MixedMessage{
//		Id:          rand.Uint64(),
//		RoundId:     rand.Uint64(),
//		RecipientId: rand.Int63(),
//	}
//	_ = m.InsertMixedMessages([]*MixedMessage{testMixedMessage})
//
//	// Get list of 3 items
//	mixedMsgs, err = m.getMixedMessages(testRecipientID, testRoundID)
//	if err != nil {
//		t.Errorf("Unexpected error retrieving mixedMessage: %v", err)
//	}
//	if len(mixedMsgs) != 3 {
//		t.Errorf("Received unexpected number of MixedMessages: %v", mixedMsgs)
//	}
//}
//
//// m.mixedMessages.insert(t, testMixedMessage)
//// Error Path: No matching messages exist in the map.
//func TestMapImpl_getMixedMessages_NoMessageError(t *testing.T) {
//	testRoundID := id.Round(rand.Uint64())
//	testRecipientID := &ephemeral.Id{1, 2, 3}
//	m := &MapImpl{
//		mixedMessages: MixedMessageMap{
//			RoundId:      map[id.Round]map[int64]map[uint64]*MixedMessage{},
//			RecipientId:  map[int64]map[id.Round]map[uint64]*MixedMessage{},
//			RoundIdCount: map[id.Round]uint64{},
//		},
//	}
//
//	_ = m.InsertMixedMessages([]*MixedMessage{
//		{
//			RoundId:     rand.Uint64(),
//			RecipientId: rand.Int63(),
//		}, {
//			RoundId:     rand.Uint64(),
//			RecipientId: rand.Int63(),
//		},
//	})
//
//	// Attempt to get message that is not in map
//	mixedMsgs, err := m.getMixedMessages(testRecipientID, testRoundID)
//	if err == nil {
//		t.Errorf("Expected an error when mixedMessage is not found in map.")
//	}
//	if mixedMsgs != nil {
//		t.Errorf("Expected nil mixedMessages. Received: %v", mixedMsgs)
//	}
//}
//
//// Happy path.
//func TestMapImpl_InsertMixedMessages(t *testing.T) {
//	roundID := id.Round(rand.Uint64())
//	recipientId := &ephemeral.Id{1, 2, 3}
//	testMixedMessage := &MixedMessage{
//		RoundId:     uint64(roundID),
//		RecipientId: recipientId.Int64(),
//	}
//	m := &MapImpl{
//		mixedMessages: MixedMessageMap{
//			RoundId:      map[id.Round]map[int64]map[uint64]*MixedMessage{},
//			RecipientId:  map[int64]map[id.Round]map[uint64]*MixedMessage{},
//			RoundIdCount: map[id.Round]uint64{},
//		},
//	}
//
//	err := m.InsertMixedMessages([]*MixedMessage{testMixedMessage})
//	if err != nil || m.mixedMessages.RecipientId[recipientId.Int64()][roundID] == nil ||
//		m.mixedMessages.RoundId[roundID][recipientId.Int64()] == nil {
//		t.Errorf("Failed to insert MixedMessage: %v", err)
//	}
//
//	if m.mixedMessages.RoundIdCount[roundID] != 1 {
//		t.Errorf("Mixed message count incorrect: %d", m.mixedMessages.RoundIdCount[roundID])
//	}
//}
//
//// Error Path: MixedMessage already exists in map.
//func TestMapImpl_InsertMixedMessages_MessageAlreadyExistsError(t *testing.T) {
//	roundId := id.Round(rand.Uint64())
//	recipientId := &ephemeral.Id{1, 2, 3}
//	testMixedMessage := &MixedMessage{
//		RoundId:     uint64(roundId),
//		RecipientId: recipientId.Int64(),
//	}
//	m := &MapImpl{
//		mixedMessages: MixedMessageMap{
//			RoundId: map[id.Round]map[int64]map[uint64]*MixedMessage{
//				roundId: {recipientId.Int64(): {testMixedMessage.Id: testMixedMessage}},
//			},
//			RecipientId: map[int64]map[id.Round]map[uint64]*MixedMessage{
//				recipientId.Int64(): {roundId: {testMixedMessage.Id: testMixedMessage}},
//			},
//			RoundIdCount: map[id.Round]uint64{roundId: 1},
//		},
//	}
//
//	err := m.InsertMixedMessages([]*MixedMessage{testMixedMessage})
//	if err == nil {
//		t.Errorf("Did not error when attempting to insert a mixedMessage that " +
//			"already exists.")
//	}
//}
//
//// Happy path
//func TestMapImpl_DeleteMixedMessageByRound(t *testing.T) {
//	testRoundId := id.Round(100)
//	testRoundId2 := id.Round(2)
//	testRecipientId := &ephemeral.Id{1, 2, 3}
//	m := &MapImpl{
//		mixedMessages: MixedMessageMap{
//			RoundId:      map[id.Round]map[int64]map[uint64]*MixedMessage{},
//			RecipientId:  map[int64]map[id.Round]map[uint64]*MixedMessage{},
//			RoundIdCount: map[id.Round]uint64{},
//		},
//	}
//
//	// Insert message not to be deleted
//	_ = m.InsertMixedMessages([]*MixedMessage{{
//		RoundId:     uint64(testRoundId2),
//		RecipientId: testRecipientId.Int64(),
//	}})
//
//	// Insert two messages to be deleted
//	_ = m.InsertMixedMessages([]*MixedMessage{
//		{
//			RoundId:     uint64(testRoundId),
//			RecipientId: testRecipientId.Int64(),
//		}, {
//			RoundId:     uint64(testRoundId),
//			RecipientId: testRecipientId.Int64(),
//		},
//	})
//
//	// Delete the two messages
//	err := m.DeleteMixedMessageByRound(testRoundId)
//	if err != nil {
//		t.Errorf("Unable to delete mixed messages by round: %+v", err)
//	}
//
//	// Ensure both messages were deleted
//	if m.mixedMessages.RoundId[testRoundId][testRecipientId.Int64()][1] != nil ||
//		m.mixedMessages.RecipientId[testRecipientId.Int64()][testRoundId][1] != nil {
//		t.Errorf("Expected to delete message with id %d from map", 1)
//	}
//	if m.mixedMessages.RoundId[testRoundId][testRecipientId.Int64()][2] != nil ||
//		m.mixedMessages.RecipientId[testRecipientId.Int64()][testRoundId][2] != nil {
//		t.Errorf("Expected to delete message with id %d from map", 2)
//	}
//
//	// Ensure other message remains
//	if m.mixedMessages.RoundId[testRoundId2][testRecipientId.Int64()][0] == nil ||
//		m.mixedMessages.RecipientId[testRecipientId.Int64()][testRoundId2][0] == nil {
//		t.Errorf("Incorrectly deleted message with id %d", 0)
//	}
//}

// Happy path.
func TestMapImpl_GetClientBloomFilters(t *testing.T) {
	// Build list of bloom filters to add
	ephemeralID, err := ephemeral.GetId(id.NewIdFromString("test", id.User, t), 16, uint64(time.Now().Unix()))
	if err != nil {
		t.Fatalf("Failed to get ephermeral ID: %+v", err)
	}
	rid := ephemeralID.Int64()
	filters := []*ClientBloomFilter{
		{RecipientId: rid, Epoch: 50},
		{RecipientId: rid, Epoch: 100},
		{RecipientId: rid, Epoch: 150},
		{RecipientId: rid, Epoch: 160},
		{RecipientId: rid, Epoch: 199},
		{RecipientId: rid, Epoch: 200},
		{RecipientId: rid, Epoch: 201},
		{RecipientId: rid, Epoch: 401},
	}

	// Initialize MapImpl with ClientBloomFilterList
	m := &MapImpl{
		bloomFilters: BloomFilterMap{
			RecipientId: map[int64]*ClientBloomFilterList{},
		},
	}

	for i, bf := range filters {
		if err := m.upsertClientBloomFilter(bf); err != nil {
			t.Errorf("Failed to insert BloomFilter (%d): %v", i, err)
		}
	}

	testVals := []struct {
		expected   []*ClientBloomFilter
		start, end uint32
	}{
		{filters[1:6], 100, 200},
		{filters[1:7], 75, 300},
		{filters[0:1], 25, 50},
		{filters[7:], 400, 600},
		{filters, 25, 600},
	}

	for i, val := range testVals {
		bloomFilters, err := m.GetClientBloomFilters(&ephemeralID, val.start, val.end)
		if err != nil {
			t.Errorf("Unexpected error retrieving bloom filters (%d): %v", i, err)
		}
		if !reflect.DeepEqual(val.expected, bloomFilters) {
			t.Errorf("Received unexpected bloom filter list (%d)."+
				"\nexpected: %+v\nreceived: %+v", i, val.expected, bloomFilters)
		}
	}
}

// Error Path: No matching bloom filters exist in the map.
func TestMapImpl_GetClientBloomFilters_NoFiltersError(t *testing.T) {
	// Build list of bloom filters to add
	ephemeralID, err := ephemeral.GetId(id.NewIdFromString("test", id.User, t), 16, uint64(time.Now().Unix()))
	if err != nil {
		t.Fatalf("Failed to get ephermeral ID: %+v", err)
	}
	rid := ephemeralID.Int64()
	filters := []*ClientBloomFilter{
		{RecipientId: rid, Epoch: 100},
		{RecipientId: rid, Epoch: 150},
		{RecipientId: rid, Epoch: 160},
		{RecipientId: rid, Epoch: 199},
		{RecipientId: rid, Epoch: 200},
		{RecipientId: rid, Epoch: 201},
		{RecipientId: rid, Epoch: 401},
		{RecipientId: rid, Epoch: 50},
	}

	// Initialize MapImpl with ClientBloomFilterList
	m := &MapImpl{
		bloomFilters: BloomFilterMap{
			RecipientId: map[int64]*ClientBloomFilterList{},
		},
	}

	for i, bf := range filters {
		if err := m.upsertClientBloomFilter(bf); err != nil {
			t.Errorf("Failed to insert BloomFilter (%d): %v", i, err)
		}
	}

	testVals := []struct {
		start, end uint32
	}{
		{10, 20},
		{402, 500},
		{110, 120},
	}

	for i, val := range testVals {
		bloomFilters, err := m.GetClientBloomFilters(&ephemeralID, val.start, val.end)
		if err == nil {
			t.Errorf("Expected an error when bloom filters is not in map (%d).", i)
		}
		if bloomFilters != nil {
			t.Errorf("Expected nil bloom filters returned (%d). Received: %v",
				i, bloomFilters)
		}
	}

	// Test with an ID not in the map
	bloomFilters, err := m.GetClientBloomFilters(&ephemeral.Id{}, 0, 1)
	if err == nil {
		t.Error("Expected an error when bloom filters is not in map.")
	}
	if bloomFilters != nil {
		t.Errorf("Expected nil bloom filters returned. Received: %v", bloomFilters)
	}
}

// Happy path.
func TestMapImpl_upsertClientBloomFilter(t *testing.T) {
	// Build list of bloom filters to add
	rid := rand.Int63()
	filters := []*ClientBloomFilter{
		{RecipientId: rid, Epoch: 100},
		{RecipientId: rid, Epoch: 150},
		{RecipientId: rid, Epoch: 160},
		{RecipientId: rid, Epoch: 199},
		{RecipientId: rid, Epoch: 200},
		{RecipientId: rid, Epoch: 201},
		{RecipientId: rid, Epoch: 401},
		{RecipientId: rid, Epoch: 401},
		{RecipientId: rid, Epoch: 50},
	}

	// Build expected bloom filter list
	expectedList := &ClientBloomFilterList{
		list:  make([]*ClientBloomFilter, 451),
		start: 50,
	}
	for _, bf := range filters {
		expectedList.list[bf.Epoch-expectedList.start] = bf
	}

	// Initialize MapImpl with ClientBloomFilterList
	m := &MapImpl{
		bloomFilters: BloomFilterMap{
			RecipientId: map[int64]*ClientBloomFilterList{},
		},
	}

	// Upsert test bloom filters and check for errors
	for i, bf := range filters {
		err := m.upsertClientBloomFilter(bf)
		if err != nil || m.bloomFilters.RecipientId[rid].list[bf.Epoch-m.bloomFilters.RecipientId[rid].start] == nil {
			t.Errorf("Failed to insert BloomFilter (%d): %v", i, err)
		}
	}

	if !reflect.DeepEqual(expectedList, m.bloomFilters.RecipientId[rid]) {
		t.Errorf("Created list does not match expected."+
			"\nexpected: %+v\nreceived: %+v",
			expectedList, m.bloomFilters.RecipientId[rid])
	}
}

// Happy path.
func TestMapImpl_DeleteClientFiltersBeforeEpoch(t *testing.T) {
	rid := []int64{rand.Int63(), rand.Int63(), rand.Int63(), rand.Int63()}
	filters := []*ClientBloomFilter{
		{RecipientId: rid[0], Epoch: 50},
		{RecipientId: rid[0], Epoch: 100},
		{RecipientId: rid[0], Epoch: 150},
		{RecipientId: rid[0], Epoch: 160},
		{RecipientId: rid[0], Epoch: 199},
		{RecipientId: rid[0], Epoch: 200},
		{RecipientId: rid[0], Epoch: 201},
		{RecipientId: rid[0], Epoch: 401},
		{RecipientId: rid[1], Epoch: 161},
		{RecipientId: rid[1], Epoch: 200},
		{RecipientId: rid[1], Epoch: 250},
		{RecipientId: rid[2], Epoch: 0},
		{RecipientId: rid[2], Epoch: 110},
		{RecipientId: rid[2], Epoch: 115},
		{RecipientId: rid[2], Epoch: 160},
		{RecipientId: rid[3], Epoch: 1},
	}

	// Initialize MapImpl with ClientBloomFilterList
	m := &MapImpl{
		bloomFilters: BloomFilterMap{
			RecipientId: map[int64]*ClientBloomFilterList{},
		},
	}

	// Upsert test bloom filters
	for i, bf := range filters {
		if err := m.upsertClientBloomFilter(bf); err != nil {
			t.Fatalf("Failed to insert BloomFilter (%d): %v", i, err)
		}
	}

	err := m.DeleteClientFiltersBeforeEpoch(160)
	if err != nil {
		t.Errorf("DeleteClientFiltersBeforeEpoch() produced an error: %+v", err)
	}

	// Get list of filters for the first ID
	var mapFilters []*ClientBloomFilter
	for _, bf := range m.bloomFilters.RecipientId[rid[0]].list {
		if bf != nil {
			mapFilters = append(mapFilters, bf)
		}
	}

	if !reflect.DeepEqual(filters[4:8], mapFilters) {
		t.Errorf("DeleteClientFiltersBeforeEpoch() did not delete the expected "+
			"bloom filters for ID %d.\nexpected: %+v\nreceived: %+v",
			rid[0], filters[4:8], mapFilters)
	}

	// Get list of filters for the second ID
	mapFilters = []*ClientBloomFilter{}
	for _, bf := range m.bloomFilters.RecipientId[rid[1]].list {
		if bf != nil {
			mapFilters = append(mapFilters, bf)
		}
	}

	if !reflect.DeepEqual(filters[8:11], mapFilters) {
		t.Errorf("DeleteClientFiltersBeforeEpoch() did not delete the expected "+
			"bloom filters for ID %d.\nexpected: %+v\nreceived: %+v",
			rid[1], filters[8:11], mapFilters)
	}

	// Get list of filters for the third ID
	mapFilters = []*ClientBloomFilter{}
	for _, bf := range m.bloomFilters.RecipientId[rid[2]].list {
		if bf != nil {
			mapFilters = append(mapFilters, bf)
		}
	}

	if !reflect.DeepEqual([]*ClientBloomFilter{}, mapFilters) {
		t.Errorf("DeleteClientFiltersBeforeEpoch() did not delete the expected "+
			"bloom filters for ID %d.\nexpected: %+v\nreceived: %+v",
			rid[2], []*ClientBloomFilter{}, mapFilters)
	}

	if m.bloomFilters.RecipientId[rid[3]] != nil {
		t.Errorf("DeleteClientFiltersBeforeEpoch() did not delete the list for ID "+
			"%d when all stores epochs occured before the given epoch.\nlist: %+v",
			rid[2], m.bloomFilters.RecipientId[rid[3]])
	}
}

// Error path: no bloom filters with epochs before the given one exist.
func TestMapImpl_DeleteClientFiltersBeforeEpoch_NoFiltersError(t *testing.T) {
	rid := rand.Int63()
	filters := []*ClientBloomFilter{
		{RecipientId: rid, Epoch: 50},
		{RecipientId: rid, Epoch: 100},
		{RecipientId: rid, Epoch: 150},
		{RecipientId: rid, Epoch: 160},
		{RecipientId: rid, Epoch: 199},
		{RecipientId: rid, Epoch: 200},
		{RecipientId: rid, Epoch: 201},
		{RecipientId: rid, Epoch: 250},
		{RecipientId: rid, Epoch: 401},
	}

	// Initialize MapImpl with ClientBloomFilterList
	m := &MapImpl{
		bloomFilters: BloomFilterMap{
			RecipientId: map[int64]*ClientBloomFilterList{},
		},
	}

	// Upsert test bloom filters
	for i, bf := range filters {
		if err := m.upsertClientBloomFilter(bf); err != nil {
			t.Fatalf("Failed to insert BloomFilter (%d): %v", i, err)
		}
	}

	err := m.DeleteClientFiltersBeforeEpoch(5)
	if err == nil {
		t.Error("DeleteClientFiltersBeforeEpoch() did not produced an error when no " +
			"filters should have been deleted.")
	}

	// Get list of filters for the first ID
	var mapFilters []*ClientBloomFilter
	for _, bf := range m.bloomFilters.RecipientId[rid].list {
		if bf != nil {
			mapFilters = append(mapFilters, bf)
		}
	}

	if !reflect.DeepEqual(filters, mapFilters) {
		t.Errorf("DeleteClientFiltersBeforeEpoch() did not delete the expected "+
			"bloom filters for ID %d.\nexpected: %+v\nreceived: %+v",
			rid, filters, mapFilters)
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
