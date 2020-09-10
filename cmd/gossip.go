///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// Contains functionality related to inter-gateway gossip

package cmd

import (
	"crypto/rand"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/crypto/signature/rsa"
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
		// Receive function
		func(msg *gossip.GossipMsg) error {
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
		},
		// Verification function
		func(msg *gossip.GossipMsg, ignore []byte) error {
			// Locate origin host
			origin, err := id.Unmarshal(msg.Origin)
			if err != nil {
				return errors.Errorf("Unable to unmarshal origin: %+v", err)
			}
			host, exists := gw.Comms.GetHost(origin)
			if !exists {
				return errors.Errorf("Unable to locate origin host: %+v", err)
			}

			// Prepare message hash
			options := rsa.NewDefaultOptions()
			hash := options.Hash.New()
			hash.Write(gossip.Marshal(msg))
			hashed := hash.Sum(nil)

			// Verify signature of message using origin host's public key
			err = rsa.Verify(host.GetPubKey(), options.Hash, hashed, msg.Signature, nil)
			if err != nil {
				return errors.Errorf("Unable to verify signature: %+v", err)
			}
			return nil
		}, nil)
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

// GossipBatch builds a gossip message containing all of the sender IDs
// within the batch and gossips it to all peers
func (gw *Instance) GossipBatch(batch *pb.Batch) error {

	// Get the relevant GossipProtocol
	gossipProtocol, ok := gw.Comms.Manager.Get(RateLimitGossip)
	if !ok {
		return errors.Errorf("Unable to get gossip protocol.")
	}

	// Collect all of the sender IDs in the batch
	senderIds := make([][]byte, len(batch.Slots))
	for i, slot := range batch.Slots {
		senderIds[i] = slot.GetSenderID()
	}
	payloadMsg := &pb.BatchSenders{SenderIds: senderIds}
	payloadBytes, err := proto.Marshal(payloadMsg)
	if err != nil {
		return errors.Errorf("Could not marshal gossip payload: %v", err)
	}

	// Build the message
	gossipMsg := &gossip.GossipMsg{
		Tag:     RateLimitGossip,
		Origin:  gw.Comms.Id.Marshal(),
		Payload: payloadBytes,
	}

	// Hash the message
	options := rsa.NewDefaultOptions()
	hash := options.Hash.New()
	hash.Write(gossip.Marshal(gossipMsg))
	hashed := hash.Sum(nil)

	// Sign the message
	gossipMsg.Signature, err = rsa.Sign(rand.Reader, gw.Comms.GetPrivateKey(),
		options.Hash, hashed, nil)
	if err != nil {
		return errors.New(err.Error())
	}

	// Gossip the message
	_, errs := gossipProtocol.Gossip(gossipMsg)

	// Return any errors up the stack
	if len(errs) != 0 {
		return errors.Errorf("Could not send to peers: %v", errs)
	}
	return nil
}
