////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Contains instance-related functionality, unrelated to messaging

package cmd

import (
	"encoding/base64"
	"fmt"
	"github.com/golang-collections/collections/set"
	"gitlab.com/elixxir/gateway/autocert"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/comms/gateway"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	ds "gitlab.com/elixxir/comms/network/dataStructures"
	"gitlab.com/elixxir/gateway/notifications"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/elixxir/primitives/states"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/primitives/hw"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/ndf"
	"gitlab.com/xx_network/primitives/rateLimiting"
	"gitlab.com/xx_network/primitives/utils"
	"gorm.io/gorm"
)

// Errors to suppress
const (
	ErrInvalidHost   = "Invalid host ID:"
	ErrAuth          = "Failed to authenticate id:"
	gwChanLen        = 1000
	period           = int64(1800000000000) // 30 minutes in nanoseconds
	maxUnknownErrors = 20
)

// The max number of rounds to be stored in the KnownRounds buffer.
const knownRoundsSize = 1512000

// EarliestRound denotes the earliest tracked round for this gateway.
type EarliestRound struct {
	clientRoundId uint64
	gwRoundID     uint64
	gwTimestamp   int64
}

// IsZero  returns whether any of the values are zero,
// representing an uninitialized object
func (er EarliestRound) IsZero() bool {
	jww.DEBUG.Printf("clientRound %d, gwRound %d, gwTs %v", er.clientRoundId, er.gwRoundID, er.gwTimestamp)
	return er.clientRoundId == 0
}

type Instance struct {
	// Storage buffer for messages to be submitted to the network
	UnmixedBuffer storage.UnmixedMessageBuffer

	// Contains all Gateway relevant fields
	Params Params

	// Contains Server Host Information
	ServerHost *connect.Host

	// Gateway object created at start
	Comms *gateway.Comms

	rateLimitQuit chan struct{}

	// struct for tracking notifications
	un notifications.UserNotifications

	// Tracker of the gateway's known rounds
	krw *knownRoundsWrapper

	storage *storage.Storage
	// TODO: Integrate and remove duplication with the stuff above.
	// NetInf is the network interface for working with the NDF poll
	// functionality in comms.
	NetInf *network.Instance
	// Filtered network updates for fast updates for client
	filteredUpdates *FilteredUpdates
	addGateway      chan network.NodeGateway
	removeGateway   chan *id.ID

	lastUpdate uint64
	period     int64 // Defines length of validity for ClientBloomFilter

	bloomFilterGossip sync.Mutex

	// Rate limiting
	RateLimitingMux sync.RWMutex

	idRateLimiting  *rateLimiting.BucketMap
	idRateLimitQuit chan struct{}

	// Rate limiting
	ipAddrRateLimiting  *rateLimiting.BucketMap
	ipAddrRateLimitQuit chan struct{}

	whitelistedIpAddressSet *set.Set
	whitelistedIdsSet       *set.Set

	LeakedCapacity uint
	LeakedTokens   uint
	LeakDuration   time.Duration

	earliestRoundTracker       atomic.Value
	earliestRoundTrackerUnsafe EarliestRound

	earliestRoundTrackerMux sync.Mutex
	earliestRoundUpdateChan chan EarliestRound
	earliestRoundQuitChan   chan struct{}

	autoCert    autocert.Client
	gwCertMux   sync.RWMutex
	gatewayCert *pb.GatewayCertificate
}

