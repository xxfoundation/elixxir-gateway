////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/comms/connect"
	"gitlab.com/elixxir/comms/gateway"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	ds "gitlab.com/elixxir/comms/network/dataStructures"
	"gitlab.com/elixxir/crypto/cmix"
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/crypto/large"
	"gitlab.com/elixxir/crypto/registration"
	"gitlab.com/elixxir/crypto/signature/rsa"
	"gitlab.com/elixxir/gateway/notifications"
	"gitlab.com/elixxir/gateway/rateLimiting"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/elixxir/primitives/format"
	"gitlab.com/elixxir/primitives/id"
	"gitlab.com/elixxir/primitives/ndf"
	"gitlab.com/elixxir/primitives/utils"
	"strings"
	"time"
)

var dummyUser = id.MakeDummyUserID()

var rateLimitErr = errors.New("Client has exceeded communications rate limit")

// Tokens required by clients for different messages
const TokensPutMessage = uint(250)  // Sends a message, the networks does n * 5 exponentiations, n = 5, 25
const TokensRequestNonce = uint(30) // Requests a nonce from the node to verify the user, 3 exponentiations
const TokensConfirmNonce = uint(10) // Requests a nonce from the node to verify the user, 1 exponentiation

var IPWhiteListArr = []string{"test"}

type Instance struct {
	// Storage buffer for inbound/outbound messages
	Buffer storage.MessageBuffer

	// Contains all Gateway relevant fields
	Params Params

	// Contains system NDF
	Ndf *ndf.NetworkDefinition

	// Contains Server Host Information
	ServerHost *connect.Host

	// Gateway object created at start
	Comms *gateway.Comms

	//Group that cmix operates within
	CmixGrp *cyclic.Group

	// Map of leaky buckets for IP addresses
	ipBuckets rateLimiting.BucketMap
	// Map of leaky buckets for user IDs
	userBuckets rateLimiting.BucketMap
	// Whitelist of IP addresses
	ipWhitelist rateLimiting.Whitelist
	// Whitelist of IP addresses
	userWhitelist rateLimiting.Whitelist

	// struct for tracking notifications
	un notifications.UserNotifications

	// TODO: Integrate and remove duplication with the stuff above.
	// NetInf is the network interface for working with the NDF poll
	// functionality in comms.
	NetInf *network.Instance
}

func (gw *Instance) Poll(*pb.GatewayPoll) (*pb.GatewayPollResponse, error) {
	jww.FATAL.Panicf("Unimplemented!")
	return nil, nil
}

type Params struct {
	BatchSize   uint64
	CMixNodes   []string
	NodeAddress string
	Port        int
	Address     string
	CertPath    string
	KeyPath     string

	ServerCertPath string
	CmixGrp        map[string]string

	FirstNode bool
	LastNode  bool

	rateLimiting.Params
}

// NewGatewayInstance initializes a gateway Handler interface
func NewGatewayInstance(params Params) *Instance {
	p := large.NewIntFromString(params.CmixGrp["prime"], 16)
	g := large.NewIntFromString(params.CmixGrp["generator"], 16)
	grp := cyclic.NewGroup(p, g)

	i := &Instance{
		Buffer:  storage.NewMessageBuffer(),
		Params:  params,
		CmixGrp: grp,

		ipBuckets: rateLimiting.CreateBucketMap(
			params.IpCapacity, params.IpLeakRate,
			params.CleanPeriod, params.MaxDuration,
		),

		userBuckets: rateLimiting.CreateBucketMap(
			params.UserCapacity, params.UserLeakRate,
			params.CleanPeriod, params.MaxDuration,
		),
	}

	err := rateLimiting.CreateWhitelistFile(params.IpWhitelistFile,
		IPWhiteListArr)

	if err != nil {
		jww.WARN.Printf("Could not load whitelist: %s", err)
	}

	i.ipWhitelist = *rateLimiting.InitWhitelist(params.IpWhitelistFile, nil)

	return i
}

