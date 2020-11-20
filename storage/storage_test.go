////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	"gitlab.com/xx_network/primitives/id"
	"math/rand"
	"testing"
)

// Happy path
func TestStorage_GetBloomFilters(t *testing.T) {
	testEpochId := uint64(0)
	testEpochId2 := uint64(1)
	testRoundId := uint64(0)
	testRoundId2 := uint64(100)
	testRecipientID := *id.NewIdFromUInt(rand.Uint64(), id.User, t)
	storage := &Storage{
		&MapImpl{
			epochs: EpochMap{
				M: map[uint64]*Epoch{testEpochId: {
					Id:      testEpochId,
					RoundId: testRoundId,
				},
					testEpochId2: {
						Id:      testEpochId2,
						RoundId: testRoundId2,
					}},
				IdTrack: testEpochId2 + 1,
			},
			bloomFilters: BloomFilterMap{
				RecipientId: map[id.ID]map[uint64]*BloomFilter{
					testRecipientID: {testEpochId: {RecipientId: testRecipientID.Marshal(), EpochId: testEpochId},
						testEpochId2: {RecipientId: testRecipientID.Marshal(), EpochId: testEpochId2},
					},
				},
				EpochId: map[uint64]map[id.ID]*BloomFilter{
					testEpochId:  {testRecipientID: {RecipientId: testRecipientID.Marshal(), EpochId: testEpochId}},
					testEpochId2: {testRecipientID: {RecipientId: testRecipientID.Marshal(), EpochId: testEpochId2}},
				},
			},
		},
	}

	latestRound := id.Round(100)
	results, err := storage.GetBloomFilters(&testRecipientID, latestRound)
	if err != nil {
		t.Errorf(err.Error())
	}
	if len(results) != 2 {
		t.Errorf("Returned unexpected number of results: %d", len(results))
		return
	}
	/*
	if results[0].LastRound != id.Round(testRoundId2-1) {
		t.Errorf("Got unexpected LastRound value."+
			"\n\tExpected: %d"+
			"\n\tReceived: %d", id.Round(testRoundId2-1), results[0].LastRound)
	}*/
	if results[1].LastRound != latestRound {
		t.Errorf("Got unexpected LastRound value."+
			"\n\tExpected: %d"+
			"\n\tReceived: %d", latestRound, results[1].LastRound)
	}
}

// Happy path
func TestStorage_GetMixedMessages(t *testing.T) {
	testMsgID := rand.Uint64()
	testRoundID := id.Round(rand.Uint64())
	testRecipientID := *id.NewIdFromUInt(rand.Uint64(), id.User, t)
	testMixedMessage := &MixedMessage{
		Id:          testMsgID,
		RoundId:     uint64(testRoundID),
		RecipientId: testRecipientID.Marshal(),
	}
	storage := &Storage{
		&MapImpl{
			mixedMessages: MixedMessageMap{
				RoundId:      map[id.Round]map[id.ID]map[uint64]*MixedMessage{testRoundID: {testRecipientID: {testMsgID: testMixedMessage}}},
				RecipientId:  map[id.ID]map[id.Round]map[uint64]*MixedMessage{testRecipientID: {testRoundID: {testMsgID: testMixedMessage}}},
				RoundIdCount: map[id.Round]uint64{testRoundID: 1},
			},
		},
	}

	msgs, isValidGateway, err := storage.GetMixedMessages(&testRecipientID, testRoundID)
	if len(msgs) != 1 {
		t.Errorf("Retrieved unexpected number of messages: %d", len(msgs))
	}
	if !isValidGateway {
		t.Errorf("Expected valid gateway!")
	}
	if err != nil {
		t.Errorf(err.Error())
	}
}

// Invalid gateway path
func TestStorage_GetMixedMessagesInvalidGw(t *testing.T) {
	testRoundID := id.Round(rand.Uint64())
	testRecipientID := id.NewIdFromUInt(rand.Uint64(), id.User, t)

	storage := &Storage{
		&MapImpl{
			mixedMessages: MixedMessageMap{
				RoundId:     map[id.Round]map[id.ID]map[uint64]*MixedMessage{},
				RecipientId: map[id.ID]map[id.Round]map[uint64]*MixedMessage{},
			},
		},
	}

	msgs, isValidGateway, err := storage.GetMixedMessages(testRecipientID, testRoundID)
	if len(msgs) != 0 {
		t.Errorf("Retrieved unexpected number of messages: %d", len(msgs))
	}
	if isValidGateway {
		t.Errorf("Expected invalid gateway!")
	}
	if err != nil {
		t.Errorf(err.Error())
	}
}
