///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

//  Contains gossip methods specific to rate limit gossiping

package cmd

import (
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/gateway/cmd/ipAddress"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/primitives/id"
	"google.golang.org/protobuf/proto"
)

// Initialize fields required for the gossip protocol specialized to rate limiting
func (gw *Instance) InitRateLimitGossip() {

	flags := gossip.DefaultProtocolFlags()
	flags.FanOut = 4
	flags.MaximumReSends = 2
	flags.NumParallelSends = 1000
	flags.SelfGossip = false

	// Register gossip protocol for bloom filters
	gw.Comms.Manager.NewGossip(RateLimitGossip, flags,
		gw.gossipRateLimitReceive, gw.gossipVerify, nil)

}

// Receive function for Gossip messages specialized to rate limiting
func (gw *Instance) gossipRateLimitReceive(msg *gossip.GossipMsg) error {
	// Unmarshal the Sender data
	payloadMsg := &pb.BatchSenders{}
	err := proto.Unmarshal(msg.GetPayload(), payloadMsg)
	if err != nil {
		return errors.Errorf("Could not unmarshal gossip payload: %v", err)
	}

	capacity, leaked, duration := gw.GetRateLimitParams()

	// Add to leaky bucket for each sender
	for _, senderBytes := range payloadMsg.SenderIds {
		senderId, err := id.Unmarshal(senderBytes)
		if err != nil {
			return errors.Errorf("Could not unmarshal sender ID: %+v", err)
		}
		gw.idRateLimiting.LookupBucket(senderId.String()).AddWithExternalParams(1, capacity, leaked, duration)
	}
	for _, ipBytes := range payloadMsg.Ips {
		ipStr, err := ipAddress.ByteToString(ipBytes)
		if err != nil {
			jww.WARN.Printf("round %d rate limit gossip sent "+
				"an invalid ip addr %v: %s", payloadMsg.RoundID, ipBytes, err)
		} else {
			gw.idRateLimiting.LookupBucket(ipStr).AddWithExternalParams(1, capacity, leaked, duration)
		}

	}
	return nil
}

// GossipBatch builds a gossip message containing all of the sender IDs
// within the batch and gossips it to all peers
func (gw *Instance) GossipBatch(round id.Round, senders []*id.ID, ips []string) error {
	var err error

	// Build the message
	gossipMsg := &gossip.GossipMsg{
		Tag:    RateLimitGossip,
		Origin: gw.Comms.Id.Marshal(),
	}

	// Add the GossipMsg payload
	gossipMsg.Payload, err = buildGossipPayloadRateLimit(round, senders, ips)
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
func buildGossipPayloadRateLimit(round id.Round, senders []*id.ID, ips []string) ([]byte, error) {
	// Nil check for the received back
	if senders == nil || ips == nil {
		return nil, errors.New("Batch does not contain necessary round info needed to gossip")
	}

	// Collect all of the sender IDs in the batch
	ipsBytesSlice := make([][]byte, 0, len(ips))
	for _, ipStr := range ips {
		ipsBytes, err := ipAddress.StringToByte(ipStr)
		if err != nil {
			jww.WARN.Printf("ip %s failed to get added for round %d"+
				" because : %s", ipStr, round, err)
		} else {
			ipsBytesSlice = append(ipsBytesSlice, ipsBytes)
		}
	}

	sendersByteSlice := make([][]byte, 0, len(senders))
	for _, sID := range senders {
		sendersByteSlice = append(sendersByteSlice, sID.Marshal())
	}

	payloadMsg := &pb.BatchSenders{
		SenderIds: sendersByteSlice,
		Ips:       ipsBytesSlice,
		RoundID:   uint64(round),
	}
	return proto.Marshal(payloadMsg)
}
