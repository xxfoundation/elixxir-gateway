///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"fmt"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/ndf"
	"testing"
	"time"
)

// Happy path
func TestInstance_GossipReceive(t *testing.T) {
	gatewayInstance.InitGossip()
	defer gatewayInstance.KillRateLimiter()
	var err error

	// Build a test batch
	batch := &pb.Batch{Slots: make([]*pb.Slot, 10)}
	for i := 0; i < len(batch.Slots); i++ {
		senderId := id.NewIdFromString(fmt.Sprintf("%d", i), id.User, t)
		batch.Slots[i] = &pb.Slot{SenderID: senderId.Marshal()}
	}

	// Build a test gossip message
	gossipMsg := &gossip.GossipMsg{}
	gossipMsg.Payload, err = buildGossipPayload(batch)
	if err != nil {
		t.Errorf("Unable to build gossip payload: %+v", err)
	}

	// Test the gossipReceive function
	err = gatewayInstance.gossipReceive(gossipMsg)
	if err != nil {
		t.Errorf("Unable to receive gossip message: %+v", err)
	}

	// Ensure the buckets were populated
	for _, slot := range batch.Slots {
		senderId, err := id.Unmarshal(slot.GetSenderID())
		if err != nil {
			t.Errorf("Could not unmarshal sender ID: %+v", err)
		}
		bucket := gatewayInstance.rateLimit.LookupBucket(senderId.String())
		if bucket.Remaining() == 0 {
			t.Errorf("Failed to add to leaky bucket for sender %s", senderId.String())
		}
	}
}

// Happy path
func TestInstance_GossipVerify(t *testing.T) {
	gatewayInstance.InitGossip()
	defer gatewayInstance.KillRateLimiter()
	var err error

	// Build a test gossip message
	originId := id.NewIdFromString("test", id.Gateway, t)
	gossipMsg := &gossip.GossipMsg{
		Tag:     "1",
		Origin:  originId.Marshal(),
		Payload: []byte("3"),
	}
	gossipMsg.Signature, err = buildGossipSignature(gossipMsg, gatewayInstance.Comms.GetPrivateKey())

	// Set up origin host
	_, err = gatewayInstance.Comms.AddHost(originId, "", gatewayCert, false, false)
	if err != nil {
		t.Errorf("Unable to add test host: %+v", err)
	}

	// Test the gossipReceive function
	err = gatewayInstance.gossipVerify(gossipMsg, nil)
	if err != nil {
		t.Errorf("Unable to verify gossip message: %+v", err)
	}
}

// Happy path
func TestInstance_StartPeersThread(t *testing.T) {
	gatewayInstance.InitGossip()
	defer gatewayInstance.KillRateLimiter()
	var err error

	// Prepare values and host
	gwId := id.NewIdFromString("test", id.Gateway, t)
	testSignal := network.NodeGateway{
		Gateway: ndf.Gateway{
			ID: gwId.Marshal(),
		},
	}
	_, err = gatewayInstance.Comms.AddHost(gwId, "0.0.0.0", gatewayCert, false, false)
	if err != nil {
		t.Errorf("Unable to add test host: %+v", err)
	}
	protocol, exists := gatewayInstance.Comms.Manager.Get(RateLimitGossip)
	if !exists {
		t.Errorf("Unable to get gossip protocol!")
		return
	}

	// Start the channel monitor
	gatewayInstance.StartPeersThread()

	// Send the add gateway signal
	gatewayInstance.addGateway <- testSignal

	// Test the add gateway signals
	// by attempting to remove the added gateway
	for i := 0; i < 5; i++ {
		err = protocol.RemoveGossipPeer(gwId)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		t.Errorf("Unable to remove gossip peer: %+v", err)
	}

	// Now add a peer and send a a remove signal
	err = protocol.AddGossipPeer(gwId)
	if err != nil {
		t.Errorf("Unable to add gossip peer: %+v", err)
	}
	gatewayInstance.removeGateway <- gwId

	// Test the remove gateway signals
	// by attempting to remove a gateway that should have already been removed
	time.Sleep(100 * time.Millisecond)
	err = protocol.RemoveGossipPeer(gwId)
	if err == nil {
		t.Errorf("Expected failure to remove already-removed peer!")
	}
}

//
func TestInstance_GossipBatch(t *testing.T) {
	gatewayInstance.InitGossip()
	defer gatewayInstance.KillRateLimiter()
	var err error

	// Init comms and host
	_, err = gatewayInstance.Comms.AddHost(gatewayInstance.Comms.Id, GW_ADDRESS, gatewayCert, false, false)
	if err != nil {
		t.Errorf("Unable to add test host: %+v", err)
	}
	protocol, exists := gatewayInstance.Comms.Manager.Get(RateLimitGossip)
	if !exists {
		t.Errorf("Unable to get gossip protocol!")
		return
	}
	err = protocol.AddGossipPeer(gatewayInstance.Comms.Id)
	if err != nil {
		t.Errorf("Unable to add gossip peer: %+v", err)
	}

	// Build a test batch
	batch := &pb.Batch{Slots: make([]*pb.Slot, 10)}
	for i := 0; i < len(batch.Slots); i++ {
		senderId := id.NewIdFromString(fmt.Sprintf("%d", i), id.User, t)
		batch.Slots[i] = &pb.Slot{SenderID: senderId.Marshal()}
	}

	// Send the gossip
	err = gatewayInstance.GossipBatch(batch)
	if err != nil {
		t.Errorf("Unable to gossip: %+v", err)
	}

	// Verify the gossip was received
	testSenderId := id.NewIdFromString("0", id.User, t)
	if remaining := gatewayInstance.rateLimit.LookupBucket(testSenderId.String()).Remaining(); remaining != 1 {
		t.Errorf("Expected to reduce remaining message count for test sender, got %d", remaining)
	}
}
