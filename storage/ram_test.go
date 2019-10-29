////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/primitives/id"
	"os"
	"strings"
	"testing"
	"time"
)

var messageBuf *MapBuffer

func TestMain(m *testing.M) {
	messageBuf = &MapBuffer{
		messageCollection: make(map[id.User]map[string]*pb.Slot),
		messageIDs:        make(map[id.User][]string),
	}
	os.Exit(m.Run())
}

func TestMapBuffer_StartMessageCleanup(t *testing.T) {
	userId := id.NewUserFromUint(520, t)
	// Use a separate buffer to not interfere with other tests
	cleanupBuf := &MapBuffer{
		messageCollection: make(map[id.User]map[string]*pb.Slot),
		messageIDs:        make(map[id.User][]string),
	}

	// Add a few messages to the buffer
	cleanupBuf.messageCollection[*userId] = make(map[string]*pb.Slot)
	cleanupBuf.messageIDs[*userId] = make([]string, 0)
	for i := 0; i < 5; i++ {
		msgId := string(i)
		cleanupBuf.messageCollection[*userId][msgId] = &pb.Slot{}
		cleanupBuf.messageIDs[*userId] = append(cleanupBuf.messageIDs[*userId],
			msgId)
	}

	// Start the cleanup
	go cleanupBuf.StartMessageCleanup(1)
	time.Sleep(2 * time.Second)

	// Make sure the messages no longer exist after
	numMsgs := len(cleanupBuf.messageCollection[*userId])
	if numMsgs > 0 {
		t.Errorf("StartMessageCleanup: Expected all messages to be cleared, "+
			"%d messages still in buffer!", numMsgs)
	}
}

func TestMapBuffer_GetMessage(t *testing.T) {
	userId := id.ZeroID
	msgId := "msg1"
	messageBuf.messageCollection[*userId] = make(map[string]*pb.Slot)
	messageBuf.messageCollection[*userId][msgId] = &pb.Slot{
		SenderID: userId.Bytes(),
	}
	_, ok := messageBuf.GetMixedMessage(userId, msgId)
	if !ok {
		t.Errorf("GetMixedMessage: Unable to find message!")
	}
}

func TestMapBuffer_GetMessageIDs(t *testing.T) {
	userId := id.NewUserFromUint(5, t)
	msgId := "msg1"
	messageBuf.messageCollection[*userId] = make(map[string]*pb.Slot)
	messageBuf.messageCollection[*userId][msgId] = &pb.Slot{
		SenderID: userId.Bytes(),
	}
	messageBuf.messageIDs[*userId] = make([]string, 1)
	messageBuf.messageIDs[*userId][0] = msgId
	msgIds, ok := messageBuf.GetMixedMessageIDs(userId, "")
	if len(msgIds) < 1 || !ok {
		t.Errorf("GetMixedMessageIDs: Unable to get any message IDs!")
	}
	msgIds, ok = messageBuf.GetMixedMessageIDs(userId, "msg1")
	if len(msgIds) != 0 || !ok {
		t.Errorf("GetMixedMessageIDs: Could not get message IDs after 'msg1'")
	}
	//Test that it returns every msg after a found id
	msgId2 := "msg2"
	messageBuf.messageIDs[*userId] = make([]string, 2)
	messageBuf.messageIDs[*userId][0] = msgId
	messageBuf.messageIDs[*userId][1] = msgId2
	msgIds, ok = messageBuf.GetMixedMessageIDs(userId, "msg1")
	if len(msgIds) == 0 || !ok || strings.Compare(msgIds[0], msgId2) != 0 {
		t.Errorf("GetMixedMessageIDs: Could not get message IDs after 'msg1'")
	}

}

func TestMapBuffer_DeleteMessage(t *testing.T) {
	userId := id.NewUserFromUint(555, t)
	msgId := "msg1"
	messageBuf.messageCollection[*userId] = make(map[string]*pb.Slot)
	messageBuf.messageIDs[*userId] = make([]string, 0)
	messageBuf.messageIDs[*userId] = append(messageBuf.messageIDs[*userId], "msgId")
	messageBuf.messageCollection[*userId][msgId] = &pb.Slot{
		SenderID: userId.Bytes(),
	}
	messageBuf.DeleteMixedMessage(userId, msgId)
	_, ok := messageBuf.messageCollection[*userId][msgId]
	if ok {
		t.Errorf("DeleteMixedMessage: Message was not deleted properly!")
	}
}

