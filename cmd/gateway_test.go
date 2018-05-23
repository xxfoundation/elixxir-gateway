////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"gitlab.com/privategrity/comms/gateway"
	pb "gitlab.com/privategrity/comms/mixmessages"
	"os"
	"testing"
	"gitlab.com/privategrity/gateway/storage"
)

const GW_ADDRESS = "localhost:5555"
var gatewayInterface GatewayHandler

// This sets up a dummy/mock globals instance for testing purposes
func TestMain(m *testing.M) {
	cmixNodes := make([]string, 1)
	cmixNodes[0] = GW_ADDRESS
	gatewayInterface = &GatewayImpl{
		buffer:      storage.NewMessageBuffer(),
		batchSize:   1,
		gatewayNode: GW_ADDRESS,
		cmixNodes:   cmixNodes,
	}
	go gateway.StartGateway(GW_ADDRESS, gatewayInterface)
	os.Exit(m.Run())
}

func TestGatewayImpl(t *testing.T) {
	msg := pb.CmixMessage{SenderID: uint64(666)}
	userId := uint64(0)
	// msgId := "msg1"

	ok := gatewayInterface.PutMessage(&msg)
	if !ok {
		t.Errorf("PutMessage: Could not put any messages!")
	}

	_, ok = gatewayInterface.CheckMessages(userId)

	if ok {
		t.Errorf("CheckMessages: Expected no messages!")
	}
}
