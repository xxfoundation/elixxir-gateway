///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"bytes"
	"encoding/base64"
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
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/ndf"
	"gitlab.com/xx_network/primitives/utils"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var dummyUser = id.DummyUser

// Errors to suppress
const (
	ErrInvalidHost = "Invalid host ID:"
	ErrAuth        = "Failed to authenticate id:"
	gwChanLen      = 1000
)

// The max number of rounds to be stored in the KnownRounds buffer.
const knownRoundsSize = 1000

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

	address           string
	bloomFilterGossip sync.Mutex
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

	// Check if the clientID is populated and valid
	clientId, err := id.Unmarshal(clientRequest.ClientID)
	if clientRequest.ClientID == nil || err != nil {
		return &pb.GatewayPollResponse{}, errors.Errorf(
			"Poll() clientRequest.ClientID required")
	}

	// lastKnownRound := gw.NetInf.GetLastRoundID()
	// Get the range of updates from the network instance
	if clientRequest == nil {
		errStr := fmt.Sprintf(
			"ClientRequest unset, cannot call GetRoundUpdates")
		jww.WARN.Printf(errStr)
		return &pb.GatewayPollResponse{}, errors.New(errStr)
	}
	updates := gw.NetInf.GetRoundUpdates(int(clientRequest.LastUpdate))

	kr, err := gw.knownRound.Marshal()
	if err != nil {
		errStr := fmt.Sprintf("couldn't get known rounds for client "+
			"[%v]'s request: %v", clientRequest.ClientID, err)
		jww.WARN.Printf(errStr)
		return &pb.GatewayPollResponse{}, errors.New(errStr)

	}

	// These errors are suppressed, as DB errors shouldn't go to client
	//  and if there is trouble getting filters returned, nil filters
	//  are returned to the client
	clientFilters, err := gw.storage.GetBloomFilters(clientId, id.Round(clientRequest.LastRound))
	jww.INFO.Printf("Adding %d client filters for %s", len(clientFilters), clientId)
	if err != nil {
		jww.WARN.Printf("Could not get filters for %s when polling: %v", clientId, err)
	}

	var filters [][]byte
	for _, f := range clientFilters {
		filters = append(filters, f.Filter)
	}

	jww.TRACE.Printf("KnownRounds: %v", kr)

	var netDef *pb.NDF
	isSame := gw.NetInf.GetPartialNdf().CompareHash(clientRequest.Partial.Hash)
	if !isSame {
		netDef = gw.NetInf.GetPartialNdf().GetPb()
	}

	return &pb.GatewayPollResponse{
		PartialNDF:       netDef,
		Updates:          updates,
		LastTrackedRound: uint64(0), // FIXME: This should be the
		// earliest tracked network round
		KnownRounds:  kr,
		BloomFilters: filters,
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
		eMsg := fmt.Sprintf("Could not initialize database: "+
			"psql://%s@%s:%s/%s", params.DbUsername,
			params.DbAddress, params.DbPort, params.DbName)
		if params.DevMode {
			jww.WARN.Printf(eMsg)
		} else {
			jww.FATAL.Panicf(eMsg)
		}
	}
	i := &Instance{
		UnmixedBuffer: storage.NewUnmixedMessagesMap(),
		Params:        params,
		storage:       newDatabase,
		knownRound:    knownRounds.NewKnownRound(knownRoundsSize),
	}

	// There is no round 0
	i.knownRound.Check(0)
	jww.DEBUG.Printf("Initial KnownRound State: %+v", i.knownRound)
	msh, _ := i.knownRound.Marshal()
	jww.DEBUG.Printf("Initial KnownRound Marshal: %s",
		string(msh))

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
	partialNdf *network.SecuredNdf, lastUpdate uint64, addr string) (
	*pb.ServerPollResponse, error) {
	jww.TRACE.Printf("Address being sent to server: [%v]", addr)

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
		GatewayAddress: addr,
		GatewayVersion: currentVersion,
	}

	resp, err := conn.SendPoll(pollee, pollMsg)
	return resp, err
}

