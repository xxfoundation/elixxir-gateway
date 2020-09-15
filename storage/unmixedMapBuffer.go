///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package storage

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/xx_network/primitives/id"
	"sync"
)

// UnmixedMessagesMap holds messages that have been received by gateway but have
// yet to been submitted to the network for mixing.
type UnmixedMessagesMap struct {
	messages map[id.Round]*SendRound
	mux      sync.Mutex
}

type SendRound struct {
	batch       *pb.Batch
	maxElements uint32
}

// NewUnmixedMessagesMap initialize a UnmixedMessageBuffer interface.
func NewUnmixedMessagesMap() UnmixedMessageBuffer {
	// Build the UnmixedMessagesMap
	buffer := &UnmixedMessagesMap{
		messages: map[id.Round]*SendRound{},
	}

	return buffer
}

// AddUnmixedMessage adds a message to send to the cMix node.
func (umb *UnmixedMessagesMap) AddUnmixedMessage(msg *pb.Slot, roundId id.Round) {
	umb.mux.Lock()
	defer umb.mux.Unlock()

	retrievedBatch := umb.messages[roundId]

	// If the batch for this round was already created, add another message
	retrievedBatch.batch.Slots = append(retrievedBatch.batch.Slots, msg)
	return
}

// GetRoundMessages returns the batch associated with the roundID
func (umb *UnmixedMessagesMap) GetRoundMessages(minMsgCnt uint64, roundId id.Round) *pb.Batch {
	umb.mux.Lock()
	defer umb.mux.Unlock()

	retrievedBatch, ok := umb.messages[roundId]
	if !ok {
		return nil
	}

	// Handle batches too small to send
	if numMessages := len(retrievedBatch.batch.Slots); numMessages == 0 {
		return &pb.Batch{}
	} else if uint64(numMessages) < minMsgCnt {
		return nil
	}

	return retrievedBatch.batch
}

// LenUnmixed return the number of messages within the requested round
func (umb *UnmixedMessagesMap) LenUnmixed(rndId id.Round) int {
	b, ok := umb.messages[rndId]
	if !ok {
		return 0
	}

	return len(b.batch.Slots)
}

func (umb *UnmixedMessagesMap) IsRoundFull(roundId id.Round) bool {
	umb.mux.Lock()
	defer umb.mux.Unlock()
	slots := umb.messages[roundId].batch.GetSlots()
	return len(slots) == int(umb.messages[roundId].maxElements)
}

// SetAsRoundLeader initializes a round as our responsibility ny initializing
//  marking that round as non-nil within the internal map
func (umb *UnmixedMessagesMap) SetAsRoundLeader(roundId id.Round, batchsize uint32) {
	umb.mux.Lock()
	defer umb.mux.Unlock()

	umb.messages[roundId] = &SendRound{
		batch:       &pb.Batch{},
		maxElements: batchsize,
	}
}

// IsRoundLeader returns true if object mapped to this round has
// been previously set
func (umb *UnmixedMessagesMap) IsRoundLeader(roundId id.Round) bool {
	umb.mux.Lock()
	defer umb.mux.Unlock()

	return umb.messages[roundId] != nil
}
