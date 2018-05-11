////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package buffer

import (
	pb "gitlab.com/privategrity/comms/mixmessages"
	"testing"
)

func TestMapBuffer(t *testing.T) {
	buffer := MapBuffer{
		messageCollection: make(map[uint64]map[string]*pb.CmixMessage),
	}
	msg := pb.CmixMessage{SenderID: uint64(666)}
	userId := uint64(0)
	msgId := "msg1"

	_, ok  := buffer.CheckMessages(userId)

	if ok {
		t.Errorf("CheckMessages: Expected no messages!")
	}

	buffer.AddMessage(userId, msgId, &msg)

	msgIds, ok := buffer.CheckMessages(userId)

	if !ok || msgIds[0] != msgId {
		t.Errorf("CheckMessages: Expected to find a message!")
	}

	otherMsg, ok := buffer.GetMessage(userId, msgIds[0])

	if !ok || msg.SenderID != otherMsg.SenderID {
		t.Errorf("GetMessage: Retrieved wrong message!")
	}

	buffer.DeleteMessage(userId, msgId)
	otherMsg, ok = buffer.GetMessage(userId, msgId)

	if ok {
		t.Errorf("DeleteMessage: Failed to delete message!")
	}
}
