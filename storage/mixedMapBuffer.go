///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package storage

import (
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/primitives/id"
	"sync"
	"time"
)

// The Maximum number of user messages to keep. If this limit is eclipsed, then
// messages at the front of the buffer are deleted.
const MaxUserMessagesLimit = 1000

// MixedMapBuffer struct with map backend.
type MixedMapBuffer struct {
	messageCollection map[id.ID]map[string]*pb.Slot // Received messages
	messageIDs        map[id.ID][]string
	messagesToDelete  []*MessageKey
	mux               sync.Mutex
}

// MessageKey stores the user ID and message ID key pairs in the message
// deletion queue.
type MessageKey struct {
	userID *id.ID
	msgID  string
}

// NewMixedMessageBuffer initialize a MixedMessageBuffer interface.
func NewMixedMessageBuffer(messageTimeout time.Duration) MixedMessageBuffer {
	// Build the MixedMapBuffer
	buffer := &MixedMapBuffer{
		messageCollection: make(map[id.ID]map[string]*pb.Slot),
		messageIDs:        make(map[id.ID][]string),
		messagesToDelete:  make([]*MessageKey, 0),
	}

	// Start the message cleanup loop with configured message timeout
	go buffer.StartMessageCleanup(messageTimeout)

	return buffer
}

// StartMessageCleanup clears all messages from the internal MessageBuffer
// after the given message timeout. Intended to be ran in a separate thread.
func (mmb *MixedMapBuffer) StartMessageCleanup(msgTimeout time.Duration) {
	for {
		mmb.mux.Lock()

		// Delete all messages already marked for deletion
		for _, msgKey := range mmb.messagesToDelete {
			mmb.deleteMessage(msgKey.userID, msgKey.msgID)
		}

		// Clear the newly deleted messages from the deletion queue
		mmb.messagesToDelete = nil

		// Traverse the nested map structure and flag all messages for the next
		// round of deletion
		for userID, msgMap := range mmb.messageCollection {
			for msgID := range msgMap {
				mmb.messagesToDelete = append(mmb.messagesToDelete,
					&MessageKey{
						userID: &userID,
						msgID:  msgID,
					},
				)
			}
		}

		mmb.mux.Unlock()

		// Sleep for the given message timeout
		time.Sleep(msgTimeout)
	}
}

// GetMixedMessage returns message contents for the message ID or a null/
// randomized message if that ID does not exist of the same size as a regular
// message.
func (mmb *MixedMapBuffer) GetMixedMessage(userID *id.ID, msgID string) (*pb.Slot, error) {
	mmb.mux.Lock()

	msg, ok := mmb.messageCollection[*userID][msgID]

	mmb.mux.Unlock()

	if ok {
		return msg, nil
	} else {
		return msg, errors.New("Could not find any messages with the given ID")
	}
}

// GetMixedMessageIDs return a list of message IDs in the globals for this user.
func (mmb *MixedMapBuffer) GetMixedMessageIDs(userID *id.ID, messageID string) ([]string, error) {
	mmb.mux.Lock()

	// msgIDs is a view into the same memory that mmb.messageIDs has, so we must
	// hold the lock until the end to avoid a read-after-write hazard
	msgIDs, ok := mmb.messageIDs[*userID]
	foundIDs := make([]string, 0)
	foundID := false

	// Give every message ID found AFTER the specified message ID
	for i := range msgIDs {
		if foundID {
			foundIDs = append(foundIDs, msgIDs[i])
		}

		// If this messages ID matches messageID, then mark it as found
		if msgIDs[i] == messageID {
			foundID = true
		}
	}

	// If a messageID was found, then return the IDs
	if foundID {
		msgIDs = foundIDs
	}

	mmb.mux.Unlock()

	if ok {
		return msgIDs, nil
	} else {
		return msgIDs, errors.New("Could not find any message IDs for this user")
	}
}

// DeleteMixedMessage deletes a given message from the message buffer.
func (mmb *MixedMapBuffer) DeleteMixedMessage(userID *id.ID, msgID string) {
	mmb.mux.Lock()
	defer mmb.mux.Unlock()
	mmb.deleteMessage(userID, msgID)
}

// deleteMessage deletes a message without locking. Only call this from a method
// that is already locked using the mutex.
func (mmb *MixedMapBuffer) deleteMessage(userID *id.ID, msgID string) {
	delete(mmb.messageCollection[*userID], msgID)

	// Delete this ID from the messageIDs slice
	msgIDs, _ := mmb.messageIDs[*userID]
	newMsgIDs := make([]string, 0)

	for i := range msgIDs {
		if msgIDs[i] == msgID {
			continue
		}

		newMsgIDs = append(newMsgIDs, msgIDs[i])
	}

	mmb.messageIDs[*userID] = newMsgIDs
}

// AddMixedMessage adds a message to the buffer for a specific user.
func (mmb *MixedMapBuffer) AddMixedMessage(userID *id.ID, msgID string, msg *pb.Slot) {
	jww.DEBUG.Printf("Adding mixed message %v for user %v to buffer.", msgID, userID)

	mmb.mux.Lock()
	if len(mmb.messageCollection[*userID]) == 0 {
		// If the User->Message map has not been initialized, then initialize it
		mmb.messageCollection[*userID] = make(map[string]*pb.Slot)
		mmb.messageIDs[*userID] = make([]string, 0)
	}

	// Delete messages if we exceed the messages limit for this user.
	// NOTE: Careful if you decide on putting this outside the lock and removing
	// the defer it's very easy to get in race condition territory there. This
	// code was put here intentionally to keep things more readable as well as
	// make a decision inside of the lock. Delete does not panic on an already
	// deleted value, so it is ok to delete the same message multiple times.
	if (len(mmb.messageIDs[*userID]) + 1) > MaxUserMessagesLimit {
		deleteCount := len(mmb.messageIDs[*userID]) - MaxUserMessagesLimit + 1
		msgIDsToDelete := mmb.messageIDs[*userID][0:deleteCount]

		jww.DEBUG.Printf("%v message limit exceeded, deleting %d messages: %v",
			userID, deleteCount, msgIDsToDelete)

		defer func(m *MixedMapBuffer, userID *id.ID, msgIDs []string) {
			for i := range msgIDs {
				m.DeleteMixedMessage(userID, msgIDs[i])
			}
		}(mmb, userID, msgIDsToDelete)
	}

	mmb.messageCollection[*userID][msgID] = msg
	mmb.messageIDs[*userID] = append(mmb.messageIDs[*userID], msgID)
	mmb.mux.Unlock()
}
