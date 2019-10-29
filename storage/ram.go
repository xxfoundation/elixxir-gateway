////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/primitives/id"
	"sync"
	"time"
)

// The Maximum number of user messages to keep. If this limit is eclipsed,
// messages at the front of the buffer are deleted.
const MaxUserMessagesLimit = 1000

// MessageBuffer struct with map backend
type MapBuffer struct {
	messageCollection map[id.User]map[string]*pb.Slot
	messageIDs        map[id.User][]string
	outgoingMessages  pb.Batch
	messagesToDelete  []*MessageKey
	mux               sync.Mutex
}

// For storing userId and msgID key pairs in the message deletion queue
type MessageKey struct {
	userID *id.User
	msgID  string
}

// NewMessageBuffer Initialize a MessageBuffer interface
func NewMessageBuffer() MessageBuffer {
	// Build the Message Buffer
	buffer := &MapBuffer{
		messageCollection: make(map[id.User]map[string]*pb.Slot),
		messageIDs:        make(map[id.User][]string),
		messagesToDelete:  make([]*MessageKey, 0),
	}
	// Start the message cleanup loop with configured message timeout
	go buffer.StartMessageCleanup(viper.GetInt("MessageTimeout"))
	return buffer
}

// StartMessageCleanup Clear all messages from the internal MessageBuffer
// after the given message timeout. Intended to be ran in a separate thread.
func (m *MapBuffer) StartMessageCleanup(msgTimeout int) {
	for {
		m.mux.Lock()
		// Delete all messages already marked for deletion
		for _, msgKey := range m.messagesToDelete {
			m.deleteMessage(msgKey.userID, msgKey.msgID)
		}
		// Clear the newly deleted messages from the deletion queue
		m.messagesToDelete = nil
		// Traverse the nested map structure and flag
		// all messages for the next round of deletion
		for userID, msgMap := range m.messageCollection {
			for msgID := range msgMap {
				m.messagesToDelete = append(m.messagesToDelete,
					&MessageKey{
						userID: &userID,
						msgID:  msgID,
					})
			}
		}
		m.mux.Unlock()
		// Sleep for the given message timeout
		time.Sleep(time.Duration(msgTimeout) * time.Second)
	}
}

// GetMixedMessage returns message contents for MessageID, or a
// null/randomized message if that ID does not exist of the same
// size as a regular message
func (m *MapBuffer) GetMixedMessage(userID *id.User, msgID string) (*pb.Slot, bool) {
	m.mux.Lock()
	msg, ok := m.messageCollection[*userID][msgID]
	m.mux.Unlock()
	return msg, ok
}

// GetMixedMessageIDs return any MessageIDs in the globals for this User
func (m *MapBuffer) GetMixedMessageIDs(userID *id.User, messageID string) ([]string, bool) {
	m.mux.Lock()
	// msgIDs is a view into the same memory that m.messageIDs has, so we must
	// hold the lock until the end to avoid a read-after-write hazard
	msgIDs, ok := m.messageIDs[*userID]
	foundIDs := make([]string, 0)
	foundID := false
	//Give every ID AFTER a found ID
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
	m.mux.Unlock()
	return msgIDs, ok
}

// DeleteMixedMessage deletes a given message from the MessageBuffer
func (m *MapBuffer) DeleteMixedMessage(userID *id.User, msgID string) {
	m.mux.Lock()
	m.deleteMessage(userID, msgID)
	m.mux.Unlock()
}

// Delete message without locking
// Call this from a method that's already locked the mutex
func (m *MapBuffer) deleteMessage(userID *id.User, msgID string) {
	delete(m.messageCollection[*userID], msgID)

	// Delete this ID from the messageIDs slice
	msgIDs, _ := m.messageIDs[*userID]
	newMsgIDs := make([]string, 0)
	for i := range msgIDs {
		if msgIDs[i] == msgID {
			continue
		}
		newMsgIDs = append(newMsgIDs, msgIDs[i])
	}
	m.messageIDs[*userID] = newMsgIDs
}

// AddMixedMessage adds a message to the buffer for a specific user
func (m *MapBuffer) AddMixedMessage(userID *id.User, msgID string, msg *pb.Slot) {
	jww.DEBUG.Printf("Adding mixed message %v for user %v to buffer.", msgID, userID)
	m.mux.Lock()
	if len(m.messageCollection[*userID]) == 0 {
		// If the User->Message map hasn't been initialized, initialize it
		m.messageCollection[*userID] = make(map[string]*pb.Slot)
		m.messageIDs[*userID] = make([]string, 0)
	}

	// Delete messages if we exceed the messages limit for this user
	// NOTE: Careful if you decide on putting this outside the lock and removing
	// the defer it's very easy to get in race condition territory there. This
	// Code was put here intentionally to keep things more readable as well as
	// make a decision inside of the lock. Delete does not panic on an already
	// deleted value, so it is ok to delete the same message multiple times.
	if (len(m.messageIDs[*userID]) + 1) > MaxUserMessagesLimit {
		deleteCount := len(m.messageIDs[*userID]) - MaxUserMessagesLimit + 1
		msgIDsToDelete := m.messageIDs[*userID][0:deleteCount]
		jww.DEBUG.Printf("%v message limit exceeded, deleting %d messages: %v",
			userID, deleteCount, msgIDsToDelete)
		defer func(m *MapBuffer, userID *id.User, msgIDs []string) {
			for i := range msgIDs {
				m.DeleteMixedMessage(userID, msgIDs[i])
			}
		}(m, userID, msgIDsToDelete)
	}

	m.messageCollection[*userID][msgID] = msg
	m.messageIDs[*userID] = append(m.messageIDs[*userID], msgID)
	m.mux.Unlock()
}

// AddUnmixedMessage adds a message to send to the cMix node
func (m *MapBuffer) AddUnmixedMessage(msg *pb.Slot) {
	m.mux.Lock()
	m.outgoingMessages.Slots = append(m.outgoingMessages.Slots, msg)
	m.mux.Unlock()
}

// PopUnmixedMessages pops messages off the message buffer stack
func (m *MapBuffer) PopUnmixedMessages(minCnt, batchSize uint64) *pb.Batch {
	m.mux.Lock()
	defer m.mux.Unlock()

	// Handle batches too small to send
	if numMessages := len(m.outgoingMessages.Slots); numMessages == 0 {
		return &pb.Batch{}
	} else if uint64(numMessages) < minCnt {
		return nil
	}

	var messagesToTake uint64

	// if the batch is underfull or exactly full
	if uint64(len(m.outgoingMessages.Slots)) <= batchSize {
		messagesToTake = uint64(len(m.outgoingMessages.Slots))
	} else {
		messagesToTake = batchSize
	}

	slots := m.outgoingMessages.Slots[:messagesToTake]
	m.outgoingMessages.Slots = m.outgoingMessages.Slots[messagesToTake:]

	return &pb.Batch{Slots: slots}
}

// LenUnmixed returns # of messages in queue
func (m *MapBuffer) LenUnmixed() int {
	return len(m.outgoingMessages.Slots)
}
