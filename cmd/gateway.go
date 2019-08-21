////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"encoding/base64"
	"fmt"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/comms/gateway"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/utils"
	"gitlab.com/elixxir/crypto/cmix"
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/crypto/large"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/elixxir/primitives/format"
	"gitlab.com/elixxir/primitives/id"
	"io/ioutil"
	"time"
)

type connectionID string

var dummyUser *id.User = id.MakeDummyUserID()

func (c connectionID) String() string {
	return (string)(c)
}

type Instance struct {
	// Storage buffer for inbound/outbound messages
	Buffer storage.MessageBuffer

	// Contains all Gateway relevant fields
	Params Params

	// Gateway object created at start
	Comms *gateway.GatewayComms

	//Group that cmix operates within
	CmixGrp *cyclic.Group
}

type Params struct {
	BatchSize   uint64
	CMixNodes   []string
	GatewayNode connectionID
	Port        int
	CertPath    string
	KeyPath     string

	ServerCertPath string
	CmixGrp        map[string]string
}

// NewGatewayInstance initializes a gateway Handler interface
func NewGatewayInstance(params Params) *Instance {
	p := large.NewIntFromString(params.CmixGrp["prime"], 16)
	q := large.NewIntFromString(params.CmixGrp["smallprime"], 16)
	g := large.NewIntFromString(params.CmixGrp["generator"], 16)
	grp := cyclic.NewGroup(p, g, q)

	return &Instance{
		Buffer:  storage.NewMessageBuffer(),
		Params:  params,
		CmixGrp: grp,
	}
}

// InitNetwork initializes the network on this gateway instance
// After the network object is created, you need to use it to connect
// to the corresponding server in the network using ConnectToNode.
// Additionally, to clean up the network object (especially in tests), call
// Shutdown() on the network object.
func (gw *Instance) InitNetwork() {
	// Set up a comms server
	address := fmt.Sprintf("0.0.0.0:%d", gw.Params.Port)
	var err error
	var gwCert, gwKey, nodeCert []byte

	if !noTLS {
		gwCert, err = ioutil.ReadFile(utils.GetFullPath(gw.Params.CertPath))
		if err != nil {
			jww.ERROR.Printf("Failed to read certificate at %s: %+v", gw.Params.CertPath, err)
		}
		gwKey, err = ioutil.ReadFile(utils.GetFullPath(gw.Params.KeyPath))
		if err != nil {
			jww.ERROR.Printf("Failed to read gwKey at %s: %+v", gw.Params.KeyPath, err)
		}
		nodeCert, err = ioutil.ReadFile(utils.GetFullPath(gw.Params.ServerCertPath))
		if err != nil {
			jww.ERROR.Printf("Failed to read server gwCert at %s: %+v", gw.Params.ServerCertPath, err)
		}
	}
	gw.Comms = gateway.StartGateway(address, gw, gwCert, gwKey)

	// Connect to the associated Node

	err = gw.Comms.ConnectToRemote(connectionID(gw.Params.GatewayNode), string(gw.Params.GatewayNode), nodeCert)

	if !disablePermissioning {
		if noTLS {
			jww.ERROR.Panicf("Panic: cannot have permissinoning on and TLS disabled")
		}
		// Obtain signed certificates from the Node
		jww.INFO.Printf("Beginning polling for signed certs...")
		var signedCerts *pb.SignedCerts
		for signedCerts == nil {
			msg, err := gw.Comms.PollSignedCerts(gw.Params.GatewayNode, &pb.Ping{})
			if err != nil {
				jww.ERROR.Printf("Error obtaining signed certificates: %+v", err)
			}
			if msg.ServerCertPEM != "" && msg.GatewayCertPEM != "" {
				signedCerts = msg
				jww.INFO.Printf("Successfully obtained signed certs!")
			}
		}

		// Replace the comms server with the newly-signed certificate
		gw.Comms.Shutdown()
		gw.Comms = gateway.StartGateway(address, gw,
			[]byte(signedCerts.GatewayCertPEM), gwKey)

		// Use the signed Server certificate to open a new connection
		err = gw.Comms.ConnectToNode(connectionID(gw.Params.GatewayNode),
			string(gw.Params.GatewayNode), []byte(signedCerts.ServerCertPEM))
	}
}

// Returns message contents for MessageID, or a null/randomized message
// if that ID does not exist of the same size as a regular message
func (gw *Instance) GetMessage(userID *id.User, msgID string) (*pb.Slot, bool) {
	jww.DEBUG.Printf("Getting message %q:%s from buffer...", *userID, msgID)
	return gw.Buffer.GetMixedMessage(userID, msgID)
}

// Return any MessageIDs in the globals for this User
func (gw *Instance) CheckMessages(userID *id.User, msgID string) ([]string, bool) {
	jww.DEBUG.Printf("Getting message IDs for %q after %s from buffer...",
		userID, msgID)
	return gw.Buffer.GetMixedMessageIDs(userID, msgID)
}

// PutMessage adds a message to the outgoing queue and calls PostNewBatch when
// it's size is the batch size
func (gw *Instance) PutMessage(msg *pb.Slot) bool {
	jww.DEBUG.Printf("Putting message from user %v in outgoing queue...",
		msg.GetSenderID())
	gw.Buffer.AddUnmixedMessage(msg)
	return true
}

// Pass-through for Registration Nonce Communication
func (gw *Instance) RequestNonce(msg *pb.NonceRequest) (*pb.Nonce, error) {
	jww.INFO.Print("Passing on registration nonce request")
	return gw.Comms.SendRequestNonceMessage(gw.Params.GatewayNode, msg)
}

