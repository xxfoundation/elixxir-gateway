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
	GlobalMessageBuffer = newMessageBuffer()

	msg := pb.CmixMessage{SenderID: uint64(666)}
	userId := uint64(0)
	msgId := "msg1"

	GlobalMessageBuffer.AddMessage(userId, msgId, msg)

	otherMsg, ok := GlobalMessageBuffer.GetMessage(userId, msgId)

	if msg.SenderID != otherMsg.SenderID || !ok {
		t.Errorf("GetMessage: Retrieved wrong message!")
	}

	GlobalMessageBuffer.DeleteMessage(userId, msgId)
	otherMsg, ok = GlobalMessageBuffer.GetMessage(userId, msgId)

	if ok {
		t.Errorf("DeleteMessage: Failed to delete message!")
	}
}
