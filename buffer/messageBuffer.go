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

// Interface for interacting with the MessageBuffer
type MessageBuffer interface {
	GetMessage(userId uint64, msgId string) (*pb.CmixMessage, bool)
}

// MessageBuffer struct with map backend
type MapBuffer struct {
	messageCollection map[uint64]map[string]*pb.CmixMessage
}

// Initialize a MessageBuffer interface
func newMessageBuffer() MessageBuffer {
	return MessageBuffer(&MapBuffer{
		messageCollection: make(map[uint64]map[string]*pb.CmixMessage),
	})
}

// Adds a message to the MessageBuffer
func  (m *MapBuffer) AddMessage(userId uint64, msgId string, msg *pb.CmixMessage) {
	if len(m.messageCollection[userId]) == 0 {
		// If the User->Message map hasn't been initialized, initialize it
		m.messageCollection[userId] = make(map[string]*pb.CmixMessage)
	}
	m.messageCollection[userId][msgId] = msg
}

// Returns message contents for MessageID, or a null/randomized message
// if that ID does not exist of the same size as a regular message
func  (m *MapBuffer) GetMessage(userId uint64, msgId string) (*pb.CmixMessage, bool) {
	msg, ok := m.messageCollection[userId][msgId]
	return msg, ok
}

// Deletes a given message from the MessageBuffer
func  (m *MapBuffer) DeleteMessage(userId uint64, msgId string) {
	delete(m.messageCollection[userId], msgId)
}