// CreateNetworkInstance will generate a new network instance object given
// properly formed ndf, partialNdf, connection, and Storage object
func CreateNetworkInstance(conn *gateway.Comms, ndf, partialNdf *pb.NDF, ers *storage.Storage) (
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

	if err := gw.NetInf.UpdateGatewayConnections(); err != nil {
		jww.ERROR.Printf("Failed to update gateway connections: %+v",
			err)
	}

	if newInfo.Updates != nil {

		for _, update := range newInfo.Updates {
			jww.DEBUG.Printf("Processing Round Update: %s",
				SprintRoundInfo(update))
			if update.UpdateID > gw.lastUpdate {
				gw.lastUpdate = update.UpdateID

				// Save lastUpdate ID to file
				if err := gw.SaveLastUpdateID(); err != nil {
					jww.ERROR.Print(err)
				}
			}
			// Parse the topology into an id list
			idList, err := id.NewIDListFromBytes(update.Topology)
			if err != nil {
				return err
			}

			err = gw.NetInf.RoundUpdate(update)
			if err != nil {
				// do not return on round update failure, that will cause the
				// gateway to cease to process further updates, just warn
				jww.WARN.Printf("failed to insert round update: %s", err)
			}

			// Convert the ID list to a circuit
			topology := ds.NewCircuit(idList)

			// Chek if our node is the entry point fo the circuit
			if states.Round(update.State) == states.PRECOMPUTING &&
				topology.IsFirstNode(gw.ServerHost.GetId()) {
				gw.UnmixedBuffer.SetAsRoundLeader(id.Round(update.ID), update.BatchSize)
			}
		}
	}

	// Send a new batch to the server when it asks for one
	if newInfo.BatchRequest != nil {
		gw.SendBatch(newInfo.BatchRequest)
	}
	// Process a batch that has been completed by this server
	if newInfo.Batch != nil {
		gw.ProcessCompletedBatch(newInfo.Batch.Slots, id.Round(newInfo.Batch.RoundID))
	}

	return nil
}

// SprintRoundInfo prints the interesting parts of the round info object.
func SprintRoundInfo(ri *pb.RoundInfo) string {
	states := []string{"NOT_STARTED", "Waiting", "Precomp", "Standby",
		"Realtime", "Completed", "Error", "Crash"}
	topology := "v"
	for i := 0; i < len(ri.Topology); i++ {
		topology += "->" + base64.StdEncoding.EncodeToString(
			ri.Topology[i])
	}
	riStr := fmt.Sprintf("ID: %d, UpdateID: %d, State: %s, BatchSize: %d,"+
		"Topology: %s, RQTimeout: %d, Errors: %v",
		ri.ID, ri.UpdateID, states[ri.State], ri.BatchSize, topology,
		ri.ResourceQueueTimeoutMillis, ri.Errors)
	return riStr
}

