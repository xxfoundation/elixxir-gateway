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
	"testing"
	"time"
)

var messageBuf *MapBuffer

func TestMain(m *testing.M) {
	messageBuf = &MapBuffer{
		messageCollection: make(map[id.User]map[string]*pb.CmixMessage),
		messageIDs:        make(map[id.User][]string),
		outgoingMessages:  make([]*pb.CmixMessage, 0),
	}
	os.Exit(m.Run())
}

func TestMapBuffer_StartMessageCleanup(t *testing.T) {
	userId := id.NewUserFromUint(520, t)
	// Use a separate buffer to not interfere with other tests
	cleanupBuf := &MapBuffer{
		messageCollection: make(map[id.User]map[string]*pb.CmixMessage),
		messageIDs:        make(map[id.User][]string),
		outgoingMessages:  make([]*pb.CmixMessage, 0),
	}

	// Add a few messages to the buffer
	cleanupBuf.messageCollection[*userId] = make(map[string]*pb.CmixMessage)
	cleanupBuf.messageIDs[*userId] = make([]string, 0)
	for i := 0; i < 5; i++ {
		msgId := string(i)
		cleanupBuf.messageCollection[*userId][msgId] = &pb.CmixMessage{}
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
	messageBuf.messageCollection[*userId] = make(map[string]*pb.CmixMessage)
	messageBuf.messageCollection[*userId][msgId] = &pb.
		CmixMessage{SenderID: userId.Bytes()}
	_, ok := messageBuf.GetMessage(userId, msgId)
	if !ok {
		t.Errorf("GetMessage: Unable to find message!")
	}
}

func TestMapBuffer_GetMessageIDs(t *testing.T) {
	userId := id.NewUserFromUint(5, t)
	msgId := "msg1"
	messageBuf.messageCollection[*userId] = make(map[string]*pb.CmixMessage)
	messageBuf.messageCollection[*userId][msgId] = &pb.
		CmixMessage{SenderID: userId.Bytes()}
	messageBuf.messageIDs[*userId] = make([]string, 1)
	messageBuf.messageIDs[*userId][0] = msgId
	msgIds, ok := messageBuf.GetMessageIDs(userId, "")
	if len(msgIds) < 1 || !ok {
		t.Errorf("GetMessageIDs: Unable to get any message IDs!")
	}
	msgIds, ok = messageBuf.GetMessageIDs(userId, "msg1")
	if len(msgIds) != 0 || !ok {
		t.Errorf("GetMessageIDs: Could not get message IDs after 'msg1'")
	}
}

func TestMapBuffer_DeleteMessage(t *testing.T) {
	userId := id.NewUserFromUint(555, t)
	msgId := "msg1"
	messageBuf.messageCollection[*userId] = make(map[string]*pb.CmixMessage)
	messageBuf.messageIDs[*userId] = make([]string, 0)
	messageBuf.messageIDs[*userId] = append(messageBuf.messageIDs[*userId], "msgId")
	messageBuf.messageCollection[*userId][msgId] = &pb.
		CmixMessage{SenderID: userId.Bytes()}
	messageBuf.DeleteMessage(userId, msgId)
	_, ok := messageBuf.messageCollection[*userId][msgId]
	if ok {
		t.Errorf("DeleteMessage: Message was not deleted properly!")
	}
}

func TestMapBuffer_AddMessage(t *testing.T) {
	userId := id.NewUserFromUint(10, t)
	msgId := "msg1"
	messageBuf.AddMessage(userId, msgId,
		&pb.CmixMessage{SenderID: userId.Bytes()})
	_, ok := messageBuf.messageCollection[*userId][msgId]
	if !ok {
		t.Errorf("AddMessage: Message was not added to message buffer" +
			" properly!")
	}
}

func TestMapBuffer_AddOutgoingMessage(t *testing.T) {
	numOutgoingMsgs := len(messageBuf.outgoingMessages)
	messageBuf.AddOutgoingMessage(&pb.CmixMessage{SenderID: id.ZeroID.Bytes()})
	if len(messageBuf.outgoingMessages) != numOutgoingMsgs+1 {
		t.Errorf("AddOutgoingMessage: Message was not added to outgoing" +
			" message buffer properly!")
	}
}

func TestMapBuffer_PopOutgoingBatch(t *testing.T) {
	messageBuf.outgoingMessages = append(messageBuf.outgoingMessages,
		&pb.CmixMessage{SenderID: id.ZeroID.Bytes()})
	messageBuf.PopOutgoingBatch(1)
	if len(messageBuf.outgoingMessages) > 0 {
		t.Errorf("PopOutgoingBatch: Batch was not popped correctly!")
	}
}

func TestMapBuffer_ExceedUserMsgsLimit(t *testing.T) {
	userId := id.NewUserFromUint(10, t)
	msgIDFmt := "msg1"

	deleteme := messageBuf.messageIDs[*userId]
	for i := range deleteme {
		messageBuf.DeleteMessage(userId, deleteme[i])
	}

	for i := 0; i < MaxUserMessagesLimit; i++ {
		msgID := msgIDFmt + string(i)
		messageBuf.AddMessage(userId, msgID,
			&pb.CmixMessage{SenderID: userId.Bytes()})
	}

	if len(messageBuf.messageIDs[*userId]) != MaxUserMessagesLimit {
		t.Errorf("Message limit not exceeded, but length incorrect: %d v. %d",
			len(messageBuf.messageIDs[*userId]), MaxUserMessagesLimit)
	}

	msgID := msgIDFmt + "Hello"
	messageBuf.AddMessage(userId, msgID,
		&pb.CmixMessage{SenderID: userId.Bytes()})

	if len(messageBuf.messageIDs[*userId]) != MaxUserMessagesLimit {
		t.Errorf("Message limit exceeded, but length incorrect: %d v. %d",
			len(messageBuf.messageIDs[*userId]), MaxUserMessagesLimit)
	}

	_, ok := messageBuf.messageCollection[*userId][msgID]
	if !ok {
		t.Errorf("AddMessage: Message was not added to message buffer" +
			" properly!")
	}
}
