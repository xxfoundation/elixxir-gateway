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
	GetMessage(userId uint64, msgId string) (*pb.CmixMessage, bool)
	GetMessageIDs(userId uint64) ([]string, bool)
	DeleteMessage(userId uint64, msgId string)
	AddMessage(userId uint64, msgId string, msg *pb.CmixMessage)
	AddOutgoingMessage(msg *pb.CmixMessage)
	PopOutgoingBatch(batchSize uint64) []*pb.CmixMessage
}
