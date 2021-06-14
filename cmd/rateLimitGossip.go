///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

//  Contains gossip methods specific to rate limit gossiping

package cmd

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/comms/network/dataStructures"
	"gitlab.com/elixxir/primitives/states"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/rateLimiting"
	"time"
)

// Initialize fields required for the gossip protocol specialized to rate limiting
func (gw *Instance) InitRateLimitGossip() {

	// Initialize leaky bucket
	gw.rateLimitQuit = make(chan struct{}, 1)
	gw.rateLimit = rateLimiting.CreateBucketMapFromParams(gw.Params.rateLimitParams, nil, gw.rateLimitQuit)
	fmt.Printf("gwComms: %v\n", gw.Comms)
	fmt.Printf("gwManager: %v\n", gw.Comms.Manager)

	// Register gossip protocol for client rate limiting
	gw.Comms.Manager.NewGossip(RateLimitGossip, gossip.DefaultProtocolFlags(),
		gw.gossipRateLimitReceive, gw.gossipVerify, nil)
}

func verifyRateLimit(msg *gossip.GossipMsg, origin *id.ID, instance *network.Instance) error {
	// Parse the payload message
	payloadMsg := &pb.BatchSenders{}
	err := proto.Unmarshal(msg.Payload, payloadMsg)
	if err != nil {
		return errors.Errorf("Could not unmarshal message into expected format: %s", err)
	}

	// Check if we recognize the round
	r := id.Round(payloadMsg.RoundID)
	ri, err := instance.GetRound(r)
	if err != nil {
		eventChan := make(chan dataStructures.EventReturn, 1)
		instance.GetRoundEvents().AddRoundEventChan(r,
			eventChan, 60*time.Second, states.COMPLETED, states.FAILED)

		// Check if we recognize the round
		event := <-eventChan
		if event.TimedOut {
			return errors.Errorf("Failed to lookup round %v sent out by gossip message.", payloadMsg.RoundID)
		} else if states.Round(event.RoundInfo.State) == states.FAILED {
			return errors.Errorf("Round %v sent out by gossip message failed.", payloadMsg.RoundID)
		}

		ri = event.RoundInfo
	}

	// Parse the round topology
	idList, err := id.NewIDListFromBytes(ri.Topology)
	if err != nil {
		return errors.Errorf("Could not read topology from gossip message: %s", err)
	}

	topology := connect.NewCircuit(idList)

	senderIdCopy := origin.DeepCopy()
	senderIdCopy.SetType(id.Node)

	// Check if the sender is in the round
	//  we have tracked
	if topology.GetNodeLocation(senderIdCopy) < 0 {
		return errors.Errorf("Origin gateway is not in round it's gossiping about. Gateway ID %v", origin)
	}
	return nil
}

// Receive function for Gossip messages specialized to rate limiting
func (gw *Instance) gossipRateLimitReceive(msg *gossip.GossipMsg) error {
	// Unmarshal the Sender data
	payloadMsg := &pb.BatchSenders{}
	err := proto.Unmarshal(msg.GetPayload(), payloadMsg)
	if err != nil {
		return errors.Errorf("Could not unmarshal gossip payload: %v", err)
	}

	// Add to leaky bucket for each sender
	for _, senderBytes := range payloadMsg.SenderIds {
		senderId, err := id.Unmarshal(senderBytes)
		if err != nil {
			return errors.Errorf("Could not unmarshal sender ID: %+v", err)
		}
		gw.rateLimit.LookupBucket(senderId.String()).Add(1)
	}
	return nil
}

// KillRateLimiter is a helper function which sends the kill
// signal to the gateway's rate limiter
func (gw *Instance) KillRateLimiter() {
	gw.rateLimitQuit <- struct{}{}
}

// GossipBatch builds a gossip message containing all of the sender IDs
// within the batch and gossips it to all peers
func (gw *Instance) GossipBatch(batch *pb.Batch) error {
	var err error

	// Build the message
	gossipMsg := &gossip.GossipMsg{
		Tag:    RateLimitGossip,
		Origin: gw.Comms.Id.Marshal(),
	}

	// Add the GossipMsg payload
	gossipMsg.Payload, err = buildGossipPayloadRateLimit(batch)
	if err != nil {
		return errors.Errorf("Unable to build gossip payload: %+v", err)
	}

	// Add the GossipMsg signature
	gossipMsg.Signature, err = buildGossipSignature(gossipMsg, gw.Comms.GetPrivateKey())
	if err != nil {
		return errors.Errorf("Unable to build gossip signature: %+v", err)
	}

	// Gossip the message
	gossipProtocol, ok := gw.Comms.Manager.Get(RateLimitGossip)
	if !ok {
		return errors.Errorf("Unable to get gossip protocol.")
	}
	numPeers, errs := gossipProtocol.Gossip(gossipMsg)

	// Return any errors up the stack
	if len(errs) != 0 {
		jww.TRACE.Printf("Failed to rate limit gossip to: %v", errs)
		return errors.Errorf("Could not send to %d out of %d peers", len(errs), numPeers)
	}
	return nil
}

// Helper function used to convert Batch into a GossipMsg payload
func buildGossipPayloadRateLimit(batch *pb.Batch) ([]byte, error) {
	// Nil check for the received batch
	if batch == nil || batch.Round == nil {
		return nil, errors.New("Batch does not contain necessary round info needed to gossip")
	}

	// Collect all of the sender IDs in the batch
	senderIds := make([][]byte, len(batch.Slots))
	for i, slot := range batch.Slots {
		senderIds[i] = slot.GetSenderID()
	}

	payloadMsg := &pb.BatchSenders{
		SenderIds: senderIds,
		RoundID:   batch.Round.ID,
	}
	return proto.Marshal(payloadMsg)
}
