////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"encoding/base64"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/comms/gateway"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/elixxir/primitives/format"
	"gitlab.com/elixxir/primitives/id"
	"time"
)

type GatewayImpl struct {
	// Storage buffer for inbound/outbound messages
	Buffer storage.MessageBuffer
	// The Address of the cMix nodes to communicate with
	cmixNodes []string
	// The address of my cMix Node
	GatewayNode string
	// The batch size of the cMix network
	BatchSize uint64
}

// NewGatewayImpl initializes a gateway Handler interface
func NewGatewayImpl(batchSize uint64, cmixNodes []string,
	gatewayNode string) *GatewayImpl {
	return &GatewayImpl{
		Buffer:      storage.NewMessageBuffer(),
		BatchSize:   batchSize,
		GatewayNode: gatewayNode,
		cmixNodes:   cmixNodes,
	}
}

// Returns message contents for MessageID, or a null/randomized message
// if that ID does not exist of the same size as a regular message
func (m *GatewayImpl) GetMessage(userID *id.User,
	msgID string) (*pb.CmixMessage, bool) {
	jww.DEBUG.Printf("Getting message %q:%s from buffer...", *userID, msgID)
	return m.Buffer.GetMessage(userID, msgID)
}

// Return any MessageIDs in the globals for this User
func (m *GatewayImpl) CheckMessages(userID *id.User, messageID string) (
	[]string, bool) {
	jww.DEBUG.Printf("Getting message IDs for %q after %s from buffer...",
		userID, messageID)
	return m.Buffer.GetMessageIDs(userID, messageID)
}

// Receives batch from server and stores it in the local MessageBuffer
// FIXME: This bidirectionality is problematic. Might make more sense to poll
// instead OR have the receive be the return on SendBatch
func (m *GatewayImpl) ReceiveBatch(msg *pb.OutputMessages) {
	jww.DEBUG.Printf("Received batch of size %d from server", len(msg.Messages))
	msgs := msg.Messages
	h, _ := hash.NewCMixHash()

	for i := range msgs {
		associatedData := format.DeserializeAssociatedData(msgs[i].AssociatedData)
		userId := associatedData.GetRecipient()
		h.Write(msgs[i].MessagePayload)
		h.Write(msgs[i].AssociatedData)
		msgId := base64.StdEncoding.EncodeToString(h.Sum(nil))
		m.Buffer.AddMessage(userId, msgId, msgs[i])
		h.Reset()
	}
	go PrintProfilingStatistics()
}

// PutMessage adds a message to the outgoing queue and
// calls SendBatch when it's size is the batch size
func (m *GatewayImpl) PutMessage(msg *pb.CmixMessage) bool {
	jww.DEBUG.Printf("Putting message in outgoing queue...")
	m.Buffer.AddOutgoingMessage(msg)

	// If there are batchsize messages, send them now
	batch := m.Buffer.PopMessages(m.BatchSize, m.BatchSize)
	if batch != nil {
		jww.DEBUG.Printf("Sending batch to %s...", m.GatewayNode)
		err := gateway.SendBatch(m.GatewayNode, batch)
		if err != nil {
			// TODO: Handle failure sending batch
		}
		return true
	}
	return false
}

// Pass-through for Registration Nonce Communication
func (m *GatewayImpl) RequestNonce(message *pb.RequestNonceMessage) (
	*pb.NonceMessage, error) {
	return gateway.SendRequestNonceMessage(m.GatewayNode, message)
}

// Pass-through for Registration Nonce Confirmation
func (m *GatewayImpl) ConfirmNonce(message *pb.ConfirmNonceMessage) (*pb.
	RegistrationConfirmation, error) {
	return gateway.SendConfirmNonceMessage(m.GatewayNode, message)
}

// GenJunkMsg generates a junk message using the gateway's client key
func GenJunkMsg() *pb.CmixMessage {
	//TODO: Real junk message
	return &pb.CmixMessage{
		AssociatedData: []byte{0x01},
		MessagePayload: []byte{0x01},
	}
}

// SendBatchWhenReady polls for the servers RoundBufferInfo object, checks
// if there are at least minRoundCnt rounds ready, and sends whenever there
// are minMsgCnt messages available in the message queue
func SendBatchWhenReady(g *GatewayImpl, minMsgCnt uint64) {
	for {
		bufSize, err := gateway.GetRoundBufferInfo(g.GatewayNode)
		if err != nil {
			jww.INFO.Printf("GetRoundBufferInfo error returned: %v", err)
			continue
		}
		if bufSize == 0 {
			continue
		}

		batch := g.Buffer.PopMessages(minMsgCnt, g.BatchSize)

		// If the batch length is zero or greater than 1, then fill the batch.
		// If the length of batch is 1, then wait for another message.
		if batch == nil || len(batch) != -1 {
			// Fill the batch with junk
			for i := uint64(len(batch)); i < g.BatchSize; i++ {
				batch = append(batch, GenJunkMsg())
			}
		} else {
			jww.INFO.Printf("Server is ready, but only have %d messages to "+
				"send, need %d! Waiting 10 seconds!",
				g.Buffer.Len(), minMsgCnt)
			time.Sleep(10 * time.Second)
			continue
		}

		// Send the batch
		err = gateway.SendBatch(g.GatewayNode, batch)
		if err != nil {
			// TODO: handle failure sending batch
		}
	}
}

// StartGateway sets up the threads and network server to run the gateway
func StartGateway(batchSize uint64, cMixNodes []string, gatewayNode, address,
	certPath, keyPath string) {
	// minMsgCnt should be no less than 33% of the batchSize
	// Note: this is security sensitive.. be careful if you pull this out to a
	// config option.
	minMsgCnt := uint64(batchSize / 3)
	if minMsgCnt == 0 {
		minMsgCnt = 1
	}
	gatewayImpl := NewGatewayImpl(batchSize, cMixNodes, gatewayNode)
	gateway.StartGateway(address, gatewayImpl, certPath, keyPath)
	go SendBatchWhenReady(gatewayImpl, minMsgCnt)
	// Wait forever
	select {}
}
