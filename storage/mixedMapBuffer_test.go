////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/primitives/id"
	"strings"
	"testing"
	"time"
)

func TestMixedMapBuffer_StartMessageCleanup(t *testing.T) {
	userId := id.NewIdFromUInt(528, id.User, t)

	// Use a separate buffer as to not interfere with other tests
	cleanupBuf := &MixedMapBuffer{
		messageCollection: make(map[id.ID]map[string]*pb.Slot),
		messageIDs:        make(map[id.ID][]string),
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
	mixedMessageBuf := initMixedMapBuffer()
	userId := id.ZeroUser
	msgId := "msg1"
	mixedMessageBuf.messageCollection[userId] = make(map[string]*pb.Slot)
	mixedMessageBuf.messageCollection[userId][msgId] = &pb.Slot{
		SenderID: userId.Bytes(),
	}

	_, err := mixedMessageBuf.GetMixedMessage(&userId, msgId)
	if err != nil {
		t.Errorf("GetMixedMessage: Unable to find message!")
	}
}

func TestMapBuffer_GetMessageIDs(t *testing.T) {
	mixedMessageBuf := initMixedMapBuffer()
	userId := id.NewIdFromUInt(5, id.User, t)
	msgId := "msg1"
	mixedMessageBuf.messageCollection[*userId] = make(map[string]*pb.Slot)
	mixedMessageBuf.messageCollection[*userId][msgId] = &pb.Slot{
		SenderID: userId.Bytes(),
	}
	mixedMessageBuf.messageIDs[*userId] = make([]string, 1)
	mixedMessageBuf.messageIDs[*userId][0] = msgId

	msgIds, err := mixedMessageBuf.GetMixedMessageIDs(userId, "")
	if len(msgIds) < 1 || err != nil {
		t.Errorf("GetMixedMessageIDs: Unable to get any message IDs!")
	}

	msgIds, err = mixedMessageBuf.GetMixedMessageIDs(userId, "msg1")
	if len(msgIds) != 0 || err != nil {
		t.Errorf("GetMixedMessageIDs: Could not get message IDs after 'msg1'")
	}

	// Test that it returns every message after a found ID
	msgId2 := "msg2"
	mixedMessageBuf.messageIDs[*userId] = make([]string, 2)
	mixedMessageBuf.messageIDs[*userId][0] = msgId
	mixedMessageBuf.messageIDs[*userId][1] = msgId2

	msgIds, err = mixedMessageBuf.GetMixedMessageIDs(userId, "msg1")
	if len(msgIds) == 0 || err != nil || strings.Compare(msgIds[0], msgId2) != 0 {
		t.Errorf("GetMixedMessageIDs: Could not get message IDs after 'msg1'")
	}

}

func TestMapBuffer_DeleteMessage(t *testing.T) {
	mixedMessageBuf := initMixedMapBuffer()
	userId := id.NewIdFromUInt(555, id.User, t)
	msgId := "msg1"
	mixedMessageBuf.messageCollection[*userId] = make(map[string]*pb.Slot)
	mixedMessageBuf.messageIDs[*userId] = make([]string, 0)
	mixedMessageBuf.messageIDs[*userId] = append(mixedMessageBuf.messageIDs[*userId], "msgId")
	mixedMessageBuf.messageCollection[*userId][msgId] = &pb.Slot{
		SenderID: userId.Bytes(),
	}

	mixedMessageBuf.DeleteMixedMessage(userId, msgId)

	_, ok := mixedMessageBuf.messageCollection[*userId][msgId]
	if ok {
		t.Errorf("DeleteMixedMessage: Message was not deleted properly!")
	}
}

func TestMapBuffer_AddMessage(t *testing.T) {
	mixedMessageBuf := initMixedMapBuffer()
	userId := id.NewIdFromUInt(10, id.User, t)
	msgId := "msg1"

	mixedMessageBuf.AddMixedMessage(userId, msgId, &pb.Slot{SenderID: userId.Bytes()})

	_, ok := mixedMessageBuf.messageCollection[*userId][msgId]
	if !ok {
		t.Errorf("AddMixedMessage: Message was not added to message buffer" +
			" properly!")
	}
}

func TestMapBuffer_ExceedUserMsgsLimit(t *testing.T) {
	mixedMessageBuf := initMixedMapBuffer()
	userId := id.NewIdFromUInt(10, id.User, t)
	msgIDFmt := "msg1"

	deleteMe := mixedMessageBuf.messageIDs[*userId]
	for i := range deleteMe {
		mixedMessageBuf.DeleteMixedMessage(userId, deleteMe[i])
	}

	for i := 0; i < MaxUserMessagesLimit; i++ {
		msgID := msgIDFmt + string(i)
		mixedMessageBuf.AddMixedMessage(userId, msgID,
			&pb.Slot{SenderID: userId.Bytes()})
	}

	if len(mixedMessageBuf.messageIDs[*userId]) != MaxUserMessagesLimit {
		t.Errorf("Message limit not exceeded, but length incorrect: %d v. %d",
			len(mixedMessageBuf.messageIDs[*userId]), MaxUserMessagesLimit)
	}

	msgID := msgIDFmt + "Hello"
	mixedMessageBuf.AddMixedMessage(userId, msgID,
		&pb.Slot{SenderID: userId.Bytes()})

	if len(mixedMessageBuf.messageIDs[*userId]) != MaxUserMessagesLimit {
		t.Errorf("Message limit exceeded, but length incorrect: %d v. %d",
			len(mixedMessageBuf.messageIDs[*userId]), MaxUserMessagesLimit)
	}

	_, ok := mixedMessageBuf.messageCollection[*userId][msgID]
	if !ok {
		t.Errorf("AddMixedMessage: Message was not added to message buffer" +
			" properly!")
	}
}

func initMixedMapBuffer() *MixedMapBuffer {
	return &MixedMapBuffer{
		messageCollection: make(map[id.ID]map[string]*pb.Slot),
		messageIDs:        make(map[id.ID][]string),
	}
}
