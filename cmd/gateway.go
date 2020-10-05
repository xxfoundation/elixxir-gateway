///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
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
	"gitlab.com/elixxir/primitives/states"
	"gitlab.com/elixxir/primitives/utils"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/ndf"
	"strconv"
	"strings"
	"time"
)

var dummyUser = id.DummyUser

// Errors to suppress
const (
	ErrInvalidHost = "Invalid host ID:"
	ErrAuth        = "Failed to authenticate id:"
)

type Instance struct {
	// Storage buffer for messages to be submitted to the network
	UnmixedBuffer storage.UnmixedMessageBuffer

	// Contains all Gateway relevant fields
	Params Params

	// Contains Server Host Information
	ServerHost *connect.Host

	// Gateway object created at start
	Comms *gateway.Comms

	// Map of leaky buckets for user IDs
	rateLimitQuit chan struct{}
	rateLimit     *rateLimiting.BucketMap

	// struct for tracking notifications
	un notifications.UserNotifications

	// Tracker of the gateway's known rounds
	knownRound *knownRounds.KnownRounds

	storage *storage.Storage
	// TODO: Integrate and remove duplication with the stuff above.
	// NetInf is the network interface for working with the NDF poll
	// functionality in comms.
	NetInf        *network.Instance
	addGateway    chan network.NodeGateway
	removeGateway chan *id.ID

	lastUpdate uint64
}

func (gw *Instance) GetBloom(msg *pb.GetBloom, ipAddress string) (*pb.GetBloomResponse, error) {
	panic("implement me")
}

// Handler for a client's poll to a gateway. Returns all the last updates and known rounds
func (gw *Instance) Poll(clientRequest *pb.GatewayPoll) (
	*pb.GatewayPollResponse, error) {
	// Nil check to check for valid clientRequest
	if clientRequest == nil {
		return &pb.GatewayPollResponse{}, errors.Errorf(
			"Poll() clientRequest is empty")
	}
	if clientRequest.ClientID == nil {
		return &pb.GatewayPollResponse{}, errors.Errorf(
			"Poll() clientRequest.ClientID required")
	}

	lastKnownRound := gw.NetInf.GetLastRoundID()

	// Get the range of updates from the network instance
	updates, err := gw.NetInf.GetHistoricalRoundRange(
		id.Round(clientRequest.LastUpdate), lastKnownRound)
	if err != nil {
		errStr := fmt.Sprintf("couldn't get updates for client "+
			"[%v]'s request4: %v", clientRequest.ClientID, err)
		jww.WARN.Printf(errStr)
		return &pb.GatewayPollResponse{}, errors.New(errStr)
	}

	kr, err := gw.knownRound.Marshal()
	if err != nil {
		errStr := fmt.Sprintf("couldn't get known rounds for client "+
			"[%v]'s request: %v", clientRequest.ClientID, err)
		jww.WARN.Printf(errStr)
		return &pb.GatewayPollResponse{}, errors.New(errStr)

	}

	return &pb.GatewayPollResponse{
		PartialNDF:       gw.NetInf.GetPartialNdf().GetPb(),
		Updates:          updates,
		LastTrackedRound: uint64(lastKnownRound),
		KnownRounds:      kr,
		FilterNew:        nil,
		FilterOld:        nil,
	}, nil
}

