////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	pb "gitlab.com/privategrity/comms/mixmessages"
)

// Interface for interacting with the MessageBuffer (in prep for a db impl)
type MessageBuffer interface {
	// StartMessageCleanup is a thread that auto-removes old messages
	StartMessageCleanup(msgTimeout int)
	// GetMessage returns a given message for a specific user
	GetMessage(userId uint64, msgId string) (*pb.CmixMessage, bool)
	// GetMessageIDs returns the mesage IDs received after the given messageID, or
	// all message IDs if that ID cannot be found or is empty
	GetMessageIDs(userId uint64, messageID string) ([]string, bool)
	// DeleteMessage deletes a specific message
	DeleteMessage(userId uint64, msgId string)
	// AddMessage adds a message to the buffer for a specific user
	AddMessage(userId uint64, msgId string, msg *pb.CmixMessage)
	// AddOutGoingMessage adds a message to send to the cMix node
	AddOutgoingMessage(msg *pb.CmixMessage)
	// PopOutgoingBatch sends a batch of messages to the cMix node
	PopOutgoingBatch(batchSize uint64) []*pb.CmixMessage
}
