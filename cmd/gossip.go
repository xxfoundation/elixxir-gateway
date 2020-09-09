///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"encoding/json"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/primitives/id"
)

// Tag for the client rate limit gossip protocol
const RateLimitGossip = "clientRateLimit"

// Initialize fields required for the gossip protocol
func (gw *Instance) InitGossip() {
	// Register channels for gateway add/remove events
	chanLen := 10
	gw.addGateway = make(chan network.NodeGateway, chanLen)
	gw.removeGateway = make(chan *id.ID, chanLen)
	gw.NetInf.SetAddGatewayChan(gw.addGateway)
	gw.NetInf.SetRemoveGatewayChan(gw.removeGateway)

	// Register gossip protocol for client rate limiting
	gw.Comms.Manager.NewGossip(RateLimitGossip, gossip.DefaultProtocolFlags(),
		func(msg *gossip.GossipMsg) error {
			// Receive function
			// TODO
			return nil
		}, func(msg *gossip.GossipMsg, ignore []byte) error {
			// Verification function
			// TODO
			return nil
		}, make([]*id.ID, 0))
}

// KillRateLimiter is a helper function which sends the kill
// signal to the gateway's rate limiter
func (gw *Instance) KillRateLimiter() {
	gw.rateLimitQuit <- struct{}{}
}

// Starts a thread for monitoring and handling changes to gossip peers
func (gw *Instance) StartPeersThread() {
	go func() {
		protocol, exists := gw.Comms.Manager.Get(RateLimitGossip)
		if !exists {
			jww.WARN.Printf("Unable to get gossip protocol!")
			return
		}
		for {
			select {
			case removeId := <-gw.removeGateway:
				err := protocol.RemoveGossipPeer(removeId)
				if err != nil {
					jww.WARN.Printf("Unable to remove gossip peer: %+v", err)
				}
			case add := <-gw.addGateway:
				gwId, err := id.Unmarshal(add.Gateway.ID)
				if err != nil {
					jww.WARN.Printf("Unable to unmarshal gossip peer: %+v", err)
					continue
				}
				err = protocol.AddGossipPeer(gwId)
				if err != nil {
					jww.WARN.Printf("Unable to add gossip peer: %+v", err)
				}
			}
		}
	}()
}

// gossipBatch builds a gossip message containing all of the sender ID's
// within the batch and gossips it to all peers
func (gw *Instance) GossipBatch(batch *pb.Batch) error {

	gossipProtocol, ok := gw.Comms.Manager.Get(RateLimitGossip)
	if !ok {
		return errors.Errorf("Unable to get gossip protocol.")
	}

	// Collect all of the sender IDs in the batch
	var senderIds []*id.ID
	for i, slot := range batch.Slots {
		sender, err := id.Unmarshal(slot.SenderID)
		if err != nil {
			return errors.Errorf("Could not completely "+
				"gossip for slot %d in round %d: "+
				"Unreadable sender ID: %v: %v",
				i, batch.Round.ID, slot.SenderID, err)
		}

		senderIds = append(senderIds, sender)
	}

	// Marshal the list of senders into a json
	payloadData, err := json.Marshal(senderIds)
	if err != nil {
		return errors.Errorf("Could not form gossip payload: %v", err)
	}

	// Build the message
	gossipMsg := &gossip.GossipMsg{
		Tag:     RateLimitGossip,
		Origin:  gw.Comms.Id.Marshal(),
		Payload: payloadData,
	}

	// Sign the gateway message
	gossipMsg.Signature, err = gw.Comms.SignMessage(gossipMsg)
	if err != nil {
		return errors.Errorf("Could not sign gossip mesage: %v", err)
	}

	// Gossip the message
	_, errs := gossipProtocol.Gossip(gossipMsg)

	// Return any errors up the stack
	if len(errs) != 0 {
		return errors.Errorf("Could not send to peers: %v", errs)
	}
	return nil
}
