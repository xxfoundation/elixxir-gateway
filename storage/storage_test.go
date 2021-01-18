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
