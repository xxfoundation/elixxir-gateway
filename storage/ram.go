////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	"github.com/spf13/viper"
	pb "gitlab.com/privategrity/comms/mixmessages"
	"sync"
	"time"
)

// MessageBuffer struct with map backend
type MapBuffer struct {
	messageCollection map[uint64]map[string]*pb.CmixMessage
	outgoingMessages  []*pb.CmixMessage
	messagesToDelete  []*MessageKey
	mux               sync.Mutex
}

type MessageKey struct {
	userId uint64
	msgId  string
}

// Initialize a MessageBuffer interface
func NewMessageBuffer() MessageBuffer {
	// Build the Message Buffer
	buffer := MessageBuffer(&MapBuffer{
		messageCollection: make(map[uint64]map[string]*pb.CmixMessage),
		outgoingMessages:  make([]*pb.CmixMessage, 0),
		messagesToDelete:  make([]*MessageKey, 0),
	})
	// Start the message cleanup loop with configured message timeout
	go buffer.StartMessageCleanup(viper.GetInt("MessageTimeout"))
	return buffer
}

// Clear all messages from the internal MessageBuffer after the given
// message timeout. Intended to be ran in a separate thread.
func (m *MapBuffer) StartMessageCleanup(msgTimeout int) {
	for {
		// Delete all messages already marked for deletion
		for _, msgKey := range m.messagesToDelete {
			m.DeleteMessage(msgKey.userId, msgKey.msgId)
		}
		// Clear the newly deleted messages from the deletion queue
		m.messagesToDelete = nil
		// Traverse the nested map structure and flag
		// all messages for the next round of deletion
		for userId, msgMap := range m.messageCollection {
			for msgId := range msgMap {
				m.messagesToDelete = append(m.messagesToDelete,
					&MessageKey{
						userId: userId,
						msgId:  msgId,
					})
			}
		}
		// Sleep for the given message timeout
		time.Sleep(time.Duration(msgTimeout) * time.Second)
	}
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
