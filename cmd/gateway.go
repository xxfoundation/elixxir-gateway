///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"
	"gitlab.com/elixxir/comms/gateway"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	ds "gitlab.com/elixxir/comms/network/dataStructures"
	"gitlab.com/elixxir/crypto/cmix"
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/gateway/notifications"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/elixxir/primitives/format"
	"gitlab.com/elixxir/primitives/knownRounds"
	"gitlab.com/elixxir/primitives/rateLimiting"
	"gitlab.com/elixxir/primitives/utils"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/ndf"
	"strings"
	"time"
)

var dummyUser = id.DummyUser

// TODO: remove this. It is currently only here to make tests work
var disablePermissioning = false

var rateLimitErr = errors.New("Client has exceeded communications rate limit")

// Tokens required by clients for different messages
const TokensPutMessage = uint(250)  // Sends a message, the networks does n * 5 exponentiations, n = 5, 25
const TokensRequestNonce = uint(30) // Requests a nonce from the node to verify the user, 3 exponentiations
const TokensConfirmNonce = uint(10) // Requests a nonce from the node to verify the user, 1 exponentiation

// Errors to suppress
const (
	ErrInvalidHost = "Invalid host ID:"
	ErrAuth        = "Failed to authenticate id:"
)

type Instance struct {
	// Storage buffer for messages after being processed by the network
	MixedBuffer storage.MixedMessageBuffer
	// Storage buffer for messages to be submitted to the network
	UnmixedBuffer storage.UnmixedMessageBuffer

	// Contains all Gateway relevant fields
	Params Params

	// Contains Server Host Information
	ServerHost *connect.Host

	// Gateway object created at start
	Comms *gateway.Comms

	// Gateway gossip manager
	gossiper *gossip.Manager

	// Gateway's rate limiter. Manages and
	rateLimiter   *rateLimiting.BucketMap
	rateLimitQuit chan struct{}

	// struct for tracking notifications
	un notifications.UserNotifications

	// Tracker of the gateway's known rounds
	knownRound *knownRounds.KnownRounds

	database storage.Storage
	// TODO: Integrate and remove duplication with the stuff above.
	// NetInf is the network interface for working with the NDF poll
	// functionality in comms.
	NetInf *network.Instance
}

func (gw *Instance) RequestHistoricalRounds(msg *pb.HistoricalRounds) (*pb.HistoricalRoundsResponse, error) {
	panic("implement me")
}

func (gw *Instance) RequestMessages(msg *pb.GetMessages) (*pb.GetMessagesResponse, error) {
	panic("implement me")
}

func (gw *Instance) RequestBloom(msg *pb.GetBloom) (*pb.GetBloomResponse, error) {
	panic("implement me")
}

func (gw *Instance) Poll(*pb.GatewayPoll) (*pb.GatewayPollResponse, error) {
	jww.FATAL.Panicf("Unimplemented!")
	return nil, nil
}

type Params struct {
	NodeAddress string
	Port        int
	Address     string
	CertPath    string
	KeyPath     string

	ServerCertPath        string
	IDFPath               string
	PermissioningCertPath string

	MessageTimeout time.Duration

	// Gossip protocol flags
	gossiperFlags gossip.ManagerFlags

	// Rate limiting parameters
	rateLimiterParams *rateLimiting.MapParams

	knownRounds int
}

// NewGatewayInstance initializes a gateway Handler interface
func NewGatewayInstance(params Params) *Instance {
	newDatabase, _, err := storage.NewDatabase(viper.GetString("dbUsername"),
		viper.GetString("dbPassword"),
		viper.GetString("dbName"),
		viper.GetString("dbAddress"),
		viper.GetString("dbPort"),
	)
	if err != nil {
		jww.FATAL.Panicf("Unable to initialize storage: %+v", err)
	}

	rateLimitKill := make(chan struct{}, 1)

	// Todo: populate with newDatabase once conforms to interface
	gwBucketMap := rateLimiting.CreateBucketMapFromParams(params.rateLimiterParams, nil, rateLimitKill)

	i := &Instance{
		MixedBuffer:   storage.NewMixedMessageBuffer(params.MessageTimeout),
		UnmixedBuffer: storage.NewUnmixedMessageBuffer(),
		Params:        params,
		database:      newDatabase,
		rateLimiter:   gwBucketMap,
		rateLimitQuit: rateLimitKill,
		knownRound:    knownRounds.NewKnownRound(params.knownRounds),
	}

	return i
}

