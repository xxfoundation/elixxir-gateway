////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/primitives/id"
)

// Interface for interacting with the MessageBuffer (in prep for a db impl)
type MessageBuffer interface {
	// StartMessageCleanup is a thread that auto-removes old messages
	StartMessageCleanup(msgTimeout int)
	// GetMixedMessage returns a given message for a specific user
	GetMixedMessage(userId *id.User, msgId string) (*pb.Slot, bool)
	// GetMixedMessageIDs returns the message IDs received after the given messageID,
	// or all message IDs if that ID cannot be found or is empty
	GetMixedMessageIDs(userId *id.User, messageID string) ([]string, bool)
	// DeleteMixedMessage deletes a specific message
	DeleteMixedMessage(userId *id.User, msgId string)
	// AddMixedMessage adds a message to the buffer for a specific user
	AddMixedMessage(userId *id.User, msgId string, msg *pb.Slot)
	// AddOutGoingMessage adds a message to send to the cMix node
	AddUnmixedMessage(msg *pb.Slot)
	// PopUnmixedMessages returns at least minCnt and at most maxCnt messages, or nil.
	PopUnmixedMessages(minCnt, maxCnt uint64) *pb.Batch
	// Return the # of messages on the buffer
	LenUnmixed() int
}
