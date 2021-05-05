///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// Contains polling-related functionality

package cmd

import (
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/comms/gateway"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/primitives/version"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gitlab.com/xx_network/primitives/ndf"
)

// Handler for a client's poll to a gateway. Returns all the last updates and known rounds
func (gw *Instance) Poll(clientRequest *pb.GatewayPoll) (
	*pb.GatewayPollResponse, error) {
	// Nil check to check for valid clientRequest
	if clientRequest == nil {
		return &pb.GatewayPollResponse{}, errors.Errorf(
			"Poll() clientRequest is empty")
	}

	// Make sure Gateway network instance is not nil
	if gw.NetInf == nil {
		return &pb.GatewayPollResponse{}, errors.New(ndf.NO_NDF)
	}

	// Get version sent from client
	clientVersion, err := version.ParseVersion(string(clientRequest.ClientVersion))
	if err != nil {
		return &pb.GatewayPollResponse{}, errors.Errorf(
			"Unable to ParseVersion for clientRequest: %+v", err)
	}
	// Get version from NDF
	expectedClientVersion, err := version.ParseVersion(gw.NetInf.GetFullNdf().Get().ClientVersion)
	if err != nil {
		return &pb.GatewayPollResponse{}, errors.Errorf(
			"Unable to ParseVersion for gateway's NDF: %+v", err)
	}
	// Check that the two versions are compatible
	if version.IsCompatible(expectedClientVersion, clientVersion) == false {
		return &pb.GatewayPollResponse{}, errors.Errorf(
			"client version \"%s\" was not compatible with NDF defined minimum version", clientRequest.ClientVersion)
	}

	// Check if the clientID is populated and valid
	receptionId, err := ephemeral.Marshal(clientRequest.GetReceptionID())
	if err != nil {
		return &pb.GatewayPollResponse{}, errors.Errorf(
			"Poll() - Valid ReceptionID required: %+v", err)
	}

	kr := gw.knownRound.Marshal()

	// Determine Client epoch range
	startEpoch, err := GetEpochEdge(time.Unix(0, clientRequest.StartTimestamp).UnixNano(), gw.period)
	if err != nil {
		return &pb.GatewayPollResponse{}, errors.WithMessage(err, "Failed to "+
			"handle client poll due to invalid start timestamp")
	}
	endEpoch, err := GetEpochEdge(time.Unix(0, clientRequest.EndTimestamp).UnixNano(), gw.period)
	if err != nil {
		return &pb.GatewayPollResponse{}, errors.WithMessage(err, "Failed to "+
			"handle client poll due to invalid end timestamp")
	}

	// These errors are suppressed, as DB errors shouldn't go to client
	//  and if there is trouble getting filters returned, nil filters
	//  are returned to the client. Debug to avoid message spam.
	clientFilters, err := gw.storage.GetClientBloomFilters(
		receptionId, startEpoch, endEpoch)
	jww.DEBUG.Printf("Adding %d client filters for %d", len(clientFilters), receptionId.Int64())
	if err != nil {
		jww.DEBUG.Printf("Could not get filters in range %d - %d for %d when polling: %v", startEpoch, endEpoch, receptionId.Int64(), err)
	}

	// Build ClientBlooms metadata
	filtersMsg := &pb.ClientBlooms{
		Period:         gw.period,
		FirstTimestamp: GetEpochTimestamp(startEpoch, gw.period),
	}

	if len(clientFilters) > 0 {
		filtersMsg.Filters = make([]*pb.ClientBloom, endEpoch-startEpoch+1)
		// Build ClientBloomFilter list for client
		for _, f := range clientFilters {
			index := f.Epoch - startEpoch
			//todo- remove the if statement 1 week after 3/24/2021
			if index < uint32(len(filtersMsg.Filters)) {
				filtersMsg.Filters[index] = &pb.ClientBloom{
					Filter:     f.Filter,
					FirstRound: f.FirstRound,
					RoundRange: f.RoundRange,
				}
			}

		}
	}

	var netDef *pb.NDF
	var updates []*pb.RoundInfo
	isSame := gw.NetInf.GetPartialNdf().CompareHash(clientRequest.Partial.Hash)
	if !isSame {
		netDef = gw.NetInf.GetPartialNdf().GetPb()
	} else {
		// Get the range of updates from the network instance
		updates = gw.NetInf.GetRoundUpdates(int(clientRequest.LastUpdate))
	}

	return &pb.GatewayPollResponse{
		PartialNDF:    netDef,
		Updates:       updates,
		KnownRounds:   kr,
		Filters:       filtersMsg,
		EarliestRound: atomic.LoadUint64(gw.lowestRound),
	}, nil
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
