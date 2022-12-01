////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/comms/gateway"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/comms/network/dataStructures"
	"gitlab.com/elixxir/comms/testkeys"
	"gitlab.com/elixxir/comms/testutils"
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/elixxir/primitives/states"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/crypto/large"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gitlab.com/xx_network/primitives/ndf"
	"gitlab.com/xx_network/primitives/rateLimiting"
	"strconv"
	"testing"
	"time"
)

// Happy path
func TestInstance_GossipReceive_RateLimit(t *testing.T) {
	gatewayInstance.InitRateLimitGossip()
	var err error

	// Create a fake round info
	ri := &pb.RoundInfo{
		ID:       10,
		UpdateID: 10,
	}

	// Sign the round info with the mock permissioning private key
	err = testutils.SignRoundInfoRsa(ri, t)
	if err != nil {
		t.Errorf("Error signing round info: %s", err)
	}

	numsender := 100
	// Build a test batch
	senderIDs := make([]*id.ID, 0, numsender)

	for i := 0; i < numsender; i++ {
		senderId := id.NewIdFromString(fmt.Sprintf("%d", i), id.User, t)
		senderIDs = append(senderIDs, senderId)
	}

	// Build a test gossip message
	gossipMsg := &gossip.GossipMsg{}
	gossipMsg.Payload, err = buildGossipPayloadRateLimit(10, senderIDs, []string{})
	if err != nil {
		t.Errorf("Unable to build gossip payload: %+v", err)
	}

	// Test the gossipRateLimitReceive function
	err = gatewayInstance.gossipRateLimitReceive(gossipMsg)
	if err != nil {
		t.Errorf("Unable to receive gossip message: %+v", err)
	}

	// Ensure the buckets were populated
	for _, senderID := range senderIDs {
		bucket := gatewayInstance.idRateLimiting.LookupBucket(senderID.String())
		if bucket.Remaining() == 0 {
			t.Errorf("Failed to add to leaky bucket for sender %s", senderID.String())
		}
	}

}

