////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"encoding/base64"
	"gitlab.com/privategrity/comms/gateway"
	pb "gitlab.com/privategrity/comms/mixmessages"
	"gitlab.com/privategrity/crypto/hash"
	"gitlab.com/privategrity/gateway/storage"
)

type GatewayImpl struct {
	// Storage buffer for inbound/outbound messages
	buffer storage.MessageBuffer
	// The Address of the cMix nodes to communicate with
	cmixNodes []string
	// The address of my cMix Node
	gatewayNode string
	// The batch size of the cMix network
	batchSize uint64
}

// Initialize a GatewayHandler interface
func NewGatewayImpl(batchSize uint64, cmixNodes []string,
	gatewayNode string) GatewayHandler {
	return GatewayHandler(&GatewayImpl{
		buffer:      storage.NewMessageBuffer(),
		batchSize:   batchSize,
		gatewayNode: gatewayNode,
		cmixNodes:   cmixNodes,
	})
}

// Returns message contents for MessageID, or a null/randomized message
// if that ID does not exist of the same size as a regular message
func (m *GatewayImpl) GetMessage(userId uint64, msgId string) (*pb.CmixMessage,
	bool) {
	return m.buffer.GetMessage(userId, msgId)
}

// Return any MessageIDs in the globals for this UserID
func (m *GatewayImpl) CheckMessages(userId uint64) ([]string, bool) {
	return m.buffer.GetMessageIDs(userId)
}

// Receives batch from server and stores it in the local MessageBuffer
func (m *GatewayImpl) ReceiveBatch(msg *pb.OutputMessages) {
	msgs := msg.Messages
	h, _ := hash.NewCMixHash()

	for i := range msgs {
		userId := msgs[i].SenderID
		h.Write(msgs[i].MessagePayload)
		msgId := base64.StdEncoding.EncodeToString(h.Sum(nil))
		m.buffer.AddMessage(userId, msgId, msgs[i])
		h.Reset()
	}
}

// PutMessage adds a message to the outgoing queue and
// calls SendBatch when it's size is the batch size
func (m *GatewayImpl) PutMessage(msg *pb.CmixMessage) bool {
	m.buffer.AddOutgoingMessage(msg)
	batch := m.buffer.PopOutgoingBatch(m.batchSize)
	if batch != nil {
		gateway.SendBatch(m.gatewayNode, batch)
		return true
	}
	return false
}