// NewGatewayInstance initializes a gateway Handler interface
func NewGatewayInstance(params Params) *Instance {
	newDatabase, err := storage.NewStorage(params.DbUsername,
		params.DbPassword,
		params.DbName,
		params.DbAddress,
		params.DbPort,
		params.DevMode,
	)
	if err != nil {
		eMsg := fmt.Sprintf("Could not initialize database psql://%s@%s:%s/%s: %+v",
			params.DbUsername, params.DbAddress, params.DbPort, params.DbName, err)
		if params.DevMode {
			jww.WARN.Printf(eMsg)
		} else {
			jww.FATAL.Panicf(eMsg)
		}
	}

	krw, err := newKnownRoundsWrapper(knownRoundsSize, newDatabase)
	if err != nil {
		jww.FATAL.Panicf("failed to create new KnownRounds wrapper: %+v", err)
	}

	earliestRoundUpdateChan := make(chan EarliestRound)
	i := &Instance{
		UnmixedBuffer:           storage.NewUnmixedMessagesMap(),
		Params:                  params,
		storage:                 newDatabase,
		krw:                     krw,
		idRateLimitQuit:         make(chan struct{}, 1),
		ipAddrRateLimitQuit:     make(chan struct{}, 1),
		whitelistedIpAddressSet: set.New(),
		whitelistedIdsSet:       set.New(),
		LeakedCapacity:          1,
		LeakDuration:            2000 * time.Millisecond,
		LeakedTokens:            1,
		earliestRoundUpdateChan: earliestRoundUpdateChan,
		earliestRoundQuitChan:   make(chan struct{}, 1),
	}

	i.autoCert = autocert.NewDNS()

	msgRateLimitParams := &rateLimiting.MapParams{
		Capacity:     uint32(i.LeakedCapacity),
		LeakedTokens: uint32(i.LeakedTokens),
		LeakDuration: i.LeakDuration,
		PollDuration: params.messageRateLimitParams.PollDuration,
		BucketMaxAge: params.messageRateLimitParams.BucketMaxAge,
	}
	i.idRateLimiting = rateLimiting.CreateBucketMapFromParams(msgRateLimitParams, nil, i.idRateLimitQuit)
	i.ipAddrRateLimiting = rateLimiting.CreateBucketMapFromParams(msgRateLimitParams, nil, i.ipAddrRateLimitQuit)

	hw.LogHardware()

	return i
}

func NewImplementation(instance *Instance) *gateway.Implementation {
	impl := gateway.NewImplementation()

	impl.Functions.RequestClientKey = func(message *pb.SignedClientKeyRequest) (*pb.SignedKeyResponse, error) {
		return instance.RequestClientKey(message)
	}

	impl.Functions.PutMessage = func(message *pb.GatewaySlot, ipAddr string) (*pb.GatewaySlotResponse, error) {
		return instance.PutMessage(message, ipAddr)
	}
	impl.Functions.PutManyMessages = func(messages *pb.GatewaySlots, ipAddr string) (*pb.GatewaySlotResponse, error) {
		return instance.PutManyMessages(messages, ipAddr)
	}
	impl.Functions.RequestClientKey = func(message *pb.SignedClientKeyRequest) (nonce *pb.SignedKeyResponse, e error) {
		return instance.RequestClientKey(message)
	}
	// Client -> Gateway historical round request
	impl.Functions.RequestHistoricalRounds = func(msg *pb.HistoricalRounds) (response *pb.HistoricalRoundsResponse, err error) {
		return instance.RequestHistoricalRounds(msg)
	}
	// Client -> Gateway message request
	impl.Functions.RequestMessages = func(msg *pb.GetMessages) (*pb.GetMessagesResponse, error) {
		return instance.RequestMessages(msg)
	}
	impl.Functions.Poll = func(msg *pb.GatewayPoll) (response *pb.GatewayPollResponse, err error) {
		return instance.Poll(msg)
	}

	impl.Functions.PutMessageProxy = func(message *pb.GatewaySlot, auth *connect.Auth) (*pb.GatewaySlotResponse, error) {
		return instance.PutMessageProxy(message, auth)
	}

	impl.Functions.PutManyMessagesProxy = func(msgs *pb.GatewaySlots, auth *connect.Auth) (*pb.GatewaySlotResponse, error) {
		return instance.PutManyMessagesProxy(msgs, auth)
	}

	impl.Functions.RequestTlsCert = func(message *pb.RequestGatewayCert) (*pb.GatewayCertificate, error) {
		return instance.RequestTlsCert(message)
	}

	return impl
}

func (gw *Instance) RequestTlsCert(_ *pb.RequestGatewayCert) (*pb.GatewayCertificate, error) {
	gw.gwCertMux.RLock()
	defer gw.gwCertMux.RUnlock()
	if gw.gatewayCert == nil {
		return nil, errors.New("Gateway HTTPS initialization has not finished yet")
	}
	return gw.gatewayCert, nil
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
	return network.NewInstance(pc, newNdf.Get(), newPartialNdf.Get(), ers, network.None, false)
}

