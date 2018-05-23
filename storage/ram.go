////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	pb "gitlab.com/privategrity/comms/mixmessages"
	"sync"
)

// MessageBuffer struct with map backend
type MapBuffer struct {
	messageCollection map[uint64]map[string]*pb.CmixMessage
	outgoingMessages  []*pb.CmixMessage
	mux               sync.Mutex
}

// Initialize a MessageBuffer interface
func NewMessageBuffer() MessageBuffer {
	return MessageBuffer(&MapBuffer{
		messageCollection: make(map[uint64]map[string]*pb.CmixMessage),
		outgoingMessages:  make([]*pb.CmixMessage, 0),
	})
}

// Returns message contents for MessageID, or a null/randomized message
// if that ID does not exist of the same size as a regular message
func (m *MapBuffer) GetMessage(userId uint64, msgId string) (*pb.CmixMessage,
	bool) {
	m.mux.Lock()
	msg, ok := m.messageCollection[userId][msgId]
	m.mux.Unlock()
	return msg, ok
}

// Return any MessageIDs in the globals for this UserID
func (m *MapBuffer) GetMessageIDs(userId uint64) ([]string, bool) {
	m.mux.Lock()
	userMap, ok := m.messageCollection[userId]
	m.mux.Unlock()
	msgIds := make([]string, 0, len(userMap))
	for msgId := range userMap {
		msgIds = append(msgIds, msgId)
	}
	return msgIds, ok
}

// Deletes a given message from the MessageBuffer
func (m *MapBuffer) DeleteMessage(userId uint64, msgId string) {
	m.mux.Lock()
	delete(m.messageCollection[userId], msgId)
	m.mux.Unlock()
}

// Adds a message to the MessageBuffer
func (m *MapBuffer) AddMessage(userId uint64, msgId string,
	msg *pb.CmixMessage) {
	m.mux.Lock()
	if len(m.messageCollection[userId]) == 0 {
		// If the User->Message map hasn't been initialized, initialize it
		m.messageCollection[userId] = make(map[string]*pb.CmixMessage)
	}
	m.messageCollection[userId][msgId] = msg
	m.mux.Unlock()
}

//
func (m *MapBuffer) AddOutgoingMessage(msg *pb.CmixMessage) {
	m.mux.Lock()
	m.outgoingMessages = append(m.outgoingMessages, msg)
	m.mux.Unlock()
}

//
func (m *MapBuffer) PopOutgoingBatch(batchSize uint64) []*pb.CmixMessage {
	if uint64(len(m.outgoingMessages)) < batchSize {
		return nil
	}
	m.mux.Lock()
	outgoingBatch := m.outgoingMessages[:batchSize]
	// If there are more outgoing messages than the batchSize
	if uint64(len(m.outgoingMessages)) > batchSize {
		// Empty the batch from the slice
		m.outgoingMessages = m.outgoingMessages[batchSize+1:]
	} else {
		// Otherwise, empty the slice
		m.outgoingMessages = nil
	}
	m.mux.Unlock()
	return outgoingBatch
}
