///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package storage

import (
	"fmt"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/xx_network/primitives/id"
	"sync"
)

// UnmixedMessagesMap holds messages that have been received by gateway but have
// yet to been submitted to the network for mixing.
type UnmixedMessagesMap struct {
	messages map[id.Round]*SendRound
	mux      sync.RWMutex
}

type SendRound struct {
	batch       *pb.Batch
	maxElements uint32
	sent        bool
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
func (umb *UnmixedMessagesMap) AddUnmixedMessage(msg *pb.Slot, roundId id.Round) error {
	umb.mux.Lock()
	defer umb.mux.Unlock()

	retrievedBatch, ok := umb.messages[roundId]
	if !ok {
		return errors.New("cannot add message to unknown round")
	}

	if retrievedBatch.sent {
		return errors.New("Cannot add message to already sent batch")
	}

	if len(retrievedBatch.batch.Slots) == int(retrievedBatch.maxElements) {
		return errors.New("Cannot add message to full batch")
	}

	// If the batch for this round was already created, add another message
	retrievedBatch.batch.Slots = append(retrievedBatch.batch.Slots, msg)
	umb.messages[roundId] = retrievedBatch
	return nil
}

// AddManyUnmixedMessage adds many unmixed messages to send to the cMix node.
func (umb *UnmixedMessagesMap) AddManyUnmixedMessages(msgs []*pb.GatewaySlot, roundId id.Round) error {
	umb.mux.Lock()
	defer umb.mux.Unlock()

	// Pull batch from store (map)
	retrievedBatch, ok := umb.messages[roundId]
	if !ok {
		return errors.New("cannot add message to unknown round")
	}

	// Check that the batch has not
	if retrievedBatch.sent {
		return errors.New("Cannot add message to already sent batch")
	}

	// Check that adding these message wil not exceed the batch size
	resultingSlots := len(retrievedBatch.batch.Slots) + len(msgs)
	if resultingSlots >= int(retrievedBatch.maxElements) {
		fmt.Printf("resulting Slots: %d\nmax %d\n", resultingSlots, int(retrievedBatch.maxElements))
		return errors.New("Cannot add messages to full batch")
	}

	// Collect all slots into a list
	slots := make([]*pb.Slot, len(msgs))
	for i := 0; i < len(msgs); i++ {
		slots[i] = msgs[i].Message
	}

	// If the batch for this round was already created, add another message
	retrievedBatch.batch.Slots = append(retrievedBatch.batch.Slots, slots...)
	umb.messages[roundId] = retrievedBatch
	return nil
}

// GetRoundMessages returns the batch associated with the roundID
func (umb *UnmixedMessagesMap) PopRound(roundId id.Round) *pb.Batch {
	umb.mux.Lock()
	defer umb.mux.Unlock()

	retrievedBatch, ok := umb.messages[roundId]
	if !ok {
		return nil
	}

	retrievedBatch.sent = true

	// Handle batches too small to send
	batch := retrievedBatch.batch
	retrievedBatch.batch = nil
	umb.messages[roundId] = retrievedBatch
	return batch
}

// LenUnmixed return the number of messages within the requested round
func (umb *UnmixedMessagesMap) LenUnmixed(rndId id.Round) int {
	umb.mux.RLock()
	defer umb.mux.RUnlock()
	b, ok := umb.messages[rndId]
	if !ok {
		return 0
	}

	return len(b.batch.Slots)
}

func (umb *UnmixedMessagesMap) IsRoundFull(roundId id.Round) bool {
	umb.mux.RLock()
	defer umb.mux.RUnlock()
	slots := umb.messages[roundId].batch.GetSlots()
	return len(slots) == int(umb.messages[roundId].maxElements)
}

// SetAsRoundLeader initializes a round as our responsibility ny initializing
//  marking that round as non-nil within the internal map
func (umb *UnmixedMessagesMap) SetAsRoundLeader(roundId id.Round, batchsize uint32) {
	umb.mux.Lock()
	defer umb.mux.Unlock()

	if _, ok := umb.messages[roundId]; ok {
		jww.FATAL.Panicf("Can set as round leader for extant round %d",
			roundId)
	}
	jww.INFO.Printf("Adding round buffer for round %d", roundId)
	umb.messages[roundId] = &SendRound{
		batch:       &pb.Batch{Slots: make([]*pb.Slot,0,batchsize)},
		maxElements: batchsize,
	}
}

// IsRoundLeader returns true if object mapped to this round has
// been previously set
func (umb *UnmixedMessagesMap) IsRoundLeader(roundId id.Round) bool {
	umb.mux.RLock()
	defer umb.mux.RUnlock()

	_, ok := umb.messages[roundId]
	return ok
}
