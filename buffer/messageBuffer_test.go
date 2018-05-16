////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package buffer

import (
	"gitlab.com/privategrity/comms/gateway"
	pb "gitlab.com/privategrity/comms/mixmessages"
	"os"
	"testing"
)

const GW_ADDRESS = "localhost:5555"

// This sets up a dummy/mock gateway instance for testing purposes
func TestMain(m *testing.M) {
	BATCH_SIZE = 1
	GATEWAY_NODE = GW_ADDRESS
	go gateway.StartGateway(GW_ADDRESS, TestInterface{})
	os.Exit(m.Run())
}

// Blank struct implementing GatewayHandler interface for testing purposes
// (Passing to StartGateway)
type TestInterface struct{}

func (m TestInterface) GetMessage(userId uint64,
	msgId string) (*pb.CmixMessage, bool) {
	return &pb.CmixMessage{}, true
}

func (m TestInterface) CheckMessages(userId uint64) ([]string, bool) {
	return make([]string, 0), true
}

func (m TestInterface) PutMessage(message *pb.CmixMessage) bool {
	return true
}

func TestMapBuffer(t *testing.T) {
	buffer := MapBuffer{
		messageCollection: make(map[uint64]map[string]*pb.CmixMessage),
	}
	msg := pb.CmixMessage{SenderID: uint64(666)}
	userId := uint64(0)
	msgId := "msg1"

	ok := buffer.PutMessage(&msg)
	if !ok {
		t.Errorf("PutMessage: Could not put any messages!")
	}

	_, ok = buffer.CheckMessages(userId)

	if ok {
		t.Errorf("CheckMessages: Expected no messages!")
	}

	buffer.AddMessage(userId, msgId, &msg)

	msgIds, ok := buffer.CheckMessages(userId)

	if !ok || msgIds[0] != msgId {
		t.Errorf("CheckMessages: Expected to find a message!")
	}

	otherMsg, ok := buffer.GetMessage(userId, msgIds[0])

	if !ok || msg.SenderID != otherMsg.SenderID {
		t.Errorf("GetMessage: Retrieved wrong message!")
	}

	buffer.DeleteMessage(userId, msgId)
	otherMsg, ok = buffer.GetMessage(userId, msgId)

	if ok {
		t.Errorf("DeleteMessage: Failed to delete message!")
	}
}
