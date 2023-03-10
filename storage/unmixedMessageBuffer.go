////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/xx_network/primitives/id"
)

// Interface for interacting with the UnmixedMessageBuffer.
type UnmixedMessageBuffer interface {
	// AddUnmixedMessage adds an unmixed message to send to the cMix node.
	AddUnmixedMessage(msg *pb.Slot, sender *id.ID, ip string, round id.Round) error

	// AddManyUnmixedMessage adds many unmixed messages to send to the cMix node.
	AddManyUnmixedMessages(msg []*pb.GatewaySlot, sender *id.ID, ip string, round id.Round) error

	// GetRoundMessages returns the batch associated with the roundID
	PopRound(rndId id.Round) (*pb.Batch, []*id.ID, []string)

	// LenUnmixed return the number of messages within the requested round
	LenUnmixed(rndId id.Round) int

	// SetAsRoundLeader initializes a round as our responsibility	batchSize := 4

	SetAsRoundLeader(rndId id.Round, batchSize uint32)

	// IsRoundFull returns true if the number of slots associated with
	// the round ID matches the batchsize of that round
	IsRoundFull(rndId id.Round) bool

	// IsRoundLeader returns true if object mapped to this round has
	// been previously set
	IsRoundLeader(rndId id.Round) bool

}
