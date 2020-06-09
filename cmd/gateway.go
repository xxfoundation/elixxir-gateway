////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
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
	"gitlab.com/elixxir/crypto/signature/rsa"
	"gitlab.com/elixxir/crypto/xx"
	"gitlab.com/elixxir/gateway/notifications"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/elixxir/primitives/format"
	"gitlab.com/elixxir/primitives/id"
	"gitlab.com/elixxir/primitives/ndf"
	"gitlab.com/elixxir/primitives/rateLimiting"
	"gitlab.com/elixxir/primitives/utils"
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

var IPWhiteListArr = []string{"test"}

type Instance struct {
	// Storage buffer for messages after being processed by the network
	MixedBuffer storage.MixedMessageBuffer
	// Storage buffer for messages to be submitted to the network
	UnmixedBuffer storage.UnmixedMessageBuffer

	// Contains all Gateway relevant fields
	Params Params

	// Contains system NDF
	Ndf *ndf.NetworkDefinition

	// Contains Server Host Information
	ServerHost *connect.Host

	// Gateway object created at start
	Comms *gateway.Comms

	// Map of leaky buckets for IP addresses
	ipBuckets *rateLimiting.BucketMap
	// Map of leaky buckets for user IDs
	userBuckets *rateLimiting.BucketMap
	// Whitelist of IP addresses
	ipWhitelist *rateLimiting.Whitelist
	// Whitelist of IP addresses
	userWhitelist *rateLimiting.Whitelist

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
	CMixNodes   []string
	NodeAddress string
	Port        int
	Address     string
	CertPath    string
	KeyPath     string

	ServerCertPath        string
	IDFPath               string
	PermissioningCertPath string

	FirstNode bool
	LastNode  bool

	IpBucket   rateLimiting.Params
	UserBucket rateLimiting.Params

	MessageTimeout time.Duration
}

