////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"encoding/base64"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/privategrity/comms/gateway"
	pb "gitlab.com/privategrity/comms/mixmessages"
	"gitlab.com/privategrity/crypto/hash"
	"gitlab.com/privategrity/gateway/storage"
	"gitlab.com/privategrity/crypto/id"
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

// NewGatewayImpl initializes a gateway Handler interface
func NewGatewayImpl(batchSize uint64, cmixNodes []string,
	gatewayNode string) gateway.Handler {
	return gateway.Handler(&GatewayImpl{
		buffer:      storage.NewMessageBuffer(),
		batchSize:   batchSize,
		gatewayNode: gatewayNode,
		cmixNodes:   cmixNodes,
	})
}

// Returns message contents for MessageID, or a null/randomized message
// if that ID does not exist of the same size as a regular message
func (m *GatewayImpl) GetMessage(userID id.UserID, msgID string) (*pb.CmixMessage,
	bool) {
	jww.DEBUG.Printf("Getting message %q:%s from buffer...", userID, msgID)
	return m.buffer.GetMessage(userID, msgID)
}

// Return any MessageIDs in the globals for this UserID
func (m *GatewayImpl) CheckMessages(userID id.UserID, messageID string) (
	[]string, bool) {
	jww.DEBUG.Printf("Getting message IDs for %q after %s from buffer...",
		userID, messageID)
	return m.buffer.GetMessageIDs(userID, messageID)
}

// Receives batch from server and stores it in the local MessageBuffer
func (m *GatewayImpl) ReceiveBatch(msg *pb.OutputMessages) {
	jww.DEBUG.Printf("Received batch of size %d from server", len(msg.Messages))
	msgs := msg.Messages
	h, _ := hash.NewCMixHash()

	for i := range msgs {
		userId := id.UserID(msgs[i].SenderID)
		h.Write(msgs[i].MessagePayload)
		msgId := base64.StdEncoding.EncodeToString(h.Sum(nil))
		m.buffer.AddMessage(userId, msgId, msgs[i])
		h.Reset()
	}
	go PrintProfilingStatistics()
}

// PutMessage adds a message to the outgoing queue and
// calls SendBatch when it's size is the batch size
func (m *GatewayImpl) PutMessage(msg *pb.CmixMessage) bool {
	jww.DEBUG.Printf("Putting message in outgoing queue...")
	m.buffer.AddOutgoingMessage(msg)
	batch := m.buffer.PopOutgoingBatch(m.batchSize)
	if batch != nil {
		jww.DEBUG.Printf("Sending batch to %s...", m.gatewayNode)
		gateway.SendBatch(m.gatewayNode, batch)
		return true
	}
	return false
}
