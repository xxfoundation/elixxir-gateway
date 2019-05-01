////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"gitlab.com/elixxir/comms/gateway"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/elixxir/primitives/id"
	"os"
	"testing"
)

const GW_ADDRESS = "localhost:5555"

var gatewayInterface gateway.Handler

// This sets up a dummy/mock globals instance for testing purposes
func TestMain(m *testing.M) {
	cmixNodes := make([]string, 1)
	cmixNodes[0] = GW_ADDRESS
	gatewayInterface = &GatewayImpl{
		Buffer:      storage.NewMessageBuffer(),
		BatchSize:   1,
		GatewayNode: GW_ADDRESS,
		cmixNodes:   cmixNodes,
	}
	go gateway.StartGateway(GW_ADDRESS, gatewayInterface, "", "")
	os.Exit(m.Run())
}

func TestGatewayImpl(t *testing.T) {
	msg := pb.CmixMessage{SenderID: id.NewUserFromUint(666, t).Bytes()}
	userId := id.ZeroID

	ok := gatewayInterface.PutMessage(&msg)
	if !ok {
		t.Errorf("PutMessage: Could not put any messages!")
	}

	_, ok = gatewayInterface.CheckMessages(userId, "")

	if ok {
		t.Errorf("CheckMessages: Expected no messages!")
	}
}
