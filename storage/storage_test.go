////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	"bytes"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"math/rand"
	"testing"
)

// Happy path
func TestOr(t *testing.T) {
	l1 := []byte{65, 0, 0, 0, 0, 172}
	l2 := []byte{72, 66, 67, 226, 130, 1}
	expected := []byte{73, 66, 67, 226, 130, 173}

	result := or(l1, l2)
	if !bytes.Equal(result, expected) {
		t.Errorf("Invalid Or Return 1: %v", result)
	}
}

// Nil paths
func TestOr_Nil(t *testing.T) {
	var l1 []byte
	var l2 []byte

	result := or(l1, l2)
	if result != nil {
		t.Errorf("Invalid Nil Or Return 1: %v", result)
	}

	l1 = []byte("test")
	result = or(l1, l2)
	if !bytes.Equal(l1, result) {
		t.Errorf("Invalid Nil Or Return 2: %v", result)
	}

	l1 = []byte("test")
	result = or(l2, l1)
	if !bytes.Equal(l1, result) {
		t.Errorf("Invalid Nil Or Return 3: %v", result)
	}
}

// Unequal length path
func TestOr_Length(t *testing.T) {
	l1 := []byte("CHUNGUS")
	l2 := []byte("no")

	result := or(l1, l2)
	if !bytes.Equal(l1, result) {
		t.Errorf("Invalid Len Or Return 1: %v", result)
	}

	result = or(l2, l1)
	if !bytes.Equal(l2, result) {
		t.Errorf("Invalid Len Or Return 2: %v", result)
	}
}

// Happy path - New filter
func TestClientBloomFilter_Combine_New(t *testing.T) {
	testFilter := []byte("test")
	oldFilter := &ClientBloomFilter{
		RecipientId: 0,
		Epoch:       0,
		FirstRound:  0,
		RoundRange:  0,
		Filter:      testFilter,
	}
	newFilter := &ClientBloomFilter{
		RecipientId: 10,
		Epoch:       10,
		FirstRound:  10,
		RoundRange:  0,
	}

	newFilter.combine(oldFilter)

	// Ensure some things did not change
	if newFilter.RecipientId != 10 {
		t.Errorf("Unexpected recipient change: %d", newFilter.RecipientId)
	}
	if newFilter.Epoch != 10 {
		t.Errorf("Unexpected epoch change: %d", newFilter.Epoch)
	}
	if newFilter.RoundRange != 0 {
		t.Errorf("Unexpected RoundRange value: %d", newFilter.RoundRange)
	}

	// Ensure some things did change
	if oldFilter.FirstRound != 10 {
		t.Errorf("Expected FirstRound change: %d", oldFilter.FirstRound)
	}
	if !bytes.Equal(newFilter.Filter, testFilter) {
		t.Errorf("Unexpected Filter value: %v", newFilter.Filter)
	}
}

// Happy path - Update filter
func TestClientBloomFilter_Combine_Update(t *testing.T) {
	testFilter := []byte("test")
	oldFilter := &ClientBloomFilter{
		RecipientId: 10,
		Epoch:       10,
		FirstRound:  10,
		RoundRange:  0,
		Filter:      testFilter,
	}
	newFilter := &ClientBloomFilter{
		RecipientId: 10,
		Epoch:       10,
		FirstRound:  20,
		RoundRange:  0,
		Filter:      testFilter,
	}

	newFilter.combine(oldFilter)

	// Ensure some things didn't change
	if oldFilter.FirstRound != 10 {
		t.Errorf("Unexpected FirstRound change: %d", oldFilter.FirstRound)
	}
	if !bytes.Equal(newFilter.Filter, testFilter) {
		t.Errorf("Unexpected Filter value: %v", newFilter.Filter)
	}

	// Ensure some things did change
	if newFilter.FirstRound != oldFilter.FirstRound {
		t.Errorf("Expected FirstRound change: %d", newFilter.FirstRound)
	}
	if newFilter.RoundRange != 10 {
		t.Errorf("Expected RoundRange change: %d", newFilter.RoundRange)
	}
}

// Happy path - Update filter with newer oldFilter
func TestClientBloomFilter_Combine_UpdateOld(t *testing.T) {
	testFilter := []byte("test")
	oldFilter := &ClientBloomFilter{
		RecipientId: 10,
		Epoch:       10,
		FirstRound:  10,
		RoundRange:  50,
		Filter:      testFilter,
	}
	newFilter := &ClientBloomFilter{
		RecipientId: 10,
		Epoch:       10,
		FirstRound:  20,
		RoundRange:  0,
		Filter:      testFilter,
	}

	newFilter.combine(oldFilter)

	// Ensure some things didn't change
	if oldFilter.FirstRound != 10 {
		t.Errorf("Unexpected FirstRound change: %d", oldFilter.FirstRound)
	}
	if !bytes.Equal(newFilter.Filter, testFilter) {
		t.Errorf("Unexpected Filter value: %v", newFilter.Filter)
	}

	// Ensure some things did change
	if newFilter.FirstRound != oldFilter.FirstRound {
		t.Errorf("Expected FirstRound change: %d", newFilter.FirstRound)
	}
	if newFilter.RoundRange != oldFilter.RoundRange {
		t.Errorf("Expected RoundRange change: %d", newFilter.RoundRange)
	}
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
