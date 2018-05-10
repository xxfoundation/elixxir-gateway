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

	buffer.AddMessage(userId, msgId, &msg)

	otherMsg, ok := buffer.GetMessage(userId, msgId)

	if msg.SenderID != otherMsg.SenderID || !ok {
		t.Errorf("GetMessage: Retrieved wrong message!")
	}

	buffer.DeleteMessage(userId, msgId)
	otherMsg, ok = buffer.GetMessage(userId, msgId)

	if ok {
		t.Errorf("DeleteMessage: Failed to delete message!")
	}
}
