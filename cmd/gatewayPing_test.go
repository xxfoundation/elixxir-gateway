///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////
package cmd

import (
	"gitlab.com/elixxir/comms/gateway"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/comms/testkeys"
	"gitlab.com/elixxir/comms/testutils"
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/primitives/states"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/crypto/large"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/ndf"
	"testing"
)

// Happy path
func TestInstance_ReportGatewayPings(t *testing.T) {
	var err error
	//Build the gateway instance
	params := Params{
		NodeAddress:           NODE_ADDRESS,
		ServerCertPath:        testkeys.GetNodeCertPath(),
		CertPath:              testkeys.GetGatewayCertPath(),
		KeyPath:               testkeys.GetGatewayKeyPath(),
		PermissioningCertPath: testkeys.GetNodeCertPath(),
		DevMode:               true,
	}

	gw := NewGatewayInstance(params)
	gw2 := NewGatewayInstance(params)

	p := large.NewIntFromString(prime, 16)
	g := large.NewIntFromString(generator, 16)
	grp2 := cyclic.NewGroup(p, g)
	addr := "0.0.0.0:8787"
	gw.Comms = gateway.StartGateway(&id.TempGateway, addr, gw,
		gatewayCert, gatewayKey, gossip.DefaultManagerFlags())

	addr2 := "0.0.0.0:7878"
	gw2.Comms = gateway.StartGateway(&id.TempGateway, addr2, gw2,
		gatewayCert, gatewayKey, gossip.DefaultManagerFlags())

	testNDF, _ := ndf.Unmarshal(ExampleJSON)

	gw.NetInf, err = network.NewInstanceTesting(gw.Comms.ProtoComms, testNDF, testNDF, grp2, grp2, t)
	if err != nil {
		t.Errorf("NewInstanceTesting encountered an error: %+v", err)
	}

	gw2.NetInf, err = network.NewInstanceTesting(gw.Comms.ProtoComms, testNDF, testNDF, grp2, grp2, t)
	if err != nil {
		t.Errorf("NewInstanceTesting encountered an error: %+v", err)
	}

	// Add permissioning as a host
	pub := testkeys.LoadFromPath(testkeys.GetNodeCertPath())
	_, err = gw.Comms.AddHost(&id.Permissioning,
		"0.0.0.0:4200", pub, connect.GetDefaultHostParams())

	// Init comms and host
	_, err = gw.Comms.AddHost(gw.Comms.Id, addr, gatewayCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Unable to add test host: %+v", err)
	}
	_, err = gw.Comms.AddHost(gw2.Comms.Id, addr, gatewayCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Unable to add test host: %+v", err)
	}

	// Build a mock node ID for a topology
	nodeID := gw.Comms.Id.DeepCopy()
	nodeID.SetType(id.Node)
	nodeId2 := gw2.Comms.Id.DeepCopy()
	nodeId2.SetType(id.Node)
	topology := [][]byte{nodeID.Bytes(), nodeId2.Bytes()}
	// Create a fake round info to store
	roundId := uint64(10)
	ri := &pb.RoundInfo{
		ID:         roundId,
		UpdateID:   10,
		Topology:   topology,
		Timestamps: make([]uint64, states.NUM_STATES),
	}

	// Sign the round info with the mock permissioning private key
	err = testutils.SignRoundInfo(ri, t)
	if err != nil {
		t.Errorf("Error signing round info: %s", err)
	}

	// Insert the mock round into the network instance
	err = gw.NetInf.RoundUpdate(ri)
	if err != nil {
		t.Errorf("Could not place mock round: %v", err)
	}
	err = gw2.NetInf.RoundUpdate(ri)
	if err != nil {
		t.Errorf("Could not place mock round: %v", err)
	}

	request := &pb.GatewayPingRequest{
		RoundId:  roundId,
		Topology: topology,
	}

	report, err := gw.ReportGatewayPings(request)
	if err != nil {
		t.Errorf("Failure in reporting gateway pings: %v", err)
	}

	if report == nil {
		t.Errorf("Failed to get report: %v", report)
	}

	if len(report.FailedGateways) != 0 {
		t.Errorf("Got unexpected failed gateways in happy path")
	}

}