// Happy path
func TestInstance_GossipVerify(t *testing.T) {
	// Build the gateway instance
	params := Params{
		NodeAddress:           NODE_ADDRESS,
		ServerCertPath:        testkeys.GetNodeCertPath(),
		CertPath:              testkeys.GetGatewayCertPath(),
		KeyPath:               testkeys.GetGatewayKeyPath(),
		PermissioningCertPath: testkeys.GetNodeCertPath(),
		DevMode:               true,
	}
	params.messageRateLimitParams = &rateLimiting.MapParams{
		Capacity:     10,
		LeakedTokens: 1,
		LeakDuration: 10 * time.Second,
		PollDuration: 10 * time.Second,
		BucketMaxAge: 10 * time.Second,
	}

	params.rateLimitParams = &rateLimiting.MapParams{
		Capacity:     capacity,
		LeakedTokens: leakedTokens,
		LeakDuration: leakDuration,
		PollDuration: pollDuration,
		BucketMaxAge: bucketMaxAge,
	}

	gw := NewGatewayInstance(params)
	p := large.NewIntFromString(prime, 16)
	g := large.NewIntFromString(generator, 16)
	grp2 := cyclic.NewGroup(p, g)
	gwID := id.NewIdFromString("Samus", id.Gateway, t)

	gw.Comms = gateway.StartGateway(gwID, "0.0.0.0:11690", gw,
		gatewayCert, gatewayKey, gossip.DefaultManagerFlags())

	testNDF, _ := ndf.Unmarshal(ExampleJSON)

	var err error
	gw.NetInf, err = network.NewInstanceTesting(gw.Comms.ProtoComms, testNDF, testNDF, grp2, grp2, t)
	if err != nil {
		t.Errorf("NewInstanceTesting encountered an error: %+v", err)
	}

	gw.InitRateLimitGossip()

	// Add permissioning as a host
	pub := testkeys.LoadFromPath(testkeys.GetNodeCertPath())
	_, err = gw.Comms.AddHost(&id.Permissioning,
		"0.0.0.0:4200", pub, connect.GetDefaultHostParams())

	privKey, err := testutils.LoadPrivateKeyTesting(t)
	if err != nil {
		t.Errorf("Could not load public key: %v", err)
		t.FailNow()
	}
	publicKey := privKey.GetPublic()

	originId := id.NewIdFromString("test", id.Gateway, t)

	// Build a mock node ID for a topology
	idCopy := originId.DeepCopy()
	idCopy.SetType(id.Node)
	topology := [][]byte{idCopy.Bytes()}

	// Create a fake round info to store
	ri := &pb.RoundInfo{
		ID:         10,
		UpdateID:   10,
		Topology:   topology,
		Timestamps: []uint64{0, 1, 2, 3, 4, 5},
	}

	// Sign the round info with the mock permissioning private key
	err = testutils.SignRoundInfoRsa(ri, t)
	if err != nil {
		t.Errorf("Error signing round info: %s", err)
	}

	// Insert the mock round into the network instance
	_, err = gw.NetInf.RoundUpdate(ri)
	if err != nil {
		t.Errorf("Could not place mock round: %v", err)
	}

	// ----------- Rate Limit Check ---------------------

	// Build the mock message
	payloadMsgRateLimit := &pb.BatchSenders{
		SenderIds: topology,
		RoundID:   10,
	}

	// Marshal the payload for the gossip message
	payload, err := proto.Marshal(payloadMsgRateLimit)
	if err != nil {
		t.Errorf("Could not marshal mock message: %s", err)
	}

	// Build a test gossip message
	gossipMsg := &gossip.GossipMsg{
		Tag:     RateLimitGossip,
		Origin:  originId.Marshal(),
		Payload: payload,
	}
	gossipMsg.Signature, err = buildGossipSignature(gossipMsg, gw.Comms.GetPrivateKey())

	// Set up origin host
	_, err = gw.Comms.AddHost(originId, "", gatewayCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Unable to add test host: %+v", err)
	}

	// Test the gossipVerify function
	err = gw.gossipVerify(gossipMsg, nil)
	if err != nil {
		t.Errorf("Unable to verify gossip message: %+v", err)
	}

	// ----------- Bloom Filter Check ---------------------
	// Build the mock message
	payloadMsgBloom := &pb.Recipients{
		RecipientIds: topology,
		RoundID:      10,
	}

	// Marshal the payload for the gossip message
	payload, err = proto.Marshal(payloadMsgBloom)
	if err != nil {
		t.Errorf("Could not marshal mock message: %s", err)
	}

	// Build a test gossip message
	gossipMsg = &gossip.GossipMsg{
		Tag:     BloomFilterGossip,
		Origin:  originId.Marshal(),
		Payload: payload,
	}
	gossipMsg.Signature, err = buildGossipSignature(gossipMsg, gw.Comms.GetPrivateKey())

	go func() {
		time.Sleep(time.Millisecond)
		ri.State = uint32(states.COMPLETED)
		err = testutils.SignRoundInfoRsa(ri, t)
		rnd := dataStructures.NewRound(ri, publicKey, nil)
		gw.NetInf.GetRoundEvents().TriggerRoundEvent(rnd)
	}()

	// Test the gossipVerify function
	err = gw.gossipVerify(gossipMsg, nil)
	if err != nil {
		t.Errorf("Unable to verify gossip message: %+v", err)
	}

}

// Happy path
func TestInstance_StartPeersThread(t *testing.T) {
	gatewayInstance.addGateway = make(chan network.NodeGateway, gwChanLen)
	gatewayInstance.removeGateway = make(chan *id.ID, gwChanLen)
	gatewayInstance.InitRateLimitGossip()
	gatewayInstance.InitBloomGossip()
	var err error

	// Prepare values and host
	gwId := id.NewIdFromString("test", id.Gateway, t)
	testSignal := network.NodeGateway{
		Gateway: ndf.Gateway{
			ID: gwId.Marshal(),
		},
	}
	_, err = gatewayInstance.Comms.AddHost(gwId, "0.0.0.0", gatewayCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Unable to add test host: %+v", err)
	}
	protocol, exists := gatewayInstance.Comms.Manager.Get(BloomFilterGossip)
	if !exists {
		t.Errorf("Unable to get gossip protocol!")
		return
	}

	// Start the channel monitor
	gatewayInstance.StartPeersThread()

	// Send the add gateway signal
	gatewayInstance.addGateway <- testSignal
	time.Sleep(1 * time.Second)

	// Test the add gateway signals
	// by attempting to remove the added gateway
	err = protocol.RemoveGossipPeer(gwId)
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
	// time.Sleep(100 * time.Millisecond)
	// err = protocol.RemoveGossipPeer(gwId)
	// if err == nil {
	// 	t.Errorf("Expected failure to remove already-removed peer!")
	// }
}

