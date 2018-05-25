////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	pb "gitlab.com/privategrity/comms/mixmessages"
	"os"
	"testing"
	"time"
)

var messageBuf *MapBuffer

func TestMain(m *testing.M) {
	messageBuf = &MapBuffer{
		messageCollection: make(map[uint64]map[string]*pb.CmixMessage),
		outgoingMessages:  make([]*pb.CmixMessage, 0),
	}
	os.Exit(m.Run())
}

func TestMapBuffer_StartMessageCleanup(t *testing.T) {
	userId := uint64(520)
	// Use a separate buffer to not interfere with other tests
	cleanupBuf := &MapBuffer{
		messageCollection: make(map[uint64]map[string]*pb.CmixMessage),
		outgoingMessages:  make([]*pb.CmixMessage, 0),
	}

	// Add a few messages to the buffer
	cleanupBuf.messageCollection[userId] = make(map[string]*pb.CmixMessage)
	for i := 0; i < 5; i++ {
		cleanupBuf.messageCollection[userId][string(i)] = &pb.CmixMessage{}
	}

	// Start the cleanup
	go cleanupBuf.StartMessageCleanup(1)
	time.Sleep(2 * time.Second)

	// Make sure the messages no longer exist after
	numMsgs := len(cleanupBuf.messageCollection[userId])
	if numMsgs > 0 {
		t.Errorf("StartMessageCleanup: Expected all messages to be cleared, "+
			"%d messages still in buffer!", numMsgs)
	}
}

func TestMapBuffer_GetMessage(t *testing.T) {
	userId := uint64(0)
	msgId := "msg1"
	messageBuf.messageCollection[userId] = make(map[string]*pb.CmixMessage)
	messageBuf.messageCollection[userId][msgId] = &pb.CmixMessage{SenderID: userId}
	_, ok := messageBuf.GetMessage(userId, msgId)
	if !ok {
		t.Errorf("GetMessage: Unable to find message!")
	}
}

func TestMapBuffer_GetMessageIDs(t *testing.T) {
	userId := uint64(5)
	msgId := "msg1"
	messageBuf.messageCollection[userId] = make(map[string]*pb.CmixMessage)
	messageBuf.messageCollection[userId][msgId] = &pb.CmixMessage{SenderID: userId}
	msgIds, ok := messageBuf.GetMessageIDs(userId)
	if len(msgIds) < 1 || !ok {
		t.Errorf("GetMessageIDs: Unable to get any message IDs!")
	}
}

func TestMapBuffer_DeleteMessage(t *testing.T) {
	userId := uint64(555)
	msgId := "msg1"
	messageBuf.messageCollection[userId] = make(map[string]*pb.CmixMessage)
	messageBuf.messageCollection[userId][msgId] = &pb.CmixMessage{SenderID: userId}
	messageBuf.DeleteMessage(userId, msgId)
	_, ok := messageBuf.messageCollection[userId][msgId]
	if ok {
		t.Errorf("DeleteMessage: Message was not deleted properly!")
	}
}

func TestMapBuffer_AddMessage(t *testing.T) {
	userId := uint64(10)
	msgId := "msg1"
	messageBuf.AddMessage(userId, msgId, &pb.CmixMessage{SenderID: userId})
	_, ok := messageBuf.messageCollection[userId][msgId]
	if !ok {
		t.Errorf("AddMessage: Message was not added to message buffer" +
			" properly!")
	}
}

func TestMapBuffer_AddOutgoingMessage(t *testing.T) {
	numOutgoingMsgs := len(messageBuf.outgoingMessages)
	messageBuf.AddOutgoingMessage(&pb.CmixMessage{SenderID: uint64(0)})
	if len(messageBuf.outgoingMessages) != numOutgoingMsgs+1 {
		t.Errorf("AddOutgoingMessage: Message was not added to outgoing" +
			" message buffer properly!")
	}
}

func TestMapBuffer_PopOutgoingBatch(t *testing.T) {
	messageBuf.outgoingMessages = append(messageBuf.outgoingMessages,
		&pb.CmixMessage{SenderID: uint64(0)})
	messageBuf.PopOutgoingBatch(1)
	if len(messageBuf.outgoingMessages) > 0 {
		t.Errorf("PopOutgoingBatch: Batch was not popped correctly!")
	}
}
