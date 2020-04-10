////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
)

// Interface for interacting with the UnmixedMessageBuffer.
type UnmixedMessageBuffer interface {
	// AddUnmixedMessage adds an unmixed message to send to the cMix node.
	AddUnmixedMessage(msg *pb.Slot)

	// PopUnmixedMessages pops messages off the message buffer stack.
	PopUnmixedMessages(minCnt, maxCnt uint64) *pb.Batch

	// LenUnmixed return the number of messages on the buffer
	LenUnmixed() int
}
