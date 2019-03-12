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
	// GetMessage returns a given message for a specific user
	GetMessage(userId *id.User, msgId string) (*pb.CmixMessage, bool)
	// GetMessageIDs returns the mesage IDs received after the given messageID, or
	// all message IDs if that ID cannot be found or is empty
	GetMessageIDs(userId *id.User, messageID string) ([]string, bool)
	// DeleteMessage deletes a specific message
	DeleteMessage(userId *id.User, msgId string)
	// AddMessage adds a message to the buffer for a specific user
	AddMessage(userId *id.User, msgId string, msg *pb.CmixMessage)
	// AddOutGoingMessage adds a message to send to the cMix node
	AddOutgoingMessage(msg *pb.CmixMessage)
	// PopMessages returns at least minCnt and at most maxCnt messages, or nil.
	PopMessages(minCnt, maxCnt uint64) []*pb.CmixMessage
	// Return the # of messages on the buffer
	Len() int
}
