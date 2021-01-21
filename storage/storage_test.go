////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"math/rand"
	"testing"
)

// Insert path
func TestStorage_HandleBloomFilter_Insert(t *testing.T) {
	// TODO: requires mapimpl
}

// Update path
func TestStorage_HandleBloomFilter_Update(t *testing.T) {
	// TODO: requires mapimpl
}

//
func TestOr(t *testing.T) {
	// TODO
}

//
func TestClientBloomFilter_Combine(t *testing.T) {
	// TODO
}

// Happy path
func TestStorage_GetMixedMessages(t *testing.T) {
	testMsgID := rand.Uint64()
	testRoundID := id.Round(rand.Uint64())
	testRecipientID := &ephemeral.Id{1, 2, 3}
	testMixedMessage := &MixedMessage{
		Id:          testMsgID,
		RoundId:     uint64(testRoundID),
		RecipientId: testRecipientID.Int64(),
	}
	storage := &Storage{
		&MapImpl{
			mixedMessages: MixedMessageMap{
				RoundId: map[id.Round]map[int64]map[uint64]*MixedMessage{
					testRoundID: {testRecipientID.Int64(): {testMsgID: testMixedMessage}},
				},
				RecipientId: map[int64]map[id.Round]map[uint64]*MixedMessage{
					testRecipientID.Int64(): {testRoundID: {testMsgID: testMixedMessage}},
				},
				RoundIdCount: map[id.Round]uint64{testRoundID: 1},
			},
		},
	}

	msgs, isValidGateway, err := storage.GetMixedMessages(testRecipientID, testRoundID)
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
	testRecipientID := &ephemeral.Id{1, 2, 3}

	storage := &Storage{
		&MapImpl{
			mixedMessages: MixedMessageMap{
				RoundId:     map[id.Round]map[int64]map[uint64]*MixedMessage{},
				RecipientId: map[int64]map[id.Round]map[uint64]*MixedMessage{},
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