// Start sets up the threads and network server to run the gateway
func (gw *Instance) Start() {
	// Now that we're set up, run a thread that constantly
	// polls for updates
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		for range ticker.C {
			msg, err := PollServer(gw.Comms,
				gw.ServerHost,
				gw.NetInf.GetFullNdf(),
				gw.NetInf.GetPartialNdf(),
				gw.lastUpdate,
				gw.Params.PublicAddress)
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

func (gw *Instance) GetRateLimitParams() (uint32, uint32, time.Duration) {
	gw.RateLimitingMux.RLock()
	defer gw.RateLimitingMux.RUnlock()
	return uint32(gw.LeakedCapacity), uint32(gw.LeakedTokens), gw.LeakDuration
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

		// Unmarshal NDF
		newNdf, err := ndf.Unmarshal(newInfo.FullNDF.Ndf)
		if err != nil {
			return err
		}

		gw.RateLimitingMux.Lock()
		if gw.LeakedCapacity != newNdf.RateLimits.Capacity ||
			gw.LeakedTokens != newNdf.RateLimits.LeakedTokens ||
			gw.LeakDuration != time.Duration(newNdf.RateLimits.LeakDuration) {

			gw.LeakedCapacity = newNdf.RateLimits.Capacity
			gw.LeakedTokens = newNdf.RateLimits.LeakedTokens
			gw.LeakDuration = time.Duration(newNdf.RateLimits.LeakDuration)

			jww.INFO.Printf("rate limit gossip updates: "+
				"(LeakedCapacity: %d, LeakedTokens: %d, "+
				"LeakDuration: %s)", gw.LeakedCapacity,
				gw.LeakedTokens, gw.LeakDuration)

		}

		// Construct set of of whitelisted IDs
		whitelistedIdsSet := set.New()
		for _, ids := range newNdf.WhitelistedIds {
			whitelistedIdsSet.Insert(ids)
		}

		// Find difference between new set from NDF and stored set from impl
		removedIds := gw.whitelistedIdsSet.Difference(whitelistedIdsSet)

		// Remove all whitelisted IDs no longer in NDF
		removedIds.Do(func(i interface{}) {
			removedId := i.(string)
			err = gw.idRateLimiting.DeleteBucket(removedId)
			if err != nil {
				jww.ERROR.Printf("Could not remove ID from whitelist: %v", err)
			}

		})

		// Store new set in impl
		gw.whitelistedIdsSet = whitelistedIdsSet

		// Construct set of of whitelisted IP addresses from NDF
		whitelistedIpAddressesSet := set.New()
		for _, ipAddr := range newNdf.WhitelistedIpAddresses {
			whitelistedIpAddressesSet.Insert(ipAddr)
		}

		// Find difference between new set from NDF and stored set from impl
		removedIpAddresses := gw.whitelistedIdsSet.Difference(whitelistedIpAddressesSet)

		// Remove all whitelisted IP addresses no longer in NDF
		removedIpAddresses.Do(func(i interface{}) {
			removedIpAddr := i.(string)
			err = gw.ipAddrRateLimiting.DeleteBucket(removedIpAddr)
			if err != nil {
				jww.ERROR.Printf("Could not remove IP address from whitelist: %v", err)
			}
		})

		// Store new set in impl
		gw.whitelistedIpAddressSet = whitelistedIpAddressesSet

		// Update the whitelisted rate limiting IDs
		gw.idRateLimiting.AddToWhitelist(newNdf.WhitelistedIds)
		gw.ipAddrRateLimiting.AddToWhitelist(newNdf.WhitelistedIpAddresses)

		gw.RateLimitingMux.Unlock()
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
		for i := len(newInfo.Updates) - 1; i >= 0; i-- {
			update := newInfo.Updates[i]
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

			// Add the updates to the consensus object
			_, err = gw.NetInf.RoundUpdate(update)
			if err != nil {
				// do not return on round update failure, that will cause the
				// gateway to cease to process further updates, just warn
				jww.WARN.Printf("failed to insert round update for %d: %s", update.ID, err)
			}

			// Add updates to filter for fast client polling
			err = gw.filteredUpdates.RoundUpdate(update)
			if err != nil {
				// do not return on round update failure, that will cause the
				// gateway to cease to process further updates, just warn
				jww.WARN.Printf("failed to insert filtered round update for %d: %s", update.ID, err)
			}

			// Convert the ID list to a circuit
			topology := ds.NewCircuit(idList)

			// Chek if our node is the entry point fo the circuit
			if states.Round(update.State) == states.PRECOMPUTING &&
				topology.IsFirstNode(gw.ServerHost.GetId()) {
				rid := id.Round(update.ID)
				gw.UnmixedBuffer.SetAsRoundLeader(rid, update.BatchSize)
			} else if states.Round(update.State) == states.FAILED {
				err = gw.krw.forceCheck(id.Round(update.ID))
				if err != nil {
					return errors.Errorf("failed to forceChech round %d: %+v",
						update.ID, err)
				}
				jww.TRACE.Printf("Instance updated, knownrounds last checked: %d", gw.krw.getLastChecked())
			}
		}
	}

	// If batch is non-nil, then server is reporting that there is a batch to stream
	if newInfo.Batch != nil {
		// Request the batch
		slots, err := gw.Comms.DownloadMixedBatch(newInfo.Batch, gw.ServerHost)
		if err != nil {
			return errors.Errorf("failed to retrieve mixed batch for round %d: %v",
				newInfo.Batch.RoundId, err)
		}

		// Process the batch
		err = gw.ProcessCompletedBatch(slots, id.Round(newInfo.Batch.RoundId))
		if err != nil {
			return err
		}
	}

	// Send a new batch to the server when it asks for one
	if newInfo.BatchRequest != nil {
		gw.UploadUnmixedBatch(newInfo.BatchRequest)
	}

	if newInfo.EarliestRoundErr == "" {
		gw.UpdateEarliestRound(newInfo.GetEarliestClientRound(),
			newInfo.GetEarliestGatewayRound(), newInfo.GetEarliestRoundTimestamp())
	}

	return nil
}

// sprintRoundInfo prints the interesting parts of the round info object.
func sprintRoundInfo(ri *pb.RoundInfo) string {
	roundStates := []string{"NOT_STARTED", "Waiting", "Precomp", "Standby",
		"Realtime", "Completed", "Error", "Crash"}
	topology := "v"
	for i := 0; i < len(ri.Topology); i++ {
		topology += "->" + base64.StdEncoding.EncodeToString(
			ri.Topology[i])
	}
	riStr := fmt.Sprintf("ID: %d, UpdateID: %d, State: %s, BatchSize: %d,"+
		"Topology: %s, RQTimeout: %d, Errors: %v",
		ri.ID, ri.UpdateID, roundStates[ri.State], ri.BatchSize, topology,
		ri.ResourceQueueTimeoutMillis, ri.Errors)
	return riStr
}

// InitNetwork initializes the network on this gateway instance
// After the network object is created, you need to use it to connect
// to the corresponding server in the network using ConnectToNode.
// Additionally, to clean up the network object (especially in tests), call
// Shutdown() on the network object.
func (gw *Instance) InitNetwork() error {
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

	// Load knownRounds data from storage if it exists
	if err := gw.LoadKnownRounds(); err != nil {
		jww.WARN.Printf("Unable to load KnownRounds: %+v", err)
	}

	// Load lastUpdate ID from storage if it exists
	if err := gw.LoadLastUpdateID(); err != nil {
		jww.WARN.Printf("Unable to load LastUpdateID: %+v", err)
	}

	jww.DEBUG.Printf("On start up last update ID is: %d", gw.lastUpdate)

	// Set up temporary gateway listener
	gatewayHandler := NewImplementation(gw)
	// Start storage cleanup thread
	go func() {
		gw.beginStorageCleanup()
	}()

	gw.Comms = gateway.StartGateway(&id.TempGateway, gw.Params.ListeningAddress,
		gatewayHandler, gwCert, gwKey, gossip.DefaultManagerFlags())

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

	// Begin polling server for NDF
	jww.INFO.Printf("Beginning polling NDF...")
	var nodeId []byte
	var serverResponse *pb.ServerPollResponse

	// fixme: determine if this a proper conditional for when server is not ready
	numUnknownErrors := 0
	for serverResponse == nil {
		// TODO: Probably not great to always sleep immediately
		time.Sleep(3 * time.Second)

		// Poll Server for the NDFs, then use it to create the
		// network instance and begin polling for server updates
		serverResponse, err = PollServer(gw.Comms, gw.ServerHost, nil, nil, 0,
			gw.Params.PublicAddress)
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
				numUnknownErrors++
				if numUnknownErrors >= maxUnknownErrors {
					return errors.Errorf(
						"Error polling NDF %d times, bailing: %+v", numUnknownErrors, err)
				} else {
					jww.WARN.Printf("Error polling NDF %d/%d times: %+v",
						numUnknownErrors, maxUnknownErrors, err)
					continue
				}
			}
		}

		// Make sure the NDF is ready
		if serverResponse.FullNDF == nil || serverResponse.Id == nil {
			serverResponse = nil
			continue
		}

		netDef, err := ndf.Unmarshal(serverResponse.FullNDF.Ndf)
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
		gw.Comms = gateway.StartGateway(gatewayId, gw.Params.ListeningAddress,
			gatewayHandler, gwCert, gwKey, gossip.DefaultManagerFlags())
		gw.Comms.StartConnectionReport()

		jww.INFO.Printf("Creating instance!")
		gw.NetInf, err = CreateNetworkInstance(gw.Comms,
			serverResponse.FullNDF,
			serverResponse.PartialNDF, gw.storage)
		if err != nil {
			jww.ERROR.Printf("Unable to create network"+
				" instance: %v", err)
			continue
		}
		jww.INFO.Printf("Instance created")

		// Initialize the update tracker for fast client polling
		gw.filteredUpdates, err = NewFilteredUpdates(gw.NetInf)
		if err != nil {
			return errors.Errorf("Failed to create filtered update: %+v", err)
		}

		// Add permissioning as a host
		params := connect.GetDefaultHostParams()
		params.MaxRetries = 0
		params.AuthEnabled = false
		_, err = gw.Comms.AddHost(&id.Permissioning, permissioningAddr,
			permissioningCert, params)
		if err != nil {
			return errors.Errorf("Couldn't add permissioning host to comms: %v", err)
		}

		gw.addGateway = make(chan network.NodeGateway, gwChanLen)
		gw.removeGateway = make(chan *id.ID, gwChanLen)
		gw.NetInf.SetAddGatewayChan(gw.addGateway)
		gw.NetInf.SetRemoveGatewayChan(gw.removeGateway)

		notificationParams := connect.GetDefaultHostParams()
		notificationParams.MaxRetries = 3
		notificationParams.EnableCoolOff = true

		// Add notification bot as a host
		if gw.NetInf.GetFullNdf().Get().Notification.Address != "" {
			jww.WARN.Printf("Notifications Bot is not specified in the NDF, not adding as host")

			_, err = gw.Comms.AddHost(
				&id.NotificationBot,
				gw.NetInf.GetFullNdf().Get().Notification.Address,
				[]byte(gw.NetInf.GetFullNdf().Get().Notification.TlsCertificate),
				notificationParams,
			)
			if err != nil {
				return errors.Errorf("failed to add notification bot host to comms: %v", err)
			}
		}
		// Enable authentication on gateway to gateway communications
		gw.NetInf.SetGatewayAuthentication()

		hp := connect.GetDefaultHostParams()
		hp.AuthEnabled = false
		_, err = gw.Comms.AddHost(&id.Authorizer, gw.Params.AuthorizerAddress, permissioningCert, hp)
		if err != nil {
			return errors.WithMessage(err, "Failed to add authorizer host")
		}
		// Start https server in gofunc so it doesn't block running rounds
		go func() {
			err = gw.StartHttpsServer()
			if err != nil {
				jww.ERROR.Printf("Failed to start HTTPS listener: %+v", err)
			}
		}()

		// Turn on gossiping
		if !gw.Params.DisableGossip {
			gw.InitRateLimitGossip()
			gw.InitBloomGossip()
		}

		// Update the network instance
		// This must be below the enabling of the gossip above because it uses
		// components they initialize
		jww.INFO.Printf("Updating instance")
		err = gw.UpdateInstance(serverResponse)
		if err != nil {
			jww.ERROR.Printf("Update instance error: %v", err)
			continue
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

// Async function for cleaning up gateway storage
// and managing variables that need updated after cleanup
func (gw *Instance) beginStorageCleanup() {
	time.Sleep(1 * time.Second)

	// Begin ticker for storage cleanup
	for true {
		select {
		case earliestRound := <-gw.earliestRoundUpdateChan:
			// Run storage cleanup when timer expires
			if earliestRound.gwTimestamp > 0 {
				clearTimeStamp := time.Unix(0, earliestRound.gwTimestamp)

				err := gw.clearOldStorage(clearTimeStamp)
				if err != nil {
					jww.WARN.Printf("Issue clearing old storage: %v", err)
					continue
				}
			}
		case <-gw.earliestRoundQuitChan:
			return
		}
	}
}

// Clears out old messages, rounds and bloom filters
func (gw *Instance) clearOldStorage(threshold time.Time) error {
	// Clear out old rounds and messages
	err := gw.storage.ClearOldStorage(threshold)
	if err != nil {
		return errors.Errorf("Could not clear old rounds and/or messages: %v", err)
	}

	// Clear out filters by epoch
	timestamp := time.Unix(0, threshold.UnixNano()).UnixNano()
	epoch := GetEpoch(timestamp, gw.period)
	err = gw.storage.DeleteClientFiltersBeforeEpoch(epoch)
	if err != nil {
		return errors.Errorf("Could not clear bloom filters: %v", err)
	}
	jww.DEBUG.Printf("Deleted bloom filters before %d", epoch)
	return nil
}

// Set the gw.period attribute
// NOTE: Saves the constant to storage if it does not exist
//
//	or reads an existing value from storage and sets accordingly
//	It's not great but it's structured this way as a business requirement
func (gw *Instance) SetPeriod() error {
	// Get an existing Period value from storage
	periodStr, err := gw.storage.GetStateValue(storage.PeriodKey)
	if err != nil &&
		!strings.Contains(err.Error(), gorm.ErrRecordNotFound.Error()) &&
		!strings.Contains(err.Error(), "Unable to locate state for key") {
		// If the error is unrelated to record not in storage, return it
		return err
	}

	if len(periodStr) > 0 {
		// If period already stored, use that value
		gw.period, err = strconv.ParseInt(periodStr, 10, 64)
	} else {
		// If period not already stored, use periodConst
		gw.period = period
		err = gw.storage.UpsertState(&storage.State{
			Key:   storage.PeriodKey,
			Value: strconv.FormatInt(period, 10),
		})
	}
	return err
}

// SaveKnownRounds saves the KnownRounds to a file.
func (gw *Instance) SaveKnownRounds() error {
	return gw.krw.save()
}

// LoadKnownRounds loads the KnownRounds from storage into the Instance, if a
// stored value exists.
func (gw *Instance) LoadKnownRounds() error {
	return gw.krw.load(gw.storage)
}

// SaveLastUpdateID saves the Instance.lastUpdate value to storage
func (gw *Instance) SaveLastUpdateID() error {
	data := strconv.FormatUint(gw.lastUpdate, 10)

	return gw.storage.UpsertState(&storage.State{
		Key:   storage.LastUpdateKey,
		Value: data,
	})

}

// LoadLastUpdateID loads the Instance.lastUpdate from storage into the Instance,
// if the key exists.
func (gw *Instance) LoadLastUpdateID() error {
	// Get an existing lastUpdate value from storage
	data, err := gw.storage.GetStateValue(storage.LastUpdateKey)
	if err != nil {
		return err
	}

	// Parse the last update
	dataStr := strings.TrimSpace(data)
	lastUpdate, err := strconv.ParseUint(dataStr, 10, 64)
	if err != nil {
		return errors.Errorf("Failed to get LastUpdateID: %v", err)
	}

	gw.lastUpdate = lastUpdate
	return nil
}

func (gw *Instance) GetEarliestRound() (uint64, uint64, time.Time, error) {
	// Retrieve the earliest round from tracker
	earliestRound, ok := gw.earliestRoundTracker.Load().(EarliestRound)
	if !ok || // Or the// If not of the expected type
		earliestRound.IsZero() { // or if the object is uninitialized, return an error
		jww.DEBUG.Printf("GetEarliestRound is ok %v and isZero %v", ok, earliestRound.IsZero())
		return 0, 0, time.Time{},
			errors.New("Earliest round state does not exist, try again")
	}

	// Return values
	return earliestRound.clientRoundId,
		earliestRound.gwRoundID, time.Unix(0, earliestRound.gwTimestamp), nil
}

func (gw *Instance) UpdateEarliestRound(newClientRoundId,
	newGwRoundID uint64, newRoundTimestamp int64) {
	gw.earliestRoundTrackerMux.Lock()
	defer gw.earliestRoundTrackerMux.Unlock()

	// Create earliest round
	newEarliestRound := EarliestRound{
		clientRoundId: newClientRoundId,
		gwRoundID:     newGwRoundID,
		gwTimestamp:   newRoundTimestamp,
	}

	// Determine if values need to be updated
	isUpdate := newEarliestRound.gwTimestamp > gw.earliestRoundTrackerUnsafe.gwTimestamp ||
		newEarliestRound.clientRoundId > gw.earliestRoundTrackerUnsafe.clientRoundId ||
		newEarliestRound.gwRoundID > gw.earliestRoundTrackerUnsafe.gwRoundID

	// Update values if update is needed
	if isUpdate {
		gw.earliestRoundTracker.Store(newEarliestRound)
		gw.earliestRoundTrackerUnsafe = newEarliestRound

		// Send to storage maintenance thread
		gw.earliestRoundUpdateChan <- newEarliestRound
	}

}
