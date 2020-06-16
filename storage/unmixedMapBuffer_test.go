///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package storage

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/primitives/id"
	"testing"
)

// tests that unmixed messages are properly added to the unmixed buffer
func TestUnmixedMapBuffer_AddUnmixedMessage(t *testing.T) {
	unmixedMessageBuf := &UnmixedMapBuffer{}
	numOutgoingMsgs := len(unmixedMessageBuf.outgoingMessages.Slots)

	unmixedMessageBuf.AddUnmixedMessage(&pb.Slot{SenderID: id.ZeroUser.Marshal()})

	if len(unmixedMessageBuf.outgoingMessages.Slots) != numOutgoingMsgs+1 {
		t.Errorf("AddUnMixedMessage: Message was not added to outgoing" +
			" message buffer properly!")
	}
}

// tests that removing messages from unmixed buffer works correctly
func TestUnmixedMapBuffer_PopUnmixedMessages(t *testing.T) {
	unmixedMessageBuf := &UnmixedMapBuffer{}
	unmixedMessageBuf.outgoingMessages.Slots = make([]*pb.Slot, 0)

	if unmixedMessageBuf.LenUnmixed() != 0 {
		t.Errorf("PopUnmixedMessages: Queue should be empty! Has %d messages!",
			unmixedMessageBuf.LenUnmixed())
	}

	if len(unmixedMessageBuf.PopUnmixedMessages(1, 1).Slots) != 0 {
		t.Errorf("PopUnmixedMessages: Should have returned empty batch")
	}

	unmixedMessageBuf.outgoingMessages.Slots = append(unmixedMessageBuf.outgoingMessages.Slots,
		&pb.Slot{SenderID: id.ZeroUser.Marshal()})

	// First confirm there is a message present
	if unmixedMessageBuf.LenUnmixed() != 1 {
		t.Errorf("PopUnmixedMessages: Queue should have 1 message!")
	}

	unmixedMessageBuf.PopUnmixedMessages(1, 1)

	if len(unmixedMessageBuf.outgoingMessages.Slots) > 0 {
		t.Errorf("PopUnmixedMessages: Batch was not popped correctly!")
	}

	// Test that if minCount is greater than the amount of messages, then the
	// batch that is returned is nil
	unmixedMessageBuf.outgoingMessages.Slots = append(unmixedMessageBuf.outgoingMessages.Slots,
		&pb.Slot{SenderID: id.ZeroUser.Marshal()})

	batch := unmixedMessageBuf.PopUnmixedMessages(4, 1)

	if batch != nil {
		t.Errorf("Error case of minCount being greater than the amount of"+
			"messages, should recieved a nil batch but recieved: %v", batch)
	}

	// Test when the outgoing message is overfull
	unmixedMessageBuf.outgoingMessages.Slots = append(
		unmixedMessageBuf.outgoingMessages.Slots,
		&pb.Slot{SenderID: id.ZeroUser.Marshal()},
	)

	unmixedMessageBuf.PopUnmixedMessages(1, 1)
}
