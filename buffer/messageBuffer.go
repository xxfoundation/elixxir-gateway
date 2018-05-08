////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package buffer

import (
	pb "gitlab.com/privategrity/comms/mixmessages"
)

// Global instance of the in-memory Message Buffer
var GlobalMessageBuffer MessageBuffer = newMessageBuffer()

//
type MessageBuffer interface {
	GetMessage(userId uint64, msgId string) pb.CmixMessage
}

// MessageBuffer struct with map backend
type MapBuffer struct {
	messageCollection map[uint64]map[string]pb.CmixMessage
}

// Initialize a MessageBuffer interface
func newMessageBuffer() MessageBuffer {
	return MessageBuffer(&MapBuffer{
		messageCollection: make(map[uint64]map[string]pb.CmixMessage),
	})
}

// Returns message contents for MessageID, or a null/randomized message
// if that ID does not exist of the same size as a regular message
func  (m *MapBuffer) GetMessage(userId uint64, msgId string) pb.CmixMessage {
	return m.messageCollection[userId][msgId]
}
