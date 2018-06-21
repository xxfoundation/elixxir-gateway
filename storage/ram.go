////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"
	pb "gitlab.com/privategrity/comms/mixmessages"
	"sync"
	"time"
)

// MessageBuffer struct with map backend
type MapBuffer struct {
	messageCollection map[uint64]map[string]*pb.CmixMessage
	messageIDs        map[uint64][]string
	outgoingMessages  []*pb.CmixMessage
	messagesToDelete  []*MessageKey
	mux               sync.Mutex
}

// For storing userId and msgID key pairs in the message deletion queue
type MessageKey struct {
	userID uint64
	msgID  string
}

// Initialize a MessageBuffer interface
func NewMessageBuffer() MessageBuffer {
	// Build the Message Buffer
	buffer := MessageBuffer(&MapBuffer{
		messageCollection: make(map[uint64]map[string]*pb.CmixMessage),
		messageIDs:        make(map[uint64][]string),
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
			m.DeleteMessage(msgKey.userID, msgKey.msgID)
		}
		// Clear the newly deleted messages from the deletion queue
		m.messagesToDelete = nil
		// Traverse the nested map structure and flag
		// all messages for the next round of deletion
		for userID, msgMap := range m.messageCollection {
			for msgID := range msgMap {
				m.messagesToDelete = append(m.messagesToDelete,
					&MessageKey{
						userID: userID,
						msgID:  msgID,
					})
			}
		}
		// Sleep for the given message timeout
		time.Sleep(time.Duration(msgTimeout) * time.Second)
	}
}

// Returns message contents for MessageID, or a null/randomized message
// if that ID does not exist of the same size as a regular message
func (m *MapBuffer) GetMessage(userID uint64, msgID string) (*pb.CmixMessage,
	bool) {
	m.mux.Lock()
	msg, ok := m.messageCollection[userID][msgID]
	m.mux.Unlock()
	return msg, ok
}

// Return any MessageIDs in the globals for this UserID
func (m *MapBuffer) GetMessageIDs(userID uint64, messageID string) (
	[]string, bool) {
	m.mux.Lock()
	msgIDs, ok := m.messageIDs[userID]
	m.mux.Unlock()
	foundIDs := make([]string, 0)
	foundID := false
	for i := range msgIDs {
		if foundID {
			foundIDs = append(foundIDs, msgIDs[i])
		}
		// If this messages ID matches messageID, then mark we found it
		if msgIDs[i] == messageID {
			foundID = true
		}
	}
	// If we found messageID, return the IDs seen after we found it
	if foundID {
		msgIDs = foundIDs
	}
	return msgIDs, ok
}

// Deletes a given message from the MessageBuffer
func (m *MapBuffer) DeleteMessage(userID uint64, msgID string) {
	m.mux.Lock()
	delete(m.messageCollection[userID], msgID)

	// Delete this ID from the messageIDs slice
	msgIDs, _ := m.messageIDs[userID]
	newMsgIDs := make([]string, 0)
	for i := range msgIDs {
		if msgIDs[i] == msgID {
			continue
		}
		newMsgIDs = append(newMsgIDs, msgIDs[i])
	}
	m.messageIDs[userID] = newMsgIDs

	m.mux.Unlock()
}

// AddMessage adds a message to the buffer for a specific user
func (m *MapBuffer) AddMessage(userID uint64, msgID string,
	msg *pb.CmixMessage) {
	jww.DEBUG.Printf("Adding message %v from user %v to buffer.", msgID, userID)
	m.mux.Lock()
	if len(m.messageCollection[userID]) == 0 {
		// If the User->Message map hasn't been initialized, initialize it
		m.messageCollection[userID] = make(map[string]*pb.CmixMessage)
		m.messageIDs[userID] = make([]string, 0)
	}
	m.messageCollection[userID][msgID] = msg
	m.messageIDs[userID] = append(m.messageIDs[userID], msgID)
	m.mux.Unlock()
}

// AddOutGoingMessage adds a message to send to the cMix node
func (m *MapBuffer) AddOutgoingMessage(msg *pb.CmixMessage) {
	m.mux.Lock()
	m.outgoingMessages = append(m.outgoingMessages, msg)
	m.mux.Unlock()
}

// PopOutgoingBatch sends a batch of messages to the cMix node
func (m *MapBuffer) PopOutgoingBatch(batchSize uint64) []*pb.CmixMessage {
	m.mux.Lock()
	if uint64(len(m.outgoingMessages)) < batchSize {
		m.mux.Unlock()
		return nil
	}
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
