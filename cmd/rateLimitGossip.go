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
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/primitives/rateLimiting"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/primitives/id"
)

// Initialize fields required for the gossip protocol specialized to rate limiting
func (gw *Instance) InitRateLimitGossip() {
	// Register channels for gateway add/remove events
	chanLen := 10
	gw.addGateway = make(chan network.NodeGateway, chanLen)
	gw.removeGateway = make(chan *id.ID, chanLen)
	gw.NetInf.SetAddGatewayChan(gw.addGateway)
	gw.NetInf.SetRemoveGatewayChan(gw.removeGateway)

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
	ri, err := instance.GetRound(id.Round(payloadMsg.RoundID))
	if err != nil {
		return errors.Errorf("Did not recognize round sent out by gossip message: %s", err)
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
	_, errs := gossipProtocol.Gossip(gossipMsg)

	// Return any errors up the stack
	if len(errs) != 0 {
		return errors.Errorf("Could not send to peers: %v", errs)
	}
	return nil
}

// Helper function used to convert Batch into a GossipMsg payload
func buildGossipPayloadRateLimit(batch *pb.Batch) ([]byte, error) {
	// Nil check for the received back
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