// NewGatewayInstance initializes a gateway Handler interface
func NewGatewayInstance(params Params) *Instance {

	i := &Instance{
		MixedBuffer:   storage.NewMixedMessageBuffer(params.MessageTimeout),
		UnmixedBuffer: storage.NewUnmixedMessageBuffer(),
		Params:        params,

		ipBuckets:   rateLimiting.CreateBucketMapFromParams(params.IpBucket),
		userBuckets: rateLimiting.CreateBucketMapFromParams(params.UserBucket),
	}

	err := rateLimiting.CreateWhitelistFile(params.IpBucket.WhitelistFile,
		IPWhiteListArr)

	if err != nil {
		jww.WARN.Printf("Could not load whitelist: %s", err)
	}

	whitelistTemp, err := rateLimiting.InitWhitelist(params.IpBucket.WhitelistFile,
		nil)
	if err != nil {
		jww.ERROR.Printf("Could not load initiate whitelist: %s", err)
	}

	i.ipWhitelist = whitelistTemp

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
	impl.Functions.PutMessage = func(message *pb.Slot, ipaddress string) error {
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
	return network.NewInstance(pc, newNdf.Get(), newPartialNdf.Get())
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

	// TLS-enabled pathway
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
		var gatewayCert []byte
		var nodeId []byte

		for gatewayCert == nil {
			// TODO: Probably not great to always sleep immediately
			time.Sleep(3 * time.Second)

			// Poll Server for the NDFs, then use it to create the
			// network instance and begin polling for server updates
			msg, err := PollServer(gw.Comms, gw.ServerHost, nil, nil, 0)
			if err != nil {
				// Catch recoverable error
				if strings.Contains(err.Error(),
					"Invalid host ID:") {
					jww.WARN.Printf(
						"Server not yet "+
							"ready...: %s",
						err)
					continue
				} else if strings.Contains(err.Error(),
					ndf.NO_NDF) {
					continue
				} else {
					return errors.Errorf(
						"Error polling NDF: %+v", err)
				}
			}

			jww.DEBUG.Printf("Creating instance!")
			gw.NetInf, err = CreateNetworkInstance(gw.Comms,
				msg.FullNDF,
				msg.PartialNDF)
			if err != nil {
				jww.ERROR.Printf("Unable to create network"+
					" instance: %v", err)
				continue
			}
			_, err = gw.Comms.AddHost(&id.Permissioning, "", permissioningCert, true, true)
			if err != nil {
				jww.ERROR.Printf("Couldn't add permissioning host to comms: %v", err)
				continue
			}
			err = gw.UpdateInstance(msg)
			if err != nil {
				jww.ERROR.Printf("Update instance error: %v", err)
				continue
			}

			// Install the NDF once we get it
			if msg.FullNDF != nil && msg.Id != nil {
				gatewayCert, err = gw.installNdf(msg.FullNDF.Ndf, msg.Id)
				nodeId = msg.Id
				if err != nil {
					jww.DEBUG.Printf("failed to install ndf: %+v", err)
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
		serverID, err2 := id.Unmarshal(nodeId)
		if err2 != nil {
			jww.ERROR.Printf("Unmarshalling serverID failed during network "+
				"init: %+v", err2)
		}

		gatewayId := serverID
		gatewayId.SetType(id.Gateway)
		gw.Comms = gateway.StartGateway(gatewayId, address, gatewayHandler, gatewayCert, gwKey)

		// Initialize hosts for reverse-authentication
		// This may be necessary to verify the NDF if it gets updated while
		// the network is up
		_, err = gw.Comms.AddHost(&id.Permissioning, "", permissioningCert, true, true)
		if err != nil {
			return errors.Errorf("Couldn't add permissioning host: %v", err)
		}
		_, err = gw.Comms.AddHost(&id.NotificationBot, gw.Ndf.Notification.Address,
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
			serverID, err2 := id.Unmarshal(node.ID)
			if err2 != nil {
				jww.ERROR.Printf("Unmarshalling serverID failed while "+
					"installing the NDF: %+v", err2)
			}
			gw.ServerHost, err = connect.NewHost(serverID, gw.Params.NodeAddress, []byte(node.TlsCertificate),
				true, true)
			if err != nil {
				return nil, errors.Errorf(
					"Unable to create updated server host: %+v", err)
			}

			// Save the IDF to the idfPath
			err := writeIDF(gw.Ndf, i, idfPath)
			if err != nil {
				jww.WARN.Printf("Could not write ID File: %s",
					idfPath)
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
func (gw *Instance) GetMessage(userID *id.ID, msgID string, ipAddress string) (*pb.Slot, error) {
	// disabled from rate limiting for now
	/*uIDStr := hex.EncodeToString(userID.Bytes())
	err := gw.FilterMessage(uIDStr, ipAddress, TokensGetMessage)

	if err != nil {
		jww.INFO.Printf("Rate limiting check failed on get message from %s", uIDStr)
		return nil, err
	}*/

	jww.DEBUG.Printf("Getting message %q:%s from buffer...", *userID, msgID)
	return gw.MixedBuffer.GetMixedMessage(userID, msgID)
}

// Return any MessageIDs in the globals for this User
func (gw *Instance) CheckMessages(userID *id.ID, msgID string, ipAddress string) ([]string, error) {
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
	return gw.MixedBuffer.GetMixedMessageIDs(userID, msgID)
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
	gw.UnmixedBuffer.AddUnmixedMessage(msg)
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

	senderID, err := xx.NewID(userPublicKey, msg.Salt, id.User)

	if err != nil {
		return nil, err
	}

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

	// Now fill with junk and send
	for i := uint64(len(batch.Slots)); i < batchSize; i++ {
		junkMsg := GenJunkMsg(gw.NetInf.GetCmixGroup(), len(gw.Params.CMixNodes),
			uint32(i))
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
		userId, err := serialmsg.GetRecipient()

		if err != nil {
			jww.ERROR.Printf("Creating userId from serialmsg failed in "+
				"ProcessCompletedBatch: %+v", err)
		}

		if !userId.Cmp(&dummyUser) {
			jww.DEBUG.Printf("Message Recieved for: %v",
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
func (gw *Instance) PollForNotifications(auth *connect.Auth) (i []*id.ID, e error) {
	// Check that authentication is good and the sender is our gateway, otherwise error
	if !auth.IsAuthenticated || auth.Sender.GetId() != &id.NotificationBot || auth.Sender.IsDynamicHost() {
		jww.WARN.Printf("PollForNotifications failed auth (sender ID: %s, auth: %v, expected: %s)",
			auth.Sender.GetId(), auth.IsAuthenticated, id.NotificationBot)
		return nil, connect.AuthError(auth.Sender.GetId())
	}
	return gw.un.Notified(), nil
}