func TestMapBuffer_AddMessage(t *testing.T) {
	userId := id.NewUserFromUint(10, t)
	msgId := "msg1"
	messageBuf.AddMixedMessage(userId, msgId, &pb.Slot{SenderID: userId.Bytes()})
	_, ok := messageBuf.messageCollection[*userId][msgId]
	if !ok {
		t.Errorf("AddMixedMessage: Message was not added to message buffer" +
			" properly!")
	}
}

func TestMapBuffer_AddUnmixedMessage(t *testing.T) {
	numOutgoingMsgs := len(messageBuf.outgoingMessages.Slots)
	messageBuf.AddUnmixedMessage(&pb.Slot{SenderID: id.ZeroID.Bytes()})
	if len(messageBuf.outgoingMessages.Slots) != numOutgoingMsgs+1 {
		t.Errorf("AddUnMixedMessage: Message was not added to outgoing" +
			" message buffer properly!")
	}
}

func TestMapBuffer_PopUnmixedMessages(t *testing.T) {
	messageBuf.outgoingMessages.Slots = make([]*pb.Slot, 0)
	if messageBuf.LenUnmixed() != 0 {
		t.Errorf("PopUnmixedMessages: Queue should be empty! Has %d messages!",
			messageBuf.LenUnmixed())
	}
	if len(messageBuf.PopUnmixedMessages(1, 1).Slots) != 0 {
		t.Errorf("PopUnmixedMessages: Should have returned empty batch")
	}
	messageBuf.outgoingMessages.Slots = append(messageBuf.outgoingMessages.Slots,
		&pb.Slot{SenderID: id.ZeroID.Bytes()})
	//First confirm there's a message present
	if messageBuf.LenUnmixed() != 1 {
		t.Errorf("PopUnmixedMessages: Queue should have 1 message!")
	}
	messageBuf.PopUnmixedMessages(1, 1)
	if len(messageBuf.outgoingMessages.Slots) > 0 {
		t.Errorf("PopUnmixedMessages: Batch was not popped correctly!")
	}
	//Test that if minCount is greater than the amount of messages, the batch returned is nil
	messageBuf.outgoingMessages.Slots = append(messageBuf.outgoingMessages.Slots,
		&pb.Slot{SenderID: id.ZeroID.Bytes()})
	batch := messageBuf.PopUnmixedMessages(4, 1)
	if batch != nil {
		t.Errorf("Error case of minCount being greater than the amount of messages, "+
			"should recieved a nil batch but recieved: %v", batch)
	}
	//Test when the outgoing message is overfull
	messageBuf.outgoingMessages.Slots = append(messageBuf.outgoingMessages.Slots,
		&pb.Slot{SenderID: id.ZeroID.Bytes()})
	messageBuf.PopUnmixedMessages(1, 1)
}

func TestMapBuffer_ExceedUserMsgsLimit(t *testing.T) {
	userId := id.NewUserFromUint(10, t)
	msgIDFmt := "msg1"

	deleteMe := messageBuf.messageIDs[*userId]
	for i := range deleteMe {
		messageBuf.DeleteMixedMessage(userId, deleteMe[i])
	}

	for i := 0; i < MaxUserMessagesLimit; i++ {
		msgID := msgIDFmt + string(i)
		messageBuf.AddMixedMessage(userId, msgID,
			&pb.Slot{SenderID: userId.Bytes()})
	}

	if len(messageBuf.messageIDs[*userId]) != MaxUserMessagesLimit {
		t.Errorf("Message limit not exceeded, but length incorrect: %d v. %d",
			len(messageBuf.messageIDs[*userId]), MaxUserMessagesLimit)
	}

	msgID := msgIDFmt + "Hello"
	messageBuf.AddMixedMessage(userId, msgID,
		&pb.Slot{SenderID: userId.Bytes()})

	if len(messageBuf.messageIDs[*userId]) != MaxUserMessagesLimit {
		t.Errorf("Message limit exceeded, but length incorrect: %d v. %d",
			len(messageBuf.messageIDs[*userId]), MaxUserMessagesLimit)
	}

	_, ok := messageBuf.messageCollection[*userId][msgID]
	if !ok {
		t.Errorf("AddMixedMessage: Message was not added to message buffer" +
			" properly!")
	}
}