func NewImplementation(instance *Instance) *gateway.Implementation {
	impl := gateway.NewImplementation()
	impl.Functions.CheckMessages = func(userID *id.ID, messageID, ipaddress string) (i []string, b error) {
		return instance.CheckMessages(userID, messageID, ipaddress)
	}
	impl.Functions.ConfirmNonce = func(message *pb.RequestRegistrationConfirmation, ipaddress string) (confirmation *pb.RegistrationConfirmation, e error) {
		return instance.ConfirmNonce(message, ipaddress)
	}
	impl.Functions.GetMessage = func(userID *id.ID, msgID, ipaddress string) (slot *pb.Slot, b error) {
		return instance.GetMessage(userID, msgID, ipaddress)
	}
	impl.Functions.PutMessage = func(message *pb.GatewaySlot, ipaddress string) (*pb.GatewaySlotResponse, error) {
		return instance.PutMessage(message, ipaddress)
	}
	impl.Functions.RequestNonce = func(message *pb.NonceRequest, ipaddress string) (nonce *pb.Nonce, e error) {
		return instance.RequestNonce(message, ipaddress)
	}
	impl.Functions.PollForNotifications = func(auth *connect.Auth) (i []*id.ID, e error) {
		return instance.PollForNotifications(auth)
	}
	return impl
}

// PollServer sends a poll message to the server and returns a response.
func PollServer(conn *gateway.Comms, pollee *connect.Host, ndf,
	partialNdf *network.SecuredNdf, lastUpdate uint64) (
	*pb.ServerPollResponse, error) {

	var ndfHash, partialNdfHash *pb.NDFHash
	ndfHash = &pb.NDFHash{
		Hash: make([]byte, 0),
	}

	partialNdfHash = &pb.NDFHash{
		Hash: make([]byte, 0),
	}

	if ndf != nil {
		ndfHash = &pb.NDFHash{Hash: ndf.GetHash()}
	}
	if partialNdf != nil {
		partialNdfHash = &pb.NDFHash{Hash: partialNdf.GetHash()}
	}

	pollMsg := &pb.ServerPoll{
		Full:           ndfHash,
		Partial:        partialNdfHash,
		LastUpdate:     lastUpdate,
		Error:          "",
		GatewayPort:    uint32(gwPort),
		GatewayVersion: currentVersion,
	}

	resp, err := conn.SendPoll(pollee, pollMsg)
	return resp, err
}

// CreateNetworkInstance will generate a new network instance object given
// properly formed ndf, partialNdf, and connection object
func CreateNetworkInstance(conn *gateway.Comms, ndf, partialNdf *pb.NDF) (
	*network.Instance, error) {
	newNdf := &ds.Ndf{}
	newPartialNdf := &ds.Ndf{}
	err := newNdf.Update(ndf)
	if err != nil {
		return nil, err
	}
	err = newPartialNdf.Update(partialNdf)
	if err != nil {
		return nil, err
	}
	pc := conn.ProtoComms
	var ers ds.ExternalRoundStorage = nil
	if storage.GatewayDB != nil {
		ers = &storage.ERS{}
	}
	return network.NewInstance(pc, newNdf.Get(), newPartialNdf.Get(), ers)
}

