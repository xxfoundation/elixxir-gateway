////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"gitlab.com/privategrity/comms/gateway"
	pb "gitlab.com/privategrity/comms/mixmessages"
	"gitlab.com/privategrity/gateway/storage"
	"os"
	"testing"
	"gitlab.com/privategrity/crypto/id"
)

const GW_ADDRESS = "localhost:5555"

var gatewayInterface gateway.Handler

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
	msg := pb.CmixMessage{SenderID: string(id.NewUserIDFromUint(666, t))}
	userId := id.ZeroID
	// msgId := "msg1"

	ok := gatewayInterface.PutMessage(&msg)
	if !ok {
		t.Errorf("PutMessage: Could not put any messages!")
	}

	_, ok = gatewayInterface.CheckMessages(userId, "")

	if ok {
		t.Errorf("CheckMessages: Expected no messages!")
	}
}
