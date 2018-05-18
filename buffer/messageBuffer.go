////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package buffer

import (
	"gitlab.com/privategrity/comms/gateway"
	pb "gitlab.com/privategrity/comms/mixmessages"
	"sync"
	"gitlab.com/privategrity/crypto/hash"
	"encoding/base64"
)

// Interface for interacting with the MessageBuffer
// TODO: Move this into it's own interface.go file
// TODO: Separate into storage/interface.go and storage/ram.go
// (in prep for a db impl)
type MessageBuffer interface {
	GetMessage(userId uint64, msgId string) (*pb.CmixMessage, bool)
	CheckMessages(userId uint64) ([]string, bool)
	PutMessage(*pb.CmixMessage) bool
	ReceiveBatch(messages *pb.OutputMessages)
}

// MessageBuffer struct with map backend
// TODO: Maybe UserMessages struct and an Outgoing messages struct?
// Left it as is for simplicity.
// TODO: Call this GatewayImpl or similar, instead, since it's implementing
// the comms gateway handler functionality?
type MapBuffer struct {
	messageCollection map[uint64]map[string]*pb.CmixMessage
	outgoingMessages  []*pb.CmixMessage
	mux               sync.Mutex
}

// Initialize a MessageBuffer interface
func newMessageBuffer() MessageBuffer {
	return MessageBuffer(&MapBuffer{
		messageCollection: make(map[uint64]map[string]*pb.CmixMessage),
		outgoingMessages:  make([]*pb.CmixMessage, 0),
	})
}

// Adds a message to the MessageBuffer
func (m *MapBuffer) AddMessage(userId uint64, msgId string,
	msg *pb.CmixMessage) {
	m.mux.Lock()
	if len(m.messageCollection[userId]) == 0 {
		// If the User->Message map hasn't been initialized, initialize it
		m.messageCollection[userId] = make(map[string]*pb.CmixMessage)
	}
	m.messageCollection[userId][msgId] = msg
	m.mux.Unlock()
}

// Returns message contents for MessageID, or a null/randomized message
// if that ID does not exist of the same size as a regular message
func (m *MapBuffer) GetMessage(userId uint64, msgId string) (*pb.CmixMessage,
	bool) {
	m.mux.Lock()
	msg, ok := m.messageCollection[userId][msgId]
	m.mux.Unlock()
	return msg, ok
}

// Return any MessageIDs in the buffer for this UserID
func (m *MapBuffer) CheckMessages(userId uint64) ([]string, bool) {
	m.mux.Lock()
	userMap, ok := m.messageCollection[userId]
	msgIds := make([]string, 0, len(userMap))
	for msgId := range userMap {
		msgIds = append(msgIds, msgId)
	}
	m.mux.Unlock()
	return msgIds, ok
}

// PutMessage adds a message to the outgoing queue and
// calls SendBatch when it's size is the batch size
func (m *MapBuffer) PutMessage(msg *pb.CmixMessage) bool {
	m.mux.Lock()
	m.outgoingMessages = append(m.outgoingMessages, msg)
	if uint64(len(m.outgoingMessages)) == BATCH_SIZE {
		gateway.SendBatch(GATEWAY_NODE, m.outgoingMessages)
		m.outgoingMessages = make([]*pb.CmixMessage, 0)
	}
	m.mux.Unlock()
	return true

}

// Deletes a given message from the MessageBuffer
func (m *MapBuffer) DeleteMessage(userId uint64, msgId string) {
	m.mux.Lock()
	delete(m.messageCollection[userId], msgId)
	m.mux.Unlock()
}

// ReceiveBatch adds a message to the outgoing queue and
// calls SendBatch when it's size is the batch size
func (m *MapBuffer) ReceiveBatch(msg *pb.OutputMessages) {
	msgs := msg.Messages
	h, _ := hash.NewCMixHash()

	for i :=range  msgs {
		userId := msgs[i].SenderID
		h.Write(msgs[i].MessagePayload)
		msgId := base64.StdEncoding.EncodeToString(h.Sum(nil))
		m.AddMessage(userId, msgId, msgs[i])
		h.Reset()
	}
}