// UpdateInstance reads a ServerPollResponse object and updates the instance
// state accordingly.
func (gw *Instance) UpdateInstance(newInfo *pb.ServerPollResponse) error {
	// Update the NDFs, and update the round info, which is currently
	// recorded but not used for anything. (maybe we should print state
	// of each round?)
	if newInfo.FullNDF != nil {
		err := gw.NetInf.UpdateFullNdf(newInfo.FullNDF)
		if err != nil {
			return err
		}
	}
	if newInfo.PartialNDF != nil {
		err := gw.NetInf.UpdatePartialNdf(newInfo.PartialNDF)
		if err != nil {
			return err
		}
	}
	if newInfo.Updates != nil {
		for _, update := range newInfo.Updates {
			// Update the network instance
			err := gw.NetInf.RoundUpdate(update)
			if err != nil {
				return err
			}
		}
	}

	// Send a new batch to the server when it asks for one
	if newInfo.BatchRequest != nil {
		gw.SendBatchWhenReady(newInfo.BatchRequest)
	}
	// Process a batch that has been completed by this server
	if newInfo.Slots != nil {
		gw.ProcessCompletedBatch(newInfo.Slots)
	}
	return nil
}

// InitNetwork initializes the network on this gateway instance
// After the network object is created, you need to use it to connect
// to the corresponding server in the network using ConnectToNode.
// Additionally, to clean up the network object (especially in tests), call
// Shutdown() on the network object.
func (gw *Instance) InitNetwork() error {
	address := fmt.Sprintf("%s:%d", gw.Params.Address, gw.Params.Port)
	var err error
	var gwCert, gwKey, nodeCert, permissioningCert []byte

	// Read our cert from file
	gwCert, err = utils.ReadFile(gw.Params.CertPath)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to read certificate at %s: %+v",
			gw.Params.CertPath, err))
	}

	// Read our private key from file
	gwKey, err = utils.ReadFile(gw.Params.KeyPath)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to read gwKey at %s: %+v",
			gw.Params.KeyPath, err))
	}

	// Read our node's cert from file
	nodeCert, err = utils.ReadFile(gw.Params.ServerCertPath)
	if err != nil {
		return errors.New(fmt.Sprintf(
			"Failed to read server gwCert at %s: %+v", gw.Params.ServerCertPath, err))
	}

	// Read the permissioning server's cert from
	if !disablePermissioning {
		permissioningCert, err = utils.ReadFile(gw.Params.PermissioningCertPath)
		if err != nil {
			return errors.WithMessagef(err,
				"Failed to read permissioning cert at %v",
				gw.Params.PermissioningCertPath)
		}
	}

	// Set up temporary gateway listener
	gatewayHandler := NewImplementation(gw)
	gw.Comms = gateway.StartGateway(&id.TempGateway, address, gatewayHandler, gwCert, gwKey)

	// Set up temporary server host
	// (id, address string, cert []byte, disableTimeout, enableAuth bool)
	dummyServerID := id.DummyUser.DeepCopy()
	dummyServerID.SetType(id.Node)
	gw.ServerHost, err = connect.NewHost(dummyServerID, gw.Params.NodeAddress,
		nodeCert, true, true)
	if err != nil {
		return errors.Errorf("Unable to create tmp server host: %+v",
			err)
	}

	// Permissioning-enabled pathway
	if !disablePermissioning {

		// Begin polling server for NDF
		jww.INFO.Printf("Beginning polling NDF...")
		var nodeId []byte
		var serverResponse *pb.ServerPollResponse

		// fixme: determine if this a proper conditional for when server is not ready
		for serverResponse == nil {
			// TODO: Probably not great to always sleep immediately
			time.Sleep(3 * time.Second)

			// Poll Server for the NDFs, then use it to create the
			// network instance and begin polling for server updates
			serverResponse, err = PollServer(gw.Comms, gw.ServerHost, nil, nil, 0)
			if err != nil {
				eMsg := err.Error()
				// Catch recoverable error
				if strings.Contains(eMsg, ErrInvalidHost) {
					jww.WARN.Printf("Node not ready...: %s",
						eMsg)
					continue
					// NO_NDF will be returned if the node
					// has not retrieved an NDF from
					// permissioning yet
				} else if strings.Contains(eMsg, ndf.NO_NDF) {
					continue
				} else if strings.Contains(eMsg, ErrAuth) {
					jww.WARN.Printf(eMsg)
					continue
				} else {
					return errors.Errorf(
						"Error polling NDF: %+v", err)
				}
			}

			jww.DEBUG.Printf("Creating instance!")
			gw.NetInf, err = CreateNetworkInstance(gw.Comms,
				serverResponse.FullNDF,
				serverResponse.PartialNDF)
			if err != nil {
				jww.ERROR.Printf("Unable to create network"+
					" instance: %v", err)
				continue
			}

			// Add permissioning as a host
			_, err = gw.Comms.AddHost(&id.Permissioning, "", permissioningCert, true, true)
			if err != nil {
				jww.ERROR.Printf("Couldn't add permissioning host to comms: %v", err)
				continue
			}

			// Update the network instance
			jww.DEBUG.Printf("Updating instance")
			err = gw.UpdateInstance(serverResponse)
			if err != nil {
				jww.ERROR.Printf("Update instance error: %v", err)
				continue
			}

			// Install the NDF once we get it
			if serverResponse.FullNDF != nil && serverResponse.Id != nil {
				err = gw.setupIDF(serverResponse.Id)
				nodeId = serverResponse.Id
				if err != nil {
					jww.WARN.Printf("failed to update node information: %+v", err)
					return err
				}
			}
		}

		jww.INFO.Printf("Successfully obtained NDF!")

		// Replace the comms server with the newly-signed certificate
		// fixme: determine if we need to restart gw for restart with new id
		gw.Comms.Shutdown()

		serverID, err2 := id.Unmarshal(nodeId)
		if err2 != nil {
			jww.ERROR.Printf("Unmarshalling serverID failed during network "+
				"init: %+v", err2)
		}
		gw.ServerHost.Disconnect()

		// Update the host information with the new server ID
		gw.ServerHost, err = connect.NewHost(serverID, gw.Params.NodeAddress, nodeCert,
			true, true)
		if err != nil {
			return errors.Errorf(
				"Unable to create updated server host: %+v", err)
		}

		gatewayId := serverID
		gatewayId.SetType(id.Gateway)
		gw.Comms = gateway.StartGateway(gatewayId, address, gatewayHandler, gwCert, gwKey)

		// Initialize hosts for reverse-authentication
		// This may be necessary to verify the NDF if it gets updated while
		// the network is up
		_, err = gw.Comms.AddHost(&id.Permissioning, "", permissioningCert, true, true)
		if err != nil {
			return errors.Errorf("Couldn't add permissioning host: %v", err)
		}

		// newNdf := gw.NetInf.GetPartialNdf().Get()

		// Add notification bot as a host
		// _, err = gw.Comms.AddHost(&id.NotificationBot, newNdf.Notification.Address,
		// 	[]byte(newNdf.Notification.TlsCertificate), false, true)
		// if err != nil {
		// 	return errors.Errorf("Unable to add notifications host: %+v", err)
		// }
	}

	gw.setupGossiper()

	return nil
}

// Helper function which initializes the gossip manager
func (gw *Instance) setupGossiper() {
	gw.gossiper = gossip.NewManager(gw.Comms.ProtoComms, gw.Params.gossiperFlags)
	receiver := func(msg *gossip.GossipMsg) error {
		// fixme: what to do with receiver?
		return nil
	}

	// Build the signature verify function
	sigVerify := func(msg *gossip.GossipMsg, receivedSignature []byte) error {
		if msg == nil {
			return errors.New("Nil message sent")
		}

		senderId, err := id.Unmarshal(msg.Origin)
		if err != nil {
			return errors.Errorf("Failed to parse sender's id: %v", err)
		}

		sender, ok := gw.Comms.GetHost(senderId)
		if !ok {
			return errors.Errorf("Failed to parse sender's id: %v", err)

		}

		options := rsa.NewDefaultOptions()
		h := options.Hash.New()
		h.Reset()
		h.Write(msg.Payload)

		return rsa.Verify(sender.GetPubKey(), options.Hash, h.Sum(nil), msg.Signature, nil)
	}

	peers := []*id.ID{gw.ServerHost.GetId()}

	// todo: consider hard-coding tags globally or in a library?
	gw.gossiper.NewGossip("batch", gossip.DefaultProtocolFlags(), receiver, sigVerify, peers)
}

// Helper that updates parses the NDF in order to create our IDF
func (gw *Instance) setupIDF(nodeId []byte) (err error) {

	// Get the ndf from our network instance
	ourNdf := gw.NetInf.GetPartialNdf().Get()

	// Determine the index of this gateway
	for i, node := range ourNdf.Nodes {
		// Find our node in the ndf
		if bytes.Compare(node.ID, nodeId) == 0 {

			// Save the IDF to the idfPath
			err := writeIDF(ourNdf, i, idfPath)
			if err != nil {
				jww.WARN.Printf("Could not write ID File: %s",
					idfPath)
			}

			return nil
		}
	}

	return errors.Errorf("Unable to locate ID %v in NDF!", nodeId)
}

// Returns message contents for MessageID, or a null/randomized message
// if that ID does not exist of the same size as a regular message
func (gw *Instance) GetMessage(userID *id.ID, msgID string, ipAddress string) (*pb.Slot, error) {
	// Check if sender has exceeded the rate limit
	senderBucket := gw.rateLimiter.LookupBucket(ipAddress)
	// fixme: Hardcoded, or base it on something like the length of the message?
	success := senderBucket.Add(1)
	if !success {
		return &pb.Slot{}, errors.New("Receiving messages at a high rate. Please " +
			"wait before sending more messages")
	}
	jww.DEBUG.Printf("Getting message %q:%s from buffer...", *userID, msgID)
	return gw.MixedBuffer.GetMixedMessage(userID, msgID)
}

// Return any MessageIDs in the globals for this User
func (gw *Instance) CheckMessages(userID *id.ID, msgID string, ipAddress string) ([]string, error) {
	// Check if sender has exceeded the rate limit
	senderBucket := gw.rateLimiter.LookupBucket(ipAddress)
	// fixme: Hardcoded, or base it on something like the length of the message?
	success := senderBucket.Add(1)
	if !success {
		return []string{}, errors.New("Receiving messages at a high rate. Please " +
			"wait before sending more messages")
	}

	jww.DEBUG.Printf("Getting message IDs for %q after %s from buffer...",
		userID, msgID)
	return gw.MixedBuffer.GetMixedMessageIDs(userID, msgID)
}

// PutMessage adds a message to the outgoing queue and calls PostNewBatch when
// it's size is the batch size
func (gw *Instance) PutMessage(msg *pb.GatewaySlot, ipAddress string) (*pb.GatewaySlotResponse, error) {
	// Fixme: work needs to be done to populate database with precanned values
	//  so that precanned users aren't rejected when sending messages
	// Construct Client ID for database lookup
	//clientID, err := id.Unmarshal(msg.Message.SenderID)
	//if err != nil {
	//	return &pb.GatewaySlotResponse{
	//		Accepted: false,
	//	}, errors.Errorf("Could not parse message: Unrecognized ID")
	//}

	// Retrieve the client from the database
	//cl, err := gw.database.GetClient(clientID)
	//if err != nil {
	//	return &pb.GatewaySlotResponse{
	//		Accepted: false,
	//	}, errors.New("Did not recognize ID. Have you registered successfully?")
	//}

	// Generate the MAC and check against the message's MAC
	//clientMac := generateClientMac(cl, msg)
	//if !bytes.Equal(clientMac, msg.MAC) {
	//	return &pb.GatewaySlotResponse{
	//		Accepted: false,
	//	}, errors.New("Could not authenticate client. Please try again later")
	//}

	// Check if sender has exceeded the rate limit
	senderBucket := gw.rateLimiter.LookupBucket(ipAddress)
	// fixme: Hardcoded, or base it on something like the length of the message?
	success := senderBucket.Add(1)
	if !success {
		return &pb.GatewaySlotResponse{}, errors.New("Receiving messages at a high rate. Please " +
			"wait before sending more messages")
	}

	jww.DEBUG.Printf("Putting message from user %v in outgoing queue...",
		msg.Message.GetSenderID())
	gw.UnmixedBuffer.AddUnmixedMessage(msg.Message)

	return &pb.GatewaySlotResponse{
		Accepted: true,
	}, nil
}

// Helper function which generates the client MAC for checking the clients
// authenticity
func generateClientMac(cl *storage.Client, msg *pb.GatewaySlot) []byte {
	// Digest the message for the MAC generation
	gatewaySlotDigest := network.GenerateSlotDigest(msg)

	// Hash the clientGatewayKey and the the slot's salt
	h, _ := hash.NewCMixHash()
	h.Write(cl.Key)
	h.Write(msg.Message.Salt)
	hashed := h.Sum(nil)

	h.Reset()

	// Hash the gatewaySlotDigest and the above hashed data
	h.Write(hashed)
	h.Write(gatewaySlotDigest)

	return h.Sum(nil)
}

// Pass-through for Registration Nonce Communication
func (gw *Instance) RequestNonce(msg *pb.NonceRequest, ipAddress string) (*pb.Nonce, error) {
	jww.INFO.Print("Checking rate limiting check on Nonce Request")

	// Check if sender has exceeded the rate limit
	senderBucket := gw.rateLimiter.LookupBucket(ipAddress)
	// fixme: Hardcoded, or base it on something like the length of the message
	success := senderBucket.Add(1)
	if !success {
		return &pb.Nonce{}, errors.New("Receiving messages at a high rate. Please " +
			"wait before sending more messages")
	}
	jww.INFO.Print("Passing on registration nonce request")
	return gw.Comms.SendRequestNonceMessage(gw.ServerHost, msg)

}

// Pass-through for Registration Nonce Confirmation
func (gw *Instance) ConfirmNonce(msg *pb.RequestRegistrationConfirmation,
	ipAddress string) (*pb.RegistrationConfirmation, error) {

	// Check if sender has exceeded the rate limit
	senderBucket := gw.rateLimiter.LookupBucket(ipAddress)
	// fixme: Hardcoded, or base it on something like the length of the message
	success := senderBucket.Add(1)
	if !success {
		return &pb.RegistrationConfirmation{}, errors.New("Receiving messages at a high rate. Please " +
			"wait before sending more messages")
	}

	jww.INFO.Print("Passing on registration nonce confirmation")

	resp, err := gw.Comms.SendConfirmNonceMessage(gw.ServerHost, msg)

	if err != nil {
		return resp, err
	}

	// Insert client information to database
	newClient := &storage.Client{
		Id:  msg.UserID,
		Key: resp.ClientGatewayKey,
	}

	err = gw.database.InsertClient(newClient)
	if err != nil {
		return resp, nil
	}

	return resp, nil
}

// GenJunkMsg generates a junk message using the gateway's client key
func GenJunkMsg(grp *cyclic.Group, numNodes int, msgNum uint32) *pb.Slot {

	baseKey := grp.NewIntFromBytes((dummyUser)[:])

	var baseKeys []*cyclic.Int

	for i := 0; i < numNodes; i++ {
		baseKeys = append(baseKeys, baseKey)
	}

	salt := make([]byte, 32)
	salt[0] = 0x01

	msg := format.NewMessage()
	payloadBytes := make([]byte, format.PayloadLen)
	bs := make([]byte, 4)
	// Note: Cannot be 0, must be inside group
	// So we add 1, and start at offset in payload
	// to avoid both conditions
	binary.LittleEndian.PutUint32(bs, msgNum+1)
	for i := 0; i < len(bs); i++ {
		payloadBytes[i+1] = bs[i]
	}
	msg.SetPayloadA(payloadBytes)
	msg.SetPayloadB(payloadBytes)
	msg.SetRecipient(&dummyUser)

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
		SenderID: (dummyUser)[:],
		KMACs:    KMACs,
	}
}

// SendBatchWhenReady polls for the servers RoundBufferInfo object, checks
// if there are at least minRoundCnt rounds ready, and sends whenever there
// are minMsgCnt messages available in the message queue
func (gw *Instance) SendBatchWhenReady(roundInfo *pb.RoundInfo) {

	batchSize := uint64(roundInfo.BatchSize)
	if batchSize == 0 {
		jww.WARN.Printf("Server sent empty roundBufferSize!")
		return
	}

	// minMsgCnt should be no less than 33% of the BatchSize
	// Note: this is security sensitive.. be careful modifying it
	minMsgCnt := uint64(roundInfo.BatchSize / 3)
	if minMsgCnt == 0 {
		minMsgCnt = 1
	}

	// FIXME: HACK HACK HACK -- disable the minMsgCnt
	// We're unable to use this right now because we are moving to
	// multi-team setups and adding a time out to rounds. So it is
	// likely we will loop forever and/or drop a lot of rounds due to
	// not receiving messages quickly enough for a certain round.
	// Will revisit if we add the ability to re-encrypt messages for
	// a different round or another mechanism becomes available.
	minMsgCnt = 0

	batch := gw.UnmixedBuffer.PopUnmixedMessages(minMsgCnt, batchSize)
	for batch == nil {
		jww.INFO.Printf(
			"Server is ready, but only have %d messages to send, "+
				"need %d! Waiting 1 seconds!",
			gw.UnmixedBuffer.LenUnmixed(),
			minMsgCnt)
		time.Sleep(1 * time.Second)
		batch = gw.UnmixedBuffer.PopUnmixedMessages(minMsgCnt,
			batchSize)
	}

	batch.Round = roundInfo

	jww.INFO.Printf("Sending batch with %d messages...", len(batch.Slots))

	numNodes := len(roundInfo.GetTopology())

	if numNodes == 0 {
		jww.ERROR.Println("Round topology empty, sending bad messages!")
	}

	// fixme: put in a go func?
	// Gossip the sender IDs in the batch to our peers
	_, errs := gw.gossipBatch(batch)
	if len(errs) != 0 {
		jww.WARN.Printf("bad error: %v", errs)
		jww.WARN.Printf("Could not gossip batch to peers")
	}
	// Now fill with junk and send
	for i := uint64(len(batch.Slots)); i < batchSize; i++ {
		junkMsg := GenJunkMsg(gw.NetInf.GetCmixGroup(), numNodes,
			uint32(i))
		batch.Slots = append(batch.Slots, junkMsg)
	}

	err := gw.Comms.PostNewBatch(gw.ServerHost, batch)
	if err != nil {
		// TODO: handle failure sending batch
		jww.WARN.Printf("Error while sending batch %v", err)

	}

	// Update the known round buffer
	gw.knownRound.Check(id.Round(roundInfo.ID))

}