func TestInstance_GossipBatch(t *testing.T) {
	// Build the gateway instance
	params := Params{
		NodeAddress:           NODE_ADDRESS,
		ServerCertPath:        testkeys.GetNodeCertPath(),
		CertPath:              testkeys.GetGatewayCertPath(),
		KeyPath:               testkeys.GetGatewayKeyPath(),
		PermissioningCertPath: testkeys.GetNodeCertPath(),
		DevMode:               true,
	}

	params.messageRateLimitParams = &rateLimiting.MapParams{
		Capacity:     10,
		LeakedTokens: 1,
		LeakDuration: 10 * time.Second,
		PollDuration: 10 * time.Second,
		BucketMaxAge: 10 * time.Second,
	}
	params.rateLimitParams = &rateLimiting.MapParams{
		Capacity:     capacity,
		LeakedTokens: leakedTokens,
		LeakDuration: leakDuration,
		PollDuration: pollDuration,
		BucketMaxAge: bucketMaxAge,
	}

	gw := NewGatewayInstance(params)
	p := large.NewIntFromString(prime, 16)
	g := large.NewIntFromString(generator, 16)
	grp2 := cyclic.NewGroup(p, g)
	addr := "0.0.0.0:6666"
	gwID := id.NewIdFromString("Samus", id.Gateway, t)
	gw.Comms = gateway.StartGateway(gwID, addr, gw,
		gatewayCert, gatewayKey, gossip.DefaultManagerFlags())
	var err error
	testNDF, _ := ndf.Unmarshal(ExampleJSON)

	gw.NetInf, err = network.NewInstanceTesting(gw.Comms.ProtoComms, testNDF, testNDF, grp2, grp2, t)
	if err != nil {
		t.Errorf("NewInstanceTesting encountered an error: %+v", err)
	}

	// Initialize leaky bucket
	gw.rateLimitQuit = make(chan struct{}, 1)
	fmt.Printf("gwComms: %v\n", gw.Comms)
	fmt.Printf("gwManager: %v\n", gw.Comms.Manager)
	flags := gossip.DefaultProtocolFlags()
	flags.SelfGossip = true
	// Register gossip protocol for client rate limiting
	gw.Comms.Manager.NewGossip(RateLimitGossip, flags,
		gw.gossipRateLimitReceive, gw.gossipVerify, nil)

	// Add permissioning as a host
	pub := testkeys.LoadFromPath(testkeys.GetNodeCertPath())
	_, err = gw.Comms.AddHost(&id.Permissioning,
		"0.0.0.0:4200", pub, connect.GetDefaultHostParams())

	// Init comms and host
	_, err = gw.Comms.AddHost(gw.Comms.GetId(), addr, gatewayCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Unable to add test host: %+v", err)
	}
	protocol, exists := gw.Comms.Manager.Get(RateLimitGossip)
	if !exists {
		t.Errorf("Unable to get gossip protocol!")
		return
	}
	err = protocol.AddGossipPeer(gw.Comms.GetId())
	if err != nil {
		t.Errorf("Unable to add gossip peer: %+v", err)
	}
	// Build a mock node ID for a topology
	nodeID := gw.Comms.GetId()
	nodeID.SetType(id.Node)
	topology := [][]byte{nodeID.Bytes()}
	// Create a fake round info to store
	ts := uint64(time.Now().UnixNano())
	ri := &pb.RoundInfo{
		ID:         10,
		UpdateID:   10,
		Topology:   topology,
		Timestamps: []uint64{ts, ts, ts, ts, ts, ts, ts},
	}
	fmt.Printf("nodeID: %v\n", nodeID)

	// Sign the round info with the mock permissioning private key
	err = testutils.SignRoundInfoRsa(ri, t)
	if err != nil {
		t.Errorf("Error signing round info: %s", err)
	}

	// Insert the mock round into the network instance
	_, err = gw.NetInf.RoundUpdate(ri)
	if err != nil {
		t.Errorf("Could not place mock round: %v", err)
	}

	numsenders := 100

	// Build a test batch
	senderIDs := make([]*id.ID, 0, numsenders)
	ips := make([]string, 0, numsenders)

	for i := 0; i < numsenders; i++ {
		senderId := id.NewIdFromString(fmt.Sprintf("%d", i), id.User, t)
		senderIDs = append(senderIDs, senderId)
		ip := fmt.Sprintf("%d.%d.%d.%d", i, i, i, i)
		ips = append(ips, ip)
	}
	batch := &pb.Batch{
		Round: ri,
		Slots: make([]*pb.Slot, 10),
	}
	for i := 0; i < len(batch.Slots); i++ {
		senderId := id.NewIdFromString(strconv.Itoa(i), id.User, t)
		batch.Slots[i] = &pb.Slot{SenderID: senderId.Marshal()}
	}

	// Send the gossip
	err = gw.GossipBatch(id.Round(ri.ID), senderIDs, ips)
	if err != nil {
		t.Errorf("Unable to gossip: %+v", err)
	}
	time.Sleep(1 * time.Millisecond)

	// Verify the gossip was received
	testSenderId := id.NewIdFromString("0", id.User, t)
	if remaining := gw.idRateLimiting.LookupBucket(testSenderId.String()).Remaining(); remaining != 1 {
		t.Errorf("Expected to reduce remaining message count for test sender, got %d", remaining)
	}
}