// Pass-through for Registration Nonce Confirmation
func (gw *Instance) ConfirmNonce(msg *pb.RequestRegistrationConfirmation) (
	*pb.RegistrationConfirmation, error) {
	jww.INFO.Print("Passing on registration nonce confirmation")
	return gw.Comms.SendConfirmNonceMessage(gw.Params.GatewayNode, msg)
}

// GenJunkMsg generates a junk message using the gateway's client key
func GenJunkMsg(grp *cyclic.Group, numnodes int) *pb.Slot {

	baseKey := grp.NewIntFromBytes((*dummyUser)[:])

	var baseKeys []*cyclic.Int

	for i := 0; i < numnodes; i++ {
		baseKeys = append(baseKeys, baseKey)
	}

	salt := make([]byte, 32)
	salt[0] = 0x01

	msg := format.NewMessage()
	payloadBytes := make([]byte, format.PayloadLen)
	payloadBytes[0] = 0x01
	msg.SetPayloadA(payloadBytes)
	msg.SetPayloadB(payloadBytes)
	msg.SetRecipient(dummyUser)

	ecrMsg := cmix.ClientEncrypt(grp, msg, salt, baseKeys)

	h, err := hash.NewCMixHash()
	if err != nil {
		jww.FATAL.Printf("Could not get hash: %+v", err)
	}

	KMACs := cmix.GenerateKMACs(salt, baseKeys, h)

	return &pb.Slot{
		PayloadB: ecrMsg.GetPayloadB(),
		PayloadA: ecrMsg.GetPayloadA(),
		Salt:     salt,
		SenderID: (*dummyUser)[:],
		KMACs:    KMACs,
	}
}

// SendBatchWhenReady polls for the servers RoundBufferInfo object, checks
// if there are at least minRoundCnt rounds ready, and sends whenever there
// are minMsgCnt messages available in the message queue
func (gw *Instance) SendBatchWhenReady(minMsgCnt uint64, junkMsg *pb.Slot) {

	bufSize, err := gw.Comms.GetRoundBufferInfo(gw.Params.GatewayNode)
	if err != nil {
		jww.INFO.Printf("GetRoundBufferInfo error returned: %v", err)
		return
	}
	if bufSize == 0 {
		return
	}

	batch := gw.Buffer.PopUnmixedMessages(0, gw.Params.BatchSize)
	if batch == nil {
		jww.INFO.Printf("Server is ready, but only have %d messages to send, "+
			"need %d! Waiting 10 seconds!", gw.Buffer.LenUnmixed(), minMsgCnt)
		time.Sleep(10 * time.Second)
		return
	}

	// Now fill with junk and send
	for i := uint64(len(batch.Slots)); i < gw.Params.BatchSize; i++ {
		newJunkMsg := &pb.Slot{
			PayloadB: junkMsg.PayloadB,
			PayloadA: junkMsg.PayloadA,
			Salt:     junkMsg.Salt,
			SenderID: junkMsg.SenderID,
		}

		batch.Slots = append(batch.Slots, newJunkMsg)
	}
	err = gw.Comms.PostNewBatch(gw.Params.GatewayNode, batch)
	if err != nil {
		// TODO: handle failure sending batch
	}

}

func (gw *Instance) PollForBatch() {
	batch, err := gw.Comms.GetCompletedBatch(gw.Params.GatewayNode)
	if err != nil {
		// Would a timeout count as an error?
		// No, because the server could just as easily return a batch
		// with no slots/an empty slice of slots
		jww.ERROR.Printf("Received error from GetCompletedBatch"+
			" call: %+v", errors.New(err.Error()))
		return
	}
	if len(batch.Slots) == 0 {
		return
	}

	numReal := 0

	// At this point, the returned batch and its fields should be non-nil
	msgs := batch.Slots
	h, _ := hash.NewCMixHash()
	for _, msg := range msgs {
		serialmsg := format.NewMessage()
		serialmsg.SetPayloadB(msg.PayloadB)
		userId := serialmsg.GetRecipient()

		if !userId.Cmp(dummyUser) {
			numReal++
			h.Write(msg.PayloadA)
			h.Write(msg.PayloadB)
			msgId := base64.StdEncoding.EncodeToString(h.Sum(nil))
			gw.Buffer.AddMixedMessage(userId, msgId, msg)
		}

		h.Reset()
	}
	jww.INFO.Printf("Round %v recieved, %v real messages "+
		"processed, %v dummies ignored", batch.Round.ID, numReal,
		int(batch.Round.ID)-numReal)

	go PrintProfilingStatistics()
}

// StartGateway sets up the threads and network server to run the gateway
func (gw *Instance) Start() {

	//Begin the thread which polls the node for a request to send a batch
	go func() {
		// minMsgCnt should be no less than 33% of the BatchSize
		// Note: this is security sensitive.. be careful if you pull this out to a
		// config option.
		minMsgCnt := uint64(gw.Params.BatchSize / 3)
		if minMsgCnt == 0 {
			minMsgCnt = 1
		}
		junkMsg := GenJunkMsg(gw.CmixGrp, len(gw.Params.CMixNodes))
		for true {
			gw.SendBatchWhenReady(minMsgCnt, junkMsg)
		}
	}()

	//Begin the thread which polls the node for a completed batch
	go func() {
		for true {
			gw.PollForBatch()
		}
	}()
}
