///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package cmd

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/comms/testkeys"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/xx_network/primitives/ndf"
	"testing"
)

// Error path: Pass in invalid messages
func TestInstance_Poll_NilCheck(t *testing.T) {
	// Build the gateway instance
	params := Params{
		NodeAddress:    NODE_ADDRESS,
		ServerCertPath: testkeys.GetNodeCertPath(),
		CertPath:       testkeys.GetGatewayCertPath(),
		DevMode:        true,
	}

	gw := NewGatewayInstance(params)
	gw.InitNetwork()

	// Pass in a nil client ID
	clientReq := &pb.GatewayPoll{
		Partial:     nil,
		LastUpdate:  0,
		ReceptionID: nil,
	}

	testNDF, _ := ndf.Unmarshal(ExampleJSON)

	// This is bad. It needs to be fixed (Ben's fault for not fixing correctly)
	var err error
	ers := &storage.Storage{}
	gw.NetInf, err = network.NewInstance(gatewayInstance.Comms.ProtoComms, testNDF, testNDF, ers, network.Lazy)

	_, err = gw.Poll(clientReq)
	if err == nil {
		t.Errorf("Expected error path. Should error when passing a nil clientID")
	}

	// Pass in a completely nil message
	_, err = gw.Poll(nil)
	if err == nil {
		t.Errorf("Expected error path. Should error when passing a nil message")
	}
}

// Happy path
//func TestInstance_Poll(t *testing.T) {
//	//Build the gateway instance
//	params := Params{
//		NodeAddress:     NODE_ADDRESS,
//		ServerCertPath:  testkeys.GetNodeCertPath(),
//		CertPath:        testkeys.GetGatewayCertPath(),
//	}
//
//	gw := NewGatewayInstance(params)
//	gw.InitNetwork()
//	gw.period = 30
//
//	clientId := id.NewIdFromBytes([]byte("test"), t)
//	ephemId, _, _, err := ephemeral.GetId(clientId, 8, time.Now().UnixNano())
//
//	clientReq := &pb.GatewayPoll{
//		Partial:    nil,
//		LastUpdate: 0,
//		ReceptionID:   ephemId[:],
//	}
//	testNDF, _ := ndf.Unmarshal(ExampleJSON)
//
//	// This is bad. It needs to be fixed (Ben's fault for not fixing correctly)
//	ers := &storage.Storage{}
//	gw.NetInf, err = network.NewInstance(gatewayInstance.Comms.ProtoComms, testNDF, testNDF, ers)
//
//	// TODO: Remove this when jake fixes the database please [Insert deity]
//	// Setup a database based on a map impl
//	gw.storage, _ = storage.NewStorage("", "", "", "", "")
//
//	_, err = gw.Poll(clientReq)
//	if err != nil {
//		t.Errorf("Failed to poll: %v", err)
//	}
//
//}