func TestInstance_GossipBloom(t *testing.T) {
	jwalterweatherman.SetLogThreshold(jwalterweatherman.LevelDebug)
	// Build the gateway instance
	params := Params{
		NodeAddress:           NODE_ADDRESS,
		ServerCertPath:        testkeys.GetNodeCertPath(),
		CertPath:              testkeys.GetGatewayCertPath(),
		KeyPath:               testkeys.GetGatewayKeyPath(),
		PermissioningCertPath: testkeys.GetNodeCertPath(),
		DevMode:               true,
	}

	params.messageRateLimitParams = &rateLimiting.MapParams{
		Capacity:     10,
		LeakedTokens: 1,
		LeakDuration: 10 * time.Second,
		PollDuration: 10 * time.Second,
		BucketMaxAge: 10 * time.Second,
	}

	params.rateLimitParams = &rateLimiting.MapParams{
		Capacity:     capacity,
		LeakedTokens: leakedTokens,
		LeakDuration: leakDuration,
		PollDuration: pollDuration,
		BucketMaxAge: bucketMaxAge,
	}

	gw := NewGatewayInstance(params)
	err := gw.SetPeriod()
	if err != nil {
		t.Errorf(err.Error())
	}
	p := large.NewIntFromString(prime, 16)
	g := large.NewIntFromString(generator, 16)
	grp2 := cyclic.NewGroup(p, g)
	addr := "0.0.0.0:7777"
	gwID := id.NewIdFromString("Samus", id.Gateway, t)
	gw.Comms = gateway.StartGateway(gwID, addr, gw,
		gatewayCert, gatewayKey, gossip.DefaultManagerFlags())
	testNDF, _ := ndf.Unmarshal(ExampleJSON)

	gw.NetInf, err = network.NewInstanceTesting(gw.Comms.ProtoComms, testNDF, testNDF, grp2, grp2, t)
	if err != nil {
		t.Errorf("NewInstanceTesting encountered an error: %+v", err)
	}

	rndId := uint64(10)
	flags := gossip.DefaultProtocolFlags()
	flags.FanOut = 25
	flags.MaximumReSends = 2
	flags.NumParallelSends = 150
	flags.SelfGossip = true
	// Register gossip protocol for bloom filters
	gw.Comms.Manager.NewGossip(BloomFilterGossip, flags,
		gw.gossipBloomFilterReceive, gw.gossipVerify, nil)

	// Add permissioning as a host
	pub := testkeys.LoadFromPath(testkeys.GetNodeCertPath())
	_, err = gw.Comms.AddHost(&id.Permissioning,
		"0.0.0.0:4200", pub, connect.GetDefaultHostParams())

	// Init comms and host
	_, err = gw.Comms.AddHost(gw.Comms.GetId(), addr, gatewayCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Unable to add test host: %+v", err)
	}
	protocol, exists := gw.Comms.Manager.Get(BloomFilterGossip)
	if !exists {
		t.Errorf("Unable to get gossip protocol!")
		return
	}
	err = protocol.AddGossipPeer(gw.Comms.GetId())
	if err != nil {
		t.Errorf("Unable to add gossip peer: %+v", err)
	}

	// Build a mock node ID for a topology
	nodeID := gw.Comms.GetId()
	nodeID.SetType(id.Node)
	topology := [][]byte{nodeID.Bytes()}
	// Create a fake round info to store
	ts := uint64(time.Now().UnixNano())
	ri := &pb.RoundInfo{
		ID:         rndId,
		UpdateID:   10,
		Topology:   topology,
		Timestamps: []uint64{ts, ts, ts, ts, ts, ts, ts},
	}

	// Sign the round info with the mock permissioning private key
	err = testutils.SignRoundInfoRsa(ri, t)
	if err != nil {
		t.Errorf("Error signing round info: %s", err)
	}

	// Insert the mock round into the network instance
	_, err = gw.NetInf.RoundUpdate(ri)
	if err != nil {
		t.Errorf("Could not place mock round: %v", err)
	}

	clients := make(map[id.ID]interface{})
	for i := uint64(0); i < 10; i++ {
		tempId := id.NewIdFromUInt(i, id.User, t)
		clients[*tempId] = nil
	}

	// Insert the first five IDs as known clients
	i := 0
	ephIds := make(map[ephemeral.Id]interface{}, len(clients))
	for client := range clients {
		mockClient := &storage.Client{
			Id: client.Bytes(),
		}
		err := gw.storage.UpsertClient(mockClient)
		if err != nil {
			t.Errorf("%+v", err)
		}
		testEphId, _, _, err := ephemeral.GetId(&client, 64, time.Now().UnixNano())
		if err != nil {
			t.Errorf("Could not create an ephemeral id: %v", err)
		}
		ephIds[testEphId] = nil
		i++
		if i == 5 {
			break
		}
	}

	privKey, err := testutils.LoadPrivateKeyTesting(t)
	if err != nil {
		t.Errorf("Could not load public key: %v", err)
		t.FailNow()
	}
	publicKey := privKey.GetPublic()

	// Send the gossip
	go func() {
		time.Sleep(250 * time.Millisecond)
		ri.State = uint32(states.COMPLETED)
		testutils.SignRoundInfoRsa(ri, t)
		rnd := dataStructures.NewRound(ri, publicKey, nil)
		gw.NetInf.GetRoundEvents().TriggerRoundEvent(rnd)
	}()
	err = gw.GossipBloom(ephIds, id.Round(rndId), int64(ts))
	if err != nil {
		t.Errorf("Unable to gossip: %+v", err)
	}
	time.Sleep(2 * time.Second)

	i = 0
	err = gw.SetPeriod()
	if err != nil {
		t.Errorf(err.Error())
	}
	round, err := gw.NetInf.GetRound(id.Round(rndId))
	if err != nil {
		t.Fatalf(err.Error())
	}
	testEpoch := GetEpoch(int64(round.Timestamps[states.QUEUED]), gw.period)
	for clientId := range ephIds {
		// Check that the first five IDs are known clients, and thus
		// in the user bloom filter
		filters, err := gw.storage.GetClientBloomFilters(clientId, testEpoch, testEpoch)
		if err != nil || filters == nil {
			t.Errorf("Could not get a bloom filter for user %d with ID %d", i, clientId.Int64())
		}
		i++
	}
}
