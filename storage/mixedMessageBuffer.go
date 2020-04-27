////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/primitives/id"
)

// Interface for interacting with the MixedMessageBuffer.
type MixedMessageBuffer interface {
	// StartMessageCleanup auto-removes old messages in the buffer and is
	// intended to be ran in a separate thread.
	StartMessageCleanup(msgTimeout int)

	// GetMixedMessage returns a given message for a message ID.
	GetMixedMessage(userId *id.User, msgId string) (*pb.Slot, error)

	// GetMixedMessageIDs returns the message IDs received after the given
	// message ID in the globals for the user.
	GetMixedMessageIDs(userId *id.User, messageID string) ([]string, error)

	// DeleteMixedMessage deletes a specific message from the buffer.
	DeleteMixedMessage(userId *id.User, msgId string)

	// AddMixedMessage adds a message to the buffer for a specific user.
	AddMixedMessage(userId *id.User, msgId string, msg *pb.Slot)
}