func NewImplementation(instance *Instance) *gateway.Implementation {
	impl := gateway.NewImplementation()
	impl.Functions.CheckMessages = func(userID *id.User, messageID, ipaddress string) (i []string, b error) {
		return instance.CheckMessages(userID, messageID, ipaddress)
	}
	impl.Functions.ConfirmNonce = func(message *pb.RequestRegistrationConfirmation, ipaddress string) (confirmation *pb.RegistrationConfirmation, e error) {
		return instance.ConfirmNonce(message, ipaddress)
	}
	impl.Functions.GetMessage = func(userID *id.User, msgID, ipaddress string) (slot *pb.Slot, b error) {
		return instance.GetMessage(userID, msgID, ipaddress)
	}
	impl.Functions.PutMessage = func(message *pb.Slot, ipaddress string) error {
		return instance.PutMessage(message, ipaddress)
	}
	impl.Functions.RequestNonce = func(message *pb.NonceRequest, ipaddress string) (nonce *pb.Nonce, e error) {
		return instance.RequestNonce(message, ipaddress)
	}
	impl.Functions.PollForNotifications = func(auth *connect.Auth) (i []string, e error) {
		return instance.PollForNotifications(auth)
	}
	return impl
}

// PollServer sends a poll message to the server and returns a response.
func PollServer(conn *gateway.Comms, pollee *connect.Host, ndf,
	partialNdf *network.SecuredNdf, lastUpdate uint64) (
	*pb.ServerPollResponse, error) {

	var ndfHash, partialNdfHash *pb.NDFHash
	ndfHash = nil
	partialNdfHash = nil
	if ndf != nil {
		ndfHash = &pb.NDFHash{Hash: ndf.GetHash()}
	}
	if partialNdf != nil {
		partialNdfHash = &pb.NDFHash{Hash: partialNdf.GetHash()}
	}
	pollMsg := &pb.ServerPoll{
		Full:       ndfHash,
		Partial:    partialNdfHash,
		LastUpdate: lastUpdate,
		Error:      "",
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
	newNdf.Update(ndf)
	newPartialNdf.Update(partialNdf)
	pc := conn.ProtoComms
	return network.NewInstance(pc, newNdf.Get(), newPartialNdf.Get())
}

// UpdateInstance reads a ServerPollResponse object and updates the instance
// state accordingly.
func (gw *Instance) UpdateInstance(newInfo *pb.ServerPollResponse) {
	// Update the NDFs, and update the round info, which is currently
	// recorded but not used for anything. (maybe we should print state
	// of each round?)
	if newInfo.FullNDF != nil {
		gw.NetInf.UpdateFullNdf(newInfo.FullNDF)
	}
	if newInfo.PartialNDF != nil {
		gw.NetInf.UpdatePartialNdf(newInfo.PartialNDF)
	}
	if newInfo.Updates != nil {
		for _, update := range newInfo.Updates {
			gw.NetInf.RoundUpdate(update)
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
}

// InitNetwork initializes the network on this gateway instance
// After the network object is created, you need to use it to connect
// to the corresponding server in the network using ConnectToNode.
// Additionally, to clean up the network object (especially in tests), call
// Shutdown() on the network object.
func (gw *Instance) InitNetwork() error {
	address := fmt.Sprintf("%s:%d", gw.Params.Address, gw.Params.Port)
	var err error
	var gwCert, gwKey, nodeCert []byte

	// TLS-enabled pathway
	if !noTLS {
		gwCert, err = utils.ReadFile(gw.Params.CertPath)
		if err != nil {
			return errors.New(fmt.Sprintf("Failed to read certificate at %s: %+v",
				gw.Params.CertPath, err))
		}
		gwKey, err = utils.ReadFile(gw.Params.KeyPath)
		if err != nil {
			return errors.New(fmt.Sprintf("Failed to read gwKey at %s: %+v",
				gw.Params.KeyPath, err))
		}
		nodeCert, err = utils.ReadFile(gw.Params.ServerCertPath)
		if err != nil {
			return errors.New(fmt.Sprintf(
				"Failed to read server gwCert at %s: %+v", gw.Params.ServerCertPath, err))
		}
	}

	// Set up temporary gateway listener
	gatewayHandler := NewImplementation(gw)
	gw.Comms = gateway.StartGateway(id.NewTmpGateway().String(), address, gatewayHandler, gwCert, gwKey)

	// If we are in the TLS-disabled pathway, we inherently want to disable
	// authentication
	if noTLS {
		gw.Comms.DisableAuth()
	}

	// Set up temporary server host
	//(id, address string, cert []byte, disableTimeout, enableAuth bool)
	gw.ServerHost, err = connect.NewHost("node", gw.Params.NodeAddress,
		nodeCert, true, true)
	if err != nil {
		return errors.Errorf("Unable to create tmp server host: %+v",
			err)
	}

	// Permissioning-enabled pathway
	if !disablePermissioning {
		if noTLS {
			return errors.Errorf("Cannot have permissioning on and TLS disabled")
		}

		// Begin polling server for NDF
		jww.INFO.Printf("Beginning polling NDF...")
		var gatewayCert []byte
		var nodeId []byte

		for gatewayCert == nil {
			// TODO: Probably not great to always sleep immediately
			time.Sleep(3 * time.Second)

			// Poll Server for the NDFs, then use it to create the
			// network instance and begin polling for server updates
			msg, err := PollServer(gw.Comms, gw.ServerHost,
				nil, nil, 0)

			if err != nil {
				// Catch recoverable error
				if strings.Contains(err.Error(),
					"Invalid host ID:") {
					jww.WARN.Printf(
						"Server not yet "+
							"ready...: %s",
						err)
					continue
				} else {
					return errors.Errorf(
						"Error polling NDF: %+v", err)
				}
			}

			gw.NetInf, err = CreateNetworkInstance(gw.Comms,
				msg.FullNDF,
				msg.PartialNDF)
			if err != nil {
				jww.ERROR.Printf("Unable to create network"+
					" instance: %v", err)
				continue
			}
			gw.UpdateInstance(msg)

			// Install the NDF once we get it
			if msg.FullNDF != nil && msg.Id != nil {
				gatewayCert, err = gw.installNdf(
					msg.FullNDF.Ndf, msg.Id)
				nodeId = msg.Id
				if err != nil {
					return err
				}
			}
		}
		jww.INFO.Printf("Successfully obtained NDF!")

		// Replace the comms server with the newly-signed certificate
		gw.Comms.Shutdown()

		// HACK HACK HACK
		// FIXME: coupling the connections with the server is horrible.
		// Technically the servers can fail to bind for up to
		// a couple minutes (depending on operating system), but
		// in practice 10 seconds works
		time.Sleep(10 * time.Second)
		gw.Comms = gateway.StartGateway(
			id.NewNodeFromBytes(nodeId).NewGateway().String(),
			address, gatewayHandler, gatewayCert, gwKey)

		// Initialize hosts for reverse-authentication
		_, err = gw.Comms.AddHost(id.NOTIFICATION_BOT, gw.Ndf.Notification.Address,
			[]byte(gw.Ndf.Notification.TlsCertificate), false, true)
		if err != nil {
			return errors.Errorf("Unable to add notifications host: %+v", err)
		}
	}

	return nil
}

// Helper that configures gateway instance according to the NDF
func (gw *Instance) installNdf(networkDef,
	nodeId []byte) (gatewayCert []byte, err error) {

	// Decode the NDF
	gw.Ndf, _, err = ndf.DecodeNDF(string(networkDef))
	if err != nil {
		return nil, errors.Errorf("Unable to decode NDF: %+v\n%+v", err,
			string(networkDef))
	}

	// Determine the index of this gateway
	for i, node := range gw.Ndf.Nodes {
		if bytes.Compare(node.ID, nodeId) == 0 {

			// Create the updated server host
			gw.ServerHost, err = connect.NewHost(id.NewNodeFromBytes(node.ID).String(),
				gw.Params.NodeAddress, []byte(node.TlsCertificate), true, true)
			if err != nil {
				return nil, errors.Errorf(
					"Unable to create updated server host: %+v", err)
			}

			// Configure gateway according to its node's index
			gw.Params.LastNode = i == len(gw.Ndf.Nodes)-1
			gw.Params.FirstNode = i == 0
			return []byte(gw.Ndf.Gateways[i].TlsCertificate), nil
		}
	}

	return nil, errors.Errorf("Unable to locate ID %v in NDF!", nodeId)
}

// Returns message contents for MessageID, or a null/randomized message
// if that ID does not exist of the same size as a regular message
func (gw *Instance) GetMessage(userID *id.User, msgID string, ipAddress string) (*pb.Slot, error) {
	//disabled from rate limiting for now
	/*uIDStr := hex.EncodeToString(userID.Bytes())
	err := gw.FilterMessage(uIDStr, ipAddress, TokensGetMessage)

	if err != nil {
		jww.INFO.Printf("Rate limiting check failed on get message from %s", uIDStr)
		return nil, err
	}*/

	jww.DEBUG.Printf("Getting message %q:%s from buffer...", *userID, msgID)
	return gw.Buffer.GetMixedMessage(userID, msgID)
}

// Return any MessageIDs in the globals for this User
func (gw *Instance) CheckMessages(userID *id.User, msgID string, ipAddress string) ([]string, error) {
	//disabled from rate limiting for now
	/*uIDStr := hex.EncodeToString(userID.Bytes())
	err := gw.FilterMessage(uIDStr, ipAddress, TokensCheckMessages)

	if err != nil {
		jww.INFO.Printf("Rate limiting check failed on check messages "+
			"from %s", uIDStr)
		return nil, err
	}*/

	jww.DEBUG.Printf("Getting message IDs for %q after %s from buffer...",
		userID, msgID)
	return gw.Buffer.GetMixedMessageIDs(userID, msgID)
}

// PutMessage adds a message to the outgoing queue and calls PostNewBatch when
// it's size is the batch size
func (gw *Instance) PutMessage(msg *pb.Slot, ipAddress string) error {

	err := gw.FilterMessage(hex.EncodeToString(msg.SenderID), ipAddress,
		TokensPutMessage)

	if err != nil {
		jww.INFO.Printf("Rate limiting check failed on send message from "+
			"%v", msg.GetSenderID())
		return err
	}

	jww.DEBUG.Printf("Putting message from user %v in outgoing queue...",
		msg.GetSenderID())
	gw.Buffer.AddUnmixedMessage(msg)
	return nil
}

// Pass-through for Registration Nonce Communication
func (gw *Instance) RequestNonce(msg *pb.NonceRequest, ipAddress string) (*pb.Nonce, error) {
	jww.INFO.Print("Checking rate limiting check on Nonce Request")
	userPublicKey, err := rsa.LoadPublicKeyFromPem([]byte(msg.ClientRSAPubKey))

	if err != nil {
		jww.ERROR.Printf("Unable to decode client RSA Pub Key: %+v", err)
		return nil, errors.New(fmt.Sprintf("Unable to decode client RSA Pub Key: %+v", err))
	}

	senderID := registration.GenUserID(userPublicKey, msg.Salt)

	//check rate limit
	err = gw.FilterMessage(hex.EncodeToString(senderID.Bytes()), ipAddress, TokensRequestNonce)

	if err != nil {
		return nil, err
	}

	jww.INFO.Print("Passing on registration nonce request")
	return gw.Comms.SendRequestNonceMessage(gw.ServerHost, msg)

}

// Pass-through for Registration Nonce Confirmation
func (gw *Instance) ConfirmNonce(msg *pb.RequestRegistrationConfirmation,
	ipAddress string) (*pb.RegistrationConfirmation, error) {

	err := gw.FilterMessage(hex.EncodeToString(msg.UserID), ipAddress, TokensConfirmNonce)

	if err != nil {
		return nil, err
	}

	jww.INFO.Print("Passing on registration nonce confirmation")
	return gw.Comms.SendConfirmNonceMessage(gw.ServerHost, msg)
}

// GenJunkMsg generates a junk message using the gateway's client key
func GenJunkMsg(grp *cyclic.Group, numNodes int) *pb.Slot {

	baseKey := grp.NewIntFromBytes((*dummyUser)[:])

	var baseKeys []*cyclic.Int

	for i := 0; i < numNodes; i++ {
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
func (gw *Instance) SendBatchWhenReady(roundInfo *pb.RoundInfo) {

	if roundInfo.BatchSize == 0 {
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

	batch := gw.Buffer.PopUnmixedMessages(minMsgCnt, gw.Params.BatchSize)
	for batch == nil {
		jww.INFO.Printf(
			"Server is ready, but only have %d messages to send, "+
				"need %d! Waiting 1 seconds!",
			gw.Buffer.LenUnmixed(),
			minMsgCnt)
		time.Sleep(1 * time.Second)
		batch = gw.Buffer.PopUnmixedMessages(minMsgCnt,
			gw.Params.BatchSize)
	}

	jww.INFO.Printf("Sending batch with real messages: %v", batch)

	// Now fill with junk and send
	for i := uint64(len(batch.Slots)); i < gw.Params.BatchSize; i++ {
		junkMsg := GenJunkMsg(gw.CmixGrp, len(gw.Params.CMixNodes))
		newJunkMsg := &pb.Slot{
			PayloadB: junkMsg.PayloadB,
			PayloadA: junkMsg.PayloadA,
			Salt:     junkMsg.Salt,
			SenderID: junkMsg.SenderID,
			KMACs:    junkMsg.KMACs,
		}

		batch.Slots = append(batch.Slots, newJunkMsg)
	}

	err := gw.Comms.PostNewBatch(gw.ServerHost, batch)
	if err != nil {
		// TODO: handle failure sending batch
		jww.WARN.Printf("Error while sending batch %v", err)

	}

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
		userId := serialmsg.GetRecipient()

		if !userId.Cmp(dummyUser) {
			jww.DEBUG.Printf("Message Recieved for: %v",
				userId.Bytes())
			gw.un.Notify(userId)
			numReal++
			h.Write(msg.PayloadA)
			h.Write(msg.PayloadB)
			msgId := base64.StdEncoding.EncodeToString(h.Sum(nil))
			gw.Buffer.AddMixedMessage(userId, msgId, msg)
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
		for {
			time.Sleep(250 * time.Millisecond)
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
			}
			gw.UpdateInstance(msg)
		}
	}()
}

// FilterMessage determines if the message should be kept or discarded based on
// the capacity of its related buckets. The message is kept if one or more of
// these three conditions is met:
//  1. Both its IP address bucket and user ID bucket have room.
//  2. Its IP address is on the whitelist and the user bucket has room/user ID
//     is on the whitelist.
//  2. If only the user ID is on the whitelist.
// TODO: re-enable user ID rate limiting after issues are fixed elsewhere
func (gw *Instance) FilterMessage(userId, ipAddress string, token uint) error {
	// If the IP address bucket is full AND the message's IP address is not on
	// the whitelist, then reject the message (unless user ID is on the
	// whitelist)
	if !gw.ipBuckets.LookupBucket(ipAddress).Add(token) && !gw.ipWhitelist.Exists(ipAddress) {
		// Checks if the user ID exists in the whitelists
		/*if gw.userWhitelist.Exists(userId) {
			return nil
		}*/

		return rateLimitErr
	}

	// If the user ID bucket is full AND the message's user ID is not on the
	// whitelist, then reject the message
	/*if !gw.userBuckets.LookupBucket(userId).Add(1) && !gw.userWhitelist.Exists(userId) {
		return errors.New("Rate limit exceeded. Try again later.")
	}*/

	// Otherwise, if the user ID bucket has room OR the user ID is on the
	// whitelist, then let the message through
	return nil
}

// Notification Server polls Gateway for mobile notifications at this endpoint
func (gw *Instance) PollForNotifications(auth *connect.Auth) (i []string, e error) {
	// Check that authentication is good and the sender is our gateway, otherwise error
	if !auth.IsAuthenticated || auth.Sender.GetId() != id.NOTIFICATION_BOT || auth.Sender.IsDynamicHost() {
		jww.WARN.Printf("PollForNotifications failed auth (sender ID: %s, auth: %v, expected: %s)",
			auth.Sender.GetId(), auth.IsAuthenticated, id.NOTIFICATION_BOT)
		return nil, connect.AuthError(auth.Sender.GetId())
	}
	return gw.un.Notified(), nil
}