// Error path: Pinged gateway will have closed comm, causing comm error
func TestInstance_ReportGatewayPings_FailedPing(t *testing.T) {
	var err error
	//Build the gateway instance
	params := Params{
		NodeAddress:           NODE_ADDRESS,
		ServerCertPath:        testkeys.GetNodeCertPath(),
		CertPath:              testkeys.GetGatewayCertPath(),
		KeyPath:               testkeys.GetGatewayKeyPath(),
		PermissioningCertPath: testkeys.GetNodeCertPath(),
		DevMode:               true,
	}

	gw := NewGatewayInstance(params)
	gw2 := NewGatewayInstance(params)

	p := large.NewIntFromString(prime, 16)
	g := large.NewIntFromString(generator, 16)
	grp2 := cyclic.NewGroup(p, g)
	addr := "0.0.0.0:8787"
	gw.Comms = gateway.StartGateway(&id.TempGateway, addr, gw,
		gatewayCert, gatewayKey, gossip.DefaultManagerFlags())

	addr2 := "0.0.0.0:7878"
	gw2Id := id.NewIdFromString("Gateway2", id.Gateway, t)
	gw2.Comms = gateway.StartGateway(gw2Id, addr2, gw2,
		gatewayCert, gatewayKey, gossip.DefaultManagerFlags())

	testNDF, _ := ndf.Unmarshal(ExampleJSON)

	gw.NetInf, err = network.NewInstanceTesting(gw.Comms.ProtoComms, testNDF, testNDF, grp2, grp2, t)
	if err != nil {
		t.Errorf("NewInstanceTesting encountered an error: %+v", err)
	}

	gw2.NetInf, err = network.NewInstanceTesting(gw.Comms.ProtoComms, testNDF, testNDF, grp2, grp2, t)
	if err != nil {
		t.Errorf("NewInstanceTesting encountered an error: %+v", err)
	}

	// Add permissioning as a host
	pub := testkeys.LoadFromPath(testkeys.GetNodeCertPath())
	_, err = gw.Comms.AddHost(&id.Permissioning,
		"0.0.0.0:4200", pub, connect.GetDefaultHostParams())

	// Init comms and host
	_, err = gw.Comms.AddHost(gw.Comms.Id, addr, gatewayCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Unable to add test host: %+v", err)
	}

	// Create an un-contactable host for gw2
	_, err = gw.Comms.AddHost(gw2.Comms.Id, "", gatewayCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Unable to add test host: %+v", err)
	}

	// Build a mock node ID for a topology
	nodeID := gw.Comms.Id.DeepCopy()
	nodeID.SetType(id.Node)
	nodeId2 := gw2.Comms.Id.DeepCopy()
	nodeId2.SetType(id.Node)
	topology := [][]byte{nodeID.Bytes(), nodeId2.Bytes()}
	// Create a fake round info to store
	roundId := uint64(10)
	ri := &pb.RoundInfo{
		ID:         roundId,
		UpdateID:   10,
		Topology:   topology,
		Timestamps: make([]uint64, states.NUM_STATES),
	}

	// Sign the round info with the mock permissioning private key
	err = testutils.SignRoundInfo(ri, t)
	if err != nil {
		t.Errorf("Error signing round info: %s", err)
	}

	// Insert the mock round into the network instance
	err = gw.NetInf.RoundUpdate(ri)
	if err != nil {
		t.Errorf("Could not place mock round: %v", err)
	}
	err = gw2.NetInf.RoundUpdate(ri)
	if err != nil {
		t.Errorf("Could not place mock round: %v", err)
	}

	request := &pb.GatewayPingRequest{
		RoundId:  roundId,
		Topology: topology,
	}

	report, err := gw.ReportGatewayPings(request)
	if err != nil {
		t.Errorf("Failure in reporting gateway pings: %v", err)
	}

	if report == nil {
		t.Errorf("Failed to get report: %v", report)
	}

	if len(report.FailedGateways) != 1 {
		t.Errorf("Did not received failed gateway in error path")
	}

}