// NewGatewayInstance initializes a gateway Handler interface
func NewGatewayInstance(params Params) *Instance {
	newDatabase, _, err := storage.NewStorage(params.DbUsername,
		params.DbPassword,
		params.DbName,
		params.DbAddress,
		params.DbPort,
	)
	if err != nil {
		jww.WARN.Printf("Could not initialize database")
	}
	i := &Instance{
		UnmixedBuffer: storage.NewUnmixedMessagesMap(),
		Params:        params,
		storage:       newDatabase,
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
	// Client -> Gateway historical round request
	impl.Functions.RequestHistoricalRounds = func(msg *pb.HistoricalRounds) (response *pb.HistoricalRoundsResponse, err error) {
		return instance.RequestHistoricalRounds(msg)
	}
	// Client -> Gateway message request
	impl.Functions.RequestMessages = func(msg *pb.GetMessages) (*pb.GetMessagesResponse, error) {
		return instance.RequestMessages(msg)
	}
	// Client -> Gateway bloom request
	impl.Functions.RequestBloom = func(msg *pb.GetBloom) (*pb.GetBloomResponse, error) {
		return instance.RequestBloom(msg)
	}
	impl.Functions.Poll = func(msg *pb.GatewayPoll) (response *pb.GatewayPollResponse, err error) {
		return instance.Poll(msg)
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
	ers := &storage.ERS{}
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
			if update.UpdateID > gw.lastUpdate {
				gw.lastUpdate = update.UpdateID
			}
			// Parse the topology into an id list
			idList, err := id.NewIDListFromBytes(update.Topology)
			if err != nil {
				return err
			}

			// Convert the ID list to a circuit
			topology := ds.NewCircuit(idList)

			// Chek if our node is the entry point fo the circuit
			if topology.IsFirstNode(gw.ServerHost.GetId()) {
				gw.UnmixedBuffer.SetAsRoundLeader(id.Round(update.ID), update.BatchSize)
			}

			err = gw.NetInf.RoundUpdate(update)
			if err != nil {
				return err
			}

			if roundState := states.Round(update.State); roundState == states.COMPLETED || roundState == states.FAILED {
				gw.knownRound.Check(id.Round(update.ID))
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
	permissioningCert, err = utils.ReadFile(gw.Params.PermissioningCertPath)
	if err != nil {
		return errors.WithMessagef(err,
			"Failed to read permissioning cert at %v",
			gw.Params.PermissioningCertPath)
	}

	// Set up temporary gateway listener
	gatewayHandler := NewImplementation(gw)
	gw.Comms = gateway.StartGateway(&id.TempGateway, address, gatewayHandler,
		gwCert, gwKey, gossip.DefaultManagerFlags())

	// Set up temporary server host
	// (id, address string, cert []byte, disableTimeout, enableAuth bool)
	dummyServerID := id.DummyUser.DeepCopy()
	dummyServerID.SetType(id.Node)
	params := connect.GetDefaultHostParams()
	params.MaxRetries = 0
	gw.ServerHost, err = connect.NewHost(dummyServerID, gw.Params.NodeAddress,
		nodeCert, params)
	if err != nil {
		return errors.Errorf("Unable to create tmp server host: %+v",
			err)
	}

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
		params := connect.GetDefaultHostParams()
		params.MaxRetries = 0
		_, err = gw.Comms.AddHost(&id.Permissioning, "", permissioningCert, params)
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
		params = connect.GetDefaultHostParams()
		params.MaxRetries = 0
		gw.ServerHost, err = connect.NewHost(serverID.DeepCopy(), gw.Params.NodeAddress, nodeCert,
			params)
		if err != nil {
			return errors.Errorf(
				"Unable to create updated server host: %+v", err)
		}

		gatewayId := serverID
		gatewayId.SetType(id.Gateway)
		gw.Comms = gateway.StartGateway(gatewayId, address, gatewayHandler,
			gwCert, gwKey, gossip.DefaultManagerFlags())
		gw.InitGossip()

		// Initialize hosts for reverse-authentication
		// This may be necessary to verify the NDF if it gets updated while
		// the network is up
		_, err = gw.Comms.AddHost(&id.Permissioning, "", permissioningCert, params)
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

	return nil
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

// TODO: Refactor to get messages once the old endpoint is ready to be fully deprecated
// Client -> Gateway handler. Looks up messages based on a userID and a roundID.
// If the gateway participated in this round, and the requested client had messages in that round,
// we return these message(s) to the requester
func (gw *Instance) RequestMessages(msg *pb.GetMessages) (*pb.GetMessagesResponse, error) {
	// Error check for a invalidly crafted message
	if msg == nil || msg.ClientID == nil || msg.RoundID == 0 {
		return &pb.GetMessagesResponse{}, errors.New("Could not parse message! " +
			"Please try again with a properly crafted message!")
	}

	// Parse the requested clientID within the message for the database request
	userId, err := id.Unmarshal(msg.ClientID)
	if err != nil {
		return &pb.GetMessagesResponse{}, errors.New("Could not parse requested user ID!")
	}

	// Parse the roundID within the message
	// Fixme: double check that it is in fact bigEndian
	roundID := id.Round(msg.RoundID)

	// Search the database for the requested messages
	msgs, err := gw.storage.GetMixedMessages(userId, roundID)
	if err != nil {
		return &pb.GetMessagesResponse{
				HasRound: true,
			}, errors.Errorf("Could not find any MixedMessages with "+
				"recipient ID %v and round ID %v.", userId, roundID)
	}

	// Parse the database response to construct individual slots
	var slots []*pb.Slot
	for _, msg := range msgs {
		// Get the message contents
		payloadA, payloadB := msg.GetMessageContents()
		// Construct the slot and place in the list
		data := &pb.Slot{
			PayloadA: payloadA,
			PayloadB: payloadB,
		}
		slots = append(slots, data)
	}

	// Return all messages to the requester
	return &pb.GetMessagesResponse{
		HasRound: true,
		Messages: slots,
	}, nil

}

// Returns message contents for MessageID, or a null/randomized message
// if that ID does not exist of the same size as a regular message
func (gw *Instance) GetMessage(userID *id.ID, msgID string, ipAddress string) (*pb.Slot, error) {
	// Fixme: populate function with requestMessage logic when comms is ready to be refactored
	return &pb.Slot{}, nil
}

// Return any MessageIDs in the globals for this User
func (gw *Instance) CheckMessages(userID *id.ID, msgID string, ipAddress string) ([]string, error) {
	jww.DEBUG.Printf("Getting message IDs for %q after %s from buffer...",
		userID, msgID)

	msgs, err := gw.storage.GetMixedMessages(userID, gw.NetInf.GetLastRoundID())
	if err != nil {
		return nil, errors.Errorf("Could not look up message ids")
	}

	// Parse the message ids returned and send back to sender
	var msgIds []string
	for _, msg := range msgs {
		data := strconv.FormatUint(msg.Id, 10)
		msgIds = append(msgIds, data)
	}
	return msgIds, nil
}

// RequestHistoricalRounds retrieves all rounds requested within the HistoricalRounds
// message from the gateway's database. A list of round info messages are returned
// to the sender
func (gw *Instance) RequestHistoricalRounds(msg *pb.HistoricalRounds) (*pb.HistoricalRoundsResponse, error) {
	// Nil check external messages to avoid potential crashes
	if msg == nil || msg.Rounds == nil {
		return &pb.HistoricalRoundsResponse{}, errors.New("Invalid historical" +
			" round request, could not look up rounds. Please send a valid message.")
	}

	// Parse the message for all requested rounds
	var roundIds []id.Round
	for _, rnd := range msg.Rounds {
		roundIds = append(roundIds, id.Round(rnd))
	}
	// Look up requested rounds in the database
	retrievedRounds, err := gw.storage.GetRounds(roundIds)
	if err != nil {
		return &pb.HistoricalRoundsResponse{}, errors.New("Could not look up rounds requested.")
	}

	// Parse the retrieved rounds into the roundInfo message type
	// Fixme: there is a back and forth type casting going on between placing
	//  data into the database per the spec laid out
	//  and taking that data out and casting it back to the original format.
	//  it's really dumb and shouldn't happen, it should be fixed.
	var rounds []*pb.RoundInfo
	for _, rnd := range retrievedRounds {
		ri := &pb.RoundInfo{}
		err = proto.Unmarshal(rnd.InfoBlob, ri)
		if err != nil {
			// If trouble unmarshalling, move to next round
			// Note this should never happen with
			// rounds placed by us in our own database
			jww.WARN.Printf("Could not unmarshal round %d in our database. "+
				"Could the database be corrupted?", rnd.Id)
			continue
		}

		rounds = append(rounds, ri)
	}

	// Return the retrievedRounds
	return &pb.HistoricalRoundsResponse{
		Rounds: rounds,
	}, nil

}

// PutMessage adds a message to the outgoing queue
func (gw *Instance) PutMessage(msg *pb.GatewaySlot, ipAddress string) (*pb.GatewaySlotResponse, error) {
	// Fixme: work needs to be done to populate database with precanned values
	//  so that precanned users aren't rejected when sending messages
	// Construct Client ID for database lookup
	clientID, err := id.Unmarshal(msg.Message.SenderID)
	if err != nil {
		return &pb.GatewaySlotResponse{
			Accepted: false,
		}, errors.Errorf("Could not parse message: Unrecognized ID")
	}

	//Retrieve the client from the database
	cl, err := gw.storage.GetClient(clientID)
	if err != nil {
		return &pb.GatewaySlotResponse{
			Accepted: false,
		}, errors.New("Did not recognize ID. Have you registered successfully?")
	}

	//Generate the MAC and check against the message's MAC
	clientMac := generateClientMac(cl, msg)
	if !bytes.Equal(clientMac, msg.MAC) {
		return &pb.GatewaySlotResponse{
			Accepted: false,
		}, errors.New("Could not authenticate client. Please try again later")
	}
	thisRound := id.Round(msg.RoundID)

	// Check if we manage this round
	if !gw.UnmixedBuffer.IsRoundLeader(thisRound) {
		return &pb.GatewaySlotResponse{Accepted: false}, errors.Errorf("Could not find round. " +
			"Please try a different gateway.")
	}

	if gw.UnmixedBuffer.IsRoundFull(thisRound) {
		return &pb.GatewaySlotResponse{Accepted: false}, errors.Errorf("This round is full and " +
			"will not accept any new messages. Please try a different round.")
	}

	gw.UnmixedBuffer.AddUnmixedMessage(msg.Message, thisRound)

	// Rate limit messages
	senderId, err := id.Unmarshal(msg.GetMessage().GetSenderID())
	if err != nil {
		return nil, errors.Errorf("Unable to unmarshal sender ID: %+v", err)
	}
	err = gw.FilterMessage(senderId)
	if err != nil {
		jww.INFO.Printf("Rate limiting check failed on send message from "+
			"%v", msg.Message.GetSenderID())
		return &pb.GatewaySlotResponse{
			Accepted: false,
		}, err
	}

	jww.DEBUG.Printf("Putting message from user %v in outgoing queue...",
		msg.Message.GetSenderID())

	return &pb.GatewaySlotResponse{
		Accepted: true,
		RoundID:  msg.GetRoundID(),
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
	jww.INFO.Print("Passing on registration nonce request")
	return gw.Comms.SendRequestNonceMessage(gw.ServerHost, msg)

}

// Pass-through for Registration Nonce Confirmation
func (gw *Instance) ConfirmNonce(msg *pb.RequestRegistrationConfirmation,
	ipAddress string) (*pb.RegistrationConfirmation, error) {

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

	err = gw.storage.InsertClient(newClient)
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

	batch := gw.UnmixedBuffer.GetRoundMessages(minMsgCnt, id.Round(roundInfo.ID))
	for batch == nil {
		jww.INFO.Printf(
			"Server is ready, but only have %d messages to send, "+
				"need %d! Waiting 1 seconds!",
			gw.UnmixedBuffer.LenUnmixed(id.Round(roundInfo.ID)),
			minMsgCnt)
		time.Sleep(1 * time.Second)
		batch = gw.UnmixedBuffer.GetRoundMessages(minMsgCnt, id.Round(roundInfo.ID))
	}

	batch.Round = roundInfo

	jww.INFO.Printf("Sending batch with %d messages...", len(batch.Slots))

	numNodes := len(roundInfo.GetTopology())

	if numNodes == 0 {
		jww.ERROR.Println("Round topology empty, sending bad messages!")
	}

	// Now fill with junk and send
	for i := uint64(len(batch.Slots)); i < batchSize; i++ {
		junkMsg := GenJunkMsg(gw.NetInf.GetCmixGroup(), numNodes,
			uint32(i))
		batch.Slots = append(batch.Slots, junkMsg)
	}

	// Send the completed batch
	err := gw.Comms.PostNewBatch(gw.ServerHost, batch)
	if err != nil {
		// TODO: handle failure sending batch
		jww.WARN.Printf("Error while sending batch: %v", err)
	}

	// Gossip senders included in the batch to other gateways
	err = gw.GossipBatch(batch)
	if err != nil {
		jww.WARN.Printf("Unable to gossip batch information: %+v", err)
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
			jww.DEBUG.Printf("Message Received for: %s",
				userId)

			gw.un.Notify(userId)
			numReal++
			h.Write(msg.PayloadA)
			h.Write(msg.PayloadB)
			msgID := binary.BigEndian.Uint64(h.Sum(nil))
			roundID := gw.NetInf.GetLastRoundID()

			// Create new message and insert into database
			newMsg := storage.NewMixedMessage(&roundID, userId, msg.PayloadA, msg.PayloadB)
			newMsg.Id = msgID
			err = gw.storage.InsertMixedMessage(newMsg)
			if err != nil {
				jww.ERROR.Printf("Inserting a new mixed message failed in "+
					"ProcessCompletedBatch: %+v", err)
			}
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
		//fix-me: this last update needs to be persistant across resets
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			msg, err := PollServer(gw.Comms,
				gw.ServerHost,
				gw.NetInf.GetFullNdf(),
				gw.NetInf.GetPartialNdf(),
				gw.lastUpdate)
			if err != nil {
				jww.WARN.Printf(
					"Failed to Poll: %v",
					err)
				continue
			}
			err = gw.UpdateInstance(msg)
			if err != nil {
				jww.WARN.Printf("Unable to update instance: %+v", err)
			}
		}
	}()
}

// FilterMessage determines if the message should be kept or discarded based on
// the capacity of the sender's ID bucket.
func (gw *Instance) FilterMessage(userId *id.ID) error {
	// If the user ID bucket is full AND the message's user ID is not on the
	// whitelist, then reject the message
	if !gw.rateLimit.LookupBucket(userId.String()).Add(1) {
		return errors.New("Rate limit exceeded. Try again later.")
	}

	// Otherwise, if the user ID bucket has room then let the message through
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

// Client -> Gateway bloom request
func (gw *Instance) RequestBloom(msg *pb.GetBloom) (*pb.GetBloomResponse, error) {
	return nil, nil
}
