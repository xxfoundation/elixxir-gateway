////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"sync"
)

// UnmixedMapBuffer
type UnmixedMapBuffer struct {
	outgoingMessages pb.Batch
	mux              sync.Mutex
}

// NewUnmixedMessageBuffer initialize a UnmixedMessageBuffer interface.
func NewUnmixedMessageBuffer() UnmixedMessageBuffer {
	// Build the UnmixedMapBuffer
	buffer := &UnmixedMapBuffer{
		outgoingMessages: pb.Batch{},
	}

	return buffer
}

// AddUnmixedMessage adds a message to send to the cMix node.
func (umb *UnmixedMapBuffer) AddUnmixedMessage(msg *pb.Slot) {
	umb.mux.Lock()
	defer umb.mux.Unlock()
	umb.outgoingMessages.Slots = append(umb.outgoingMessages.Slots, msg)
}

// PopUnmixedMessages pops messages off the message buffer stack.
func (umb *UnmixedMapBuffer) PopUnmixedMessages(minCnt, batchSize uint64) *pb.Batch {
	umb.mux.Lock()
	defer umb.mux.Unlock()

	// Handle batches too small to send
	if numMessages := len(umb.outgoingMessages.Slots); numMessages == 0 {
		return &pb.Batch{}
	} else if uint64(numMessages) < minCnt {
		return nil
	}

	var messagesToTake uint64

	// If the batch is under full or exactly full
	if uint64(len(umb.outgoingMessages.Slots)) <= batchSize {
		messagesToTake = uint64(len(umb.outgoingMessages.Slots))
	} else {
		messagesToTake = batchSize
	}

	slots := umb.outgoingMessages.Slots[:messagesToTake]
	umb.outgoingMessages.Slots = umb.outgoingMessages.Slots[messagesToTake:]

	return &pb.Batch{Slots: slots}
}

// LenUnmixed returns the number of messages in queue.
func (umb *UnmixedMapBuffer) LenUnmixed() int {
	return len(umb.outgoingMessages.Slots)
}