// gossipBatch builds a gossip message containing all of the sender ID's
// within the batch and gossips it to all peers
func (gw *Instance) gossipBatch(batch *pb.Batch) (int, []error) {
	gossipProtocol, ok := gw.gossiper.Get("batch")
	if !ok {
		jww.WARN.Printf("Unable to get gossip protocol. Sending batch without gossiping...")
	}

	// Collect all of the sender IDs in the batch
	var senderIds []*id.ID
	for i, slot := range batch.Slots {
		sender, err := id.Unmarshal(slot.SenderID)
		if err != nil {
			jww.WARN.Printf("Could not gossip for slot %d in round %d: Unreadable sender ID: %v",
				i, batch.Round.ID, slot.SenderID)
		}

		senderIds = append(senderIds, sender)
	}

	// Marshal the list of senders into a json
	payloadData, err := json.Marshal(senderIds)
	if err != nil {
		jww.WARN.Printf("Could not form gossip payload!")
	}

	// Hash the payload data for signing
	privKey := gw.Comms.ProtoComms.GetPrivateKey()
	options := rsa.NewDefaultOptions()
	h := options.Hash.New()
	h.Reset()
	h.Write(payloadData)

	// Construct the message's signature
	sig, err := rsa.Sign(rand.Reader, privKey, options.Hash, h.Sum(nil), nil)
	if err != nil {
		jww.WARN.Printf("Failed to sign gossip message: %v", err)
	}

	// Build the message
	gossipMsg := &gossip.GossipMsg{
		Tag:       "batch",
		Origin:    gw.Comms.Id.Bytes(),
		Payload:   payloadData,
		Signature: sig,
	}

	// Gossip the message
	return gossipProtocol.Gossip(gossipMsg)

}

// ProcessCompletedBatch handles messages coming out of the mixnet
func (gw *Instance) ProcessCompletedBatch(msgs []*pb.Slot) {
	if len(msgs) == 0 {
		return
	}

	numReal := 0

	// At this point, the returned batch and its fields should be non-nil
	h, _ := hash.NewCMixHash()
	for _, msg := range msgs {
		serialmsg := format.NewMessage()
		serialmsg.SetPayloadB(msg.PayloadB)
		userId, err := serialmsg.GetRecipient()

		if err != nil {
			jww.ERROR.Printf("Creating userId from serialmsg failed in "+
				"ProcessCompletedBatch: %+v", err)
		}

		if !userId.Cmp(&dummyUser) {
			jww.DEBUG.Printf("Message Received for: %v",
				userId.Bytes())
			gw.un.Notify(userId)
			numReal++
			h.Write(msg.PayloadA)
			h.Write(msg.PayloadB)
			msgId := base64.StdEncoding.EncodeToString(h.Sum(nil))
			gw.MixedBuffer.AddMixedMessage(userId, msgId, msg)
		}

		h.Reset()
	}
	// FIXME: How do we get round info now?
	jww.INFO.Printf("Round UNK received, %v real messages "+
		"processed, ??? dummies ignored", numReal)

	go PrintProfilingStatistics()
}

// Start sets up the threads and network server to run the gateway
func (gw *Instance) Start() {
	// Now that we're set up, run a thread that constantly
	// polls for updates
	go func() {
		lastUpdate := uint64(time.Now().Unix())
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			msg, err := PollServer(gw.Comms,
				gw.ServerHost,
				gw.NetInf.GetFullNdf(),
				gw.NetInf.GetPartialNdf(),
				lastUpdate)
			lastUpdate = uint64(time.Now().Unix())
			if err != nil {
				jww.WARN.Printf(
					"Failed to Poll: %v",
					err)
				continue
			}
			gw.UpdateInstance(msg)
		}
	}()
}

// Notification Server polls Gateway for mobile notifications at this endpoint
func (gw *Instance) PollForNotifications(auth *connect.Auth) (i []*id.ID, e error) {
	// Check that authentication is good and the sender is our gateway, otherwise error
	if !auth.IsAuthenticated || auth.Sender.GetId() != &id.NotificationBot || auth.Sender.IsDynamicHost() {
		jww.WARN.Printf("PollForNotifications failed auth (sender ID: %s, auth: %v, expected: %s)",
			auth.Sender.GetId(), auth.IsAuthenticated, id.NotificationBot)
		return nil, connect.AuthError(auth.Sender.GetId())
	}
	return gw.un.Notified(), nil
}

// KillRateLimiter is a helper function which sends the kill
// signal to the gateway's rate limiter
func (gw *Instance) KillRateLimiter() {
	gw.rateLimitQuit <- struct{}{}

}
