///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package storage

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	id "gitlab.com/xx_network/primitives/id"
	"testing"
)

// tests that unmixed messages are properly added to the unmixed buffer
func TestUnmixedMapBuffer_AddUnmixedMessage(t *testing.T) {
	testMap := make(map[id.Round]*SendRound)
	unmixedMessageBuf := &UnmixedMessagesMap{
		messages: testMap,
	}

	numOutgoingMsgs := len(unmixedMessageBuf.messages)
	unmixedMessageBuf.SetAsRoundLeader(id.Round(0), 5)
	unmixedMessageBuf.AddUnmixedMessage(&pb.Slot{SenderID: id.ZeroUser.Marshal()}, id.Round(0))

	if len(unmixedMessageBuf.messages) != numOutgoingMsgs+1 {
		t.Errorf("AddUnMixedMessage: Message was not added to outgoing" +
			" message buffer properly!")
	}
}

// tests that removing messages from unmixed buffer works correctly
func TestUnmixedMapBuffer_GetUnmixedMessages(t *testing.T) {
	unmixedMessageBuf := NewUnmixedMessagesMap()

	if unmixedMessageBuf.LenUnmixed(id.Round(0)) != 0 {
		t.Errorf("GetRoundMessages: Queue should be empty! Has %d messages!",
			unmixedMessageBuf.LenUnmixed(id.Round(0)))
	}

	if unmixedMessageBuf.PopRound(0) != nil {
		t.Errorf("GetRoundMessages: Should have returned empty batch")
	}
	testSlot := &pb.Slot{SenderID: id.ZeroUser.Marshal()}

	unmixedMessageBuf.SetAsRoundLeader(id.Round(0), 4)

	unmixedMessageBuf.AddUnmixedMessage(testSlot, id.Round(0))

	// First confirm there is a message present
	if unmixedMessageBuf.LenUnmixed(0) != 1 {
		t.Errorf("GetRoundMessages: Queue should have 1 message!")
	}

	unmixedMessageBuf.PopRound(0)

	// Test that if minCount is greater than the amount of messages, then the
	// batch that is returned is nil
	unmixedMessageBuf.AddUnmixedMessage(testSlot, id.Round(0))

	batch := unmixedMessageBuf.PopRound(0)

	if batch != nil {
		t.Errorf("Error case of minCount being greater than the amount of"+
			"messages, should received a nil batch but received: %v", batch)
	}

}

// Happy path
func TestUnmixedMessagesMap_IsRoundFull(t *testing.T) {
	unmixedMessageBuf := NewUnmixedMessagesMap()
	rndId := id.Round(4)
	batchSize := 3
	unmixedMessageBuf.SetAsRoundLeader(rndId, uint32(batchSize))

	for i := 0; i < batchSize; i++ {
		unmixedMessageBuf.AddUnmixedMessage(&pb.Slot{}, rndId)
	}

	if !unmixedMessageBuf.IsRoundFull(rndId) {
		t.Errorf("Message buffer for round %d should be full."+
			"\n\tExpected messages: %d"+
			"\n\tReceived messaged: %d", rndId, batchSize, unmixedMessageBuf.LenUnmixed(rndId))
	}
}

// Unit test
func TestUnmixedMessagesMap_IsRoundLeader(t *testing.T) {
	unmixedMessageBuf := NewUnmixedMessagesMap()
	rndId := id.Round(4)
	batchSize := 3

	if unmixedMessageBuf.IsRoundLeader(rndId) {
		t.Errorf("Marked as a round leader incorrectly. Should only return true" +
			"after a call to SetAsRoundLeader")
	}

	unmixedMessageBuf.SetAsRoundLeader(rndId, uint32(batchSize))
	if !unmixedMessageBuf.IsRoundLeader(rndId) {
		t.Errorf("Should be marked as a round leader for round %d", rndId)
	}

}

// Unit test
func TestUnmixedMessagesMap_AddManyUnmixedMessages(t *testing.T) {
	testMap := make(map[id.Round]*SendRound)
	unmixedMessageBuf := &UnmixedMessagesMap{
		messages: testMap,
	}
	maxSlots := 5
	unmixedMessageBuf.SetAsRoundLeader(id.Round(0), uint32(maxSlots))

	// Insert slots up to a full batch
	slots := make([]*pb.GatewaySlot, 0)
	for i := 0; i < maxSlots-1; i++ {
		slot := &pb.GatewaySlot{
			Message: &pb.Slot{SenderID: id.ZeroUser.Marshal()},
		}
		slots = append(slots, slot)
	}
	rnd := id.Round(0)
	err := unmixedMessageBuf.AddManyUnmixedMessages(slots, rnd)
	if err != nil {
		t.Fatalf("AddManyUnmixedMessages error: %v", err)
	}

	// Construct an extra slot and attempt to insert
	slot := &pb.GatewaySlot{
		Message: &pb.Slot{SenderID: id.ZeroUser.Marshal()},
	}
	extraSlots := []*pb.GatewaySlot{slot}
	err = unmixedMessageBuf.AddManyUnmixedMessages(extraSlots, rnd)
	if err == nil {
		t.Fatalf("AddManyUnmixedMessages error: " +
			"Should not be able to insert into already full batch")
	}

}