// InitNetwork initializes the network on this gateway instance
// After the network object is created, you need to use it to connect
// to the corresponding server in the network using ConnectToNode.
// Additionally, to clean up the network object (especially in tests), call
// Shutdown() on the network object.
func (gw *Instance) InitNetwork() error {
	address := net.JoinHostPort(gw.Params.Address, strconv.Itoa(gw.Params.Port))
	var err error
	var gwCert, gwKey, nodeCert, permissioningCert []byte

	// Read our cert from file
	gwCert, err = utils.ReadFile(gw.Params.CertPath)
	if err != nil {
		return errors.Errorf("Failed to read certificate at %s: %+v",
			gw.Params.CertPath, err)
	}

	// Read our private key from file
	gwKey, err = utils.ReadFile(gw.Params.KeyPath)
	if err != nil {
		return errors.Errorf("Failed to read gwKey at %s: %+v",
			gw.Params.KeyPath, err)
	}

	// Read our node's cert from file
	nodeCert, err = utils.ReadFile(gw.Params.ServerCertPath)
	if err != nil {
		return errors.Errorf("Failed to read server gwCert at %s: %+v",
			gw.Params.ServerCertPath, err)
	}

	// Read the permissioning server's cert from
	permissioningCert, err = utils.ReadFile(gw.Params.PermissioningCertPath)
	if err != nil {
		return errors.WithMessagef(err,
			"Failed to read permissioning cert at %v",
			gw.Params.PermissioningCertPath)
	}

	// Load knownRounds data from file
	if err := gw.LoadKnownRounds(); err != nil {
		return err
	}
	// Ensure that knowRounds can be saved and crash if it cannot
	if err := gw.SaveKnownRounds(); err != nil {
		jww.FATAL.Panicf("Failed to save KnownRounds: %v", err)
	}

	// Load lastUpdate ID from file
	if err := gw.LoadLastUpdateID(); err != nil {
		return err
	}
	// Ensure that lastUpdate ID can be saved and crash if it cannot
	if err := gw.SaveLastUpdateID(); err != nil {
		jww.FATAL.Panicf("Failed to save lastUpdate: %v", err)
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

	// Get permissioning address from server
	permissioningAddr, err := gw.Comms.SendGetPermissioningAddress(gw.ServerHost)
	if err != nil {
		return errors.Errorf("Failed to get permissioning address from "+
			"server: %+v", err)
	}

	// Add permissioning host
	permissioningParams := connect.GetDefaultHostParams()
	permissioningParams.MaxRetries = 0
	permissioningParams.AuthEnabled = false
	_, err = gw.Comms.AddHost(&id.Permissioning, permissioningAddr,
		permissioningCert, permissioningParams)
	if err != nil {
		return errors.Errorf("Failed to add permissioning host: %+v", err)
	}

	// Get gateway's host from permissioning
	gw.Params.Address, err = CheckPermConn(gw.Params.Address, gw.Params.Port, gw.Comms)
	if err != nil {
		return errors.Errorf("Couldn't complete CheckPermConn: %v", err)
	}

	// Combine the discovered gateway host with the provided port
	gw.address = net.JoinHostPort(gw.Params.Address, strconv.Itoa(gw.Params.Port))
	address = gw.address

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
		serverResponse, err = PollServer(gw.Comms, gw.ServerHost, nil, nil, 0, gw.address)
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

		// Install the NDF once we get it
		if serverResponse.FullNDF != nil && serverResponse.Id != nil {
			netDef, _, err := ndf.DecodeNDF(string(serverResponse.FullNDF.Ndf))
			if err != nil {
				jww.WARN.Printf("failed to unmarshal the ndf: %+v", err)
				return err
			}
			err = gw.setupIDF(serverResponse.Id, netDef)
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

		jww.DEBUG.Printf("Creating instance!")
		gw.NetInf, err = CreateNetworkInstance(gw.Comms,
			serverResponse.FullNDF,
			serverResponse.PartialNDF, gw.storage)
		if err != nil {
			jww.ERROR.Printf("Unable to create network"+
				" instance: %v", err)
			continue
		}

		// Add permissioning as a host
		params := connect.GetDefaultHostParams()
		params.MaxRetries = 0
		params.AuthEnabled = false
		_, err = gw.Comms.AddHost(&id.Permissioning, permissioningAddr, permissioningCert,
			params)
		if err != nil {
			return errors.Errorf("Couldn't add permissioning host to comms: %v", err)
		}

		gw.addGateway = make(chan network.NodeGateway, gwChanLen)
		gw.removeGateway = make(chan *id.ID, gwChanLen)
		gw.NetInf.SetAddGatewayChan(gw.addGateway)
		gw.NetInf.SetRemoveGatewayChan(gw.removeGateway)

		if gw.Params.EnableGossip {
			gw.InitRateLimitGossip()
			gw.InitBloomGossip()
		}

		// Update the network instance
		// This must be below the enabling of the gossip above because it uses
		// components they initialize
		jww.DEBUG.Printf("Updating instance")
		err = gw.UpdateInstance(serverResponse)
		if err != nil {
			jww.ERROR.Printf("Update instance error: %v", err)
			continue
		}

		gw.Params.Address, err = CheckPermConn(gw.Params.Address, gw.Params.Port, gw.Comms)
		if err != nil {
			return errors.Errorf("Couldn't complete CheckPermConn: %v", err)
		}
		gw.address = fmt.Sprintf("%s:%d", gw.Params.Address, gw.Params.Port)

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
func (gw *Instance) setupIDF(nodeId []byte, ourNdf *ndf.NetworkDefinition) (err error) {

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
func (gw *Instance) RequestMessages(req *pb.GetMessages) (*pb.GetMessagesResponse, error) {
	// Error check for an invalidly crafted message
	if req == nil || req.ClientID == nil || req.RoundID == 0 {
		return &pb.GetMessagesResponse{}, errors.New("Could not parse message! " +
			"Please try again with a properly crafted message!")
	}

	// Parse the requested clientID within the message for the database request
	userId, err := id.Unmarshal(req.ClientID)
	if err != nil {
		return &pb.GetMessagesResponse{}, errors.New("Could not parse requested user ID!")
	}

	// Parse the roundID within the message
	roundID := id.Round(req.RoundID)

	// Search the database for the requested messages
	msgs, isValidGateway, err := gw.storage.GetMixedMessages(userId, roundID)
	if err != nil {
		jww.WARN.Printf("Could not find any MixedMessages with "+
			"recipient ID %v and round ID %v: %+v", userId, roundID, err)
		return &pb.GetMessagesResponse{
				HasRound: true,
			}, errors.Errorf("Could not find any MixedMessages with "+
				"recipient ID %v and round ID %v: %+v", userId, roundID, err)
	} else if !isValidGateway {
		jww.WARN.Printf("A client (%s) has requested messages for a "+
			"round (%v) which is not recorded with messages", userId, roundID)
		return &pb.GetMessagesResponse{
			HasRound: false,
		}, nil
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
		jww.DEBUG.Printf("Message Retrieved: %s, %s, %s",
			userId.String(), payloadA, payloadB)

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
// FIXME: Remove this function
func (gw *Instance) CheckMessages(userID *id.ID, msgID string, ipAddress string) ([]string, error) {
	jww.DEBUG.Printf("Getting message IDs for %q after %s from buffer...",
		userID, msgID)

	msgs, _, err := gw.storage.GetMixedMessages(userID, gw.NetInf.GetLastRoundID())
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
	// Construct Client ID for database lookup
	clientID, err := id.Unmarshal(msg.Message.SenderID)
	if err != nil {
		return &pb.GatewaySlotResponse{
			Accepted: false,
		}, errors.Errorf("Could not parse message: Unrecognized ID")
	}

	// Retrieve the client from the database
	cl, err := gw.storage.GetClient(clientID)
	if err != nil {
		return &pb.GatewaySlotResponse{
			Accepted: false,
		}, errors.New("Did not recognize ID. Have you registered successfully?")
	}

	// Generate the MAC and check against the message's MAC
	clientMac := generateClientMac(cl, msg)
	if !bytes.Equal(clientMac, msg.MAC) {
		return &pb.GatewaySlotResponse{
			Accepted: false,
		}, errors.New("Could not authenticate client. Please try again later")
	}
	thisRound := id.Round(msg.RoundID)

	// Rate limit messages
	senderId, err := id.Unmarshal(msg.GetMessage().GetSenderID())
	if err != nil {
		return nil, errors.Errorf("Unable to unmarshal sender ID: %+v", err)
	}

	if gw.Params.EnableGossip {
		err = gw.FilterMessage(senderId)
		if err != nil {
			jww.INFO.Printf("Rate limiting check failed on send message from "+
				"%v", msg.Message.GetSenderID())
			return &pb.GatewaySlotResponse{
				Accepted: false,
			}, err
		}
	}

	if err = gw.UnmixedBuffer.AddUnmixedMessage(msg.Message, thisRound); err != nil {
		return &pb.GatewaySlotResponse{Accepted: false},
			errors.WithMessage(err, "could not add to round. "+
				"Please try a different round.")
	}

	jww.DEBUG.Printf("Putting message from user %v in outgoing queue "+
		"for round %d...", msg.Message.GetSenderID(), thisRound)

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

	// Hash the clientGatewayKey and then the slot's salt
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

	err = gw.storage.UpsertClient(newClient)
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

	msg := format.NewMessage(grp.GetP().ByteLen())
	payloadBytes := make([]byte, grp.GetP().ByteLen())
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
	msg.SetRecipientID(&dummyUser)

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

// SendBatch polls sends whatever messages are in the batch associated with the
// requested round to the server
func (gw *Instance) SendBatch(roundInfo *pb.RoundInfo) {

	batchSize := uint64(roundInfo.BatchSize)
	if batchSize == 0 {
		jww.WARN.Printf("Server sent empty roundBufferSize!")
		return
	}

	batch := gw.UnmixedBuffer.PopRound(id.Round(roundInfo.ID))

	batch.Round = roundInfo

	jww.INFO.Printf("Sending batch for round %d with %d messages...", roundInfo.ID, len(batch.Slots))

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

	if gw.Params.EnableGossip {
		// Gossip senders included in the batch to other gateways
		err = gw.GossipBatch(batch)
		if err != nil {
			jww.WARN.Printf("Unable to gossip batch information: %+v", err)
		}
	}
}

// ProcessCompletedBatch handles messages coming out of the mixnet
func (gw *Instance) ProcessCompletedBatch(msgs []*pb.Slot, roundID id.Round) {
	if len(msgs) == 0 {
		return
	}

	numReal := 0
	// At this point, the returned batch and its fields should be non-nil
	msgsToInsert := make([]*storage.MixedMessage, len(msgs))
	recipients := make(map[id.ID]interface{})
	for _, msg := range msgs {
		serialMsg := format.NewMessage(gw.NetInf.GetCmixGroup().GetP().ByteLen())
		serialMsg.SetPayloadB(msg.PayloadB)
		userId := serialMsg.GetRecipientID()

		if !userId.Cmp(&dummyUser) {
			recipients[*userId] = nil

			jww.DEBUG.Printf("Message Received for: %s, %s, %s",
				userId.String(), msg.GetPayloadA(), msg.GetPayloadB())

			// Create new message and add it to the list for insertion
			msgsToInsert[numReal] = storage.NewMixedMessage(roundID, userId, msg.PayloadA, msg.PayloadB)
			numReal++
		}
	}

	// Perform the message insertion into Storage
	err := gw.storage.InsertMixedMessages(msgsToInsert[:numReal])
	if err != nil {
		jww.ERROR.Printf("Inserting new mixed messages failed in "+
			"ProcessCompletedBatch: %+v", err)
	}

	// Gossip recipients included in the completed batch to other gateways
	// in a new thread
	if gw.Params.EnableGossip {
		// Update filters in our storage system
		err = gw.UpsertFilters(recipients, gw.NetInf.GetLastRoundID())
		if err != nil {
			jww.ERROR.Printf("Unable to update local bloom filters: %+v", err)
		}

		go func() {
			err = gw.GossipBloom(recipients, gw.NetInf.GetLastRoundID())
			if err != nil {
				jww.ERROR.Printf("Unable to gossip bloom information: %+v", err)
			}
		}()
	}

	jww.INFO.Printf("Round received, %d real messages "+
		"processed, %d dummies ignored", numReal, len(msgs)-numReal)

	go PrintProfilingStatistics()
}

// Start sets up the threads and network server to run the gateway
func (gw *Instance) Start() {
	// Now that we're set up, run a thread that constantly
	// polls for updates
	go func() {
		// fixme: this last update needs to be persistent across resets
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			msg, err := PollServer(gw.Comms,
				gw.ServerHost,
				gw.NetInf.GetFullNdf(),
				gw.NetInf.GetPartialNdf(),
				gw.lastUpdate,
				gw.address)
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

// SaveKnownRounds saves the KnownRounds to a file.
func (gw *Instance) SaveKnownRounds() error {
	path := gw.Params.knownRoundsPath
	data, err := gw.knownRound.Marshal()
	if err != nil {
		return errors.Errorf("Failed to marshal KnownRounds: %v", err)
	}

	err = utils.WriteFile(path, data, utils.FilePerms, utils.DirPerms)
	if err != nil {
		return errors.Errorf("Failed to save KnownRounds to file: %v", err)
	}

	return nil
}

// LoadKnownRounds loads the KnownRounds from file into the Instance, if the
// file exists. Returns nil on successful loading or if the file does not exist.
// Returns an error if the file cannot be accessed or the data cannot be
// unmarshaled.
func (gw *Instance) LoadKnownRounds() error {
	data, err := utils.ReadFile(gw.Params.knownRoundsPath)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return errors.Errorf("Failed to read KnownRounds from file: %v", err)
	}

	err = gw.knownRound.Unmarshal(data)
	if err != nil {
		return errors.Errorf("Failed to unmarshal KnownRounds: %v", err)
	}

	return nil
}

// SaveLastUpdateID saves the Instance.lastUpdate to a file.
func (gw *Instance) SaveLastUpdateID() error {
	path := gw.Params.lastUpdateIdPath
	data := []byte(strconv.FormatUint(gw.lastUpdate, 10))

	err := utils.WriteFile(path, data, utils.FilePerms, utils.DirPerms)
	if err != nil {
		return errors.Errorf("Failed to save lastUpdate to file: %v", err)
	}

	return nil
}

// LoadLastUpdateID loads the Instance.lastUpdate from file into the Instance,
// if the file exists. Returns nil on successful loading or if the file does not
// exist. Returns an error if the file cannot be accessed or the data cannot be
// parsed.
func (gw *Instance) LoadLastUpdateID() error {
	data, err := utils.ReadFile(gw.Params.lastUpdateIdPath)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return errors.Errorf("Failed to read lastUpdate from file: %v", err)
	}

	dataStr := strings.TrimSpace(string(data))

	lastUpdate, err := strconv.ParseUint(dataStr, 10, 64)
	if err != nil {
		return errors.Errorf("Failed to parse lastUpdate from file: %v", err)
	}

	gw.lastUpdate = lastUpdate

	return nil
}
