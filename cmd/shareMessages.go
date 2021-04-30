///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// Contains logic message-sharing functionality within a round's team

package cmd

import (
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/primitives/id"
)

// Helper function for sharing messages in the batch with the rest of the team
func (gw *Instance) sendShareMessages(msgs []*pb.Slot, round *pb.RoundInfo) error {
	// Process round topology into IDs
	idList, err := id.NewIDListFromBytes(round.Topology)
	if err != nil {
		return errors.Errorf("Could not read topology from round %d: %+v", round.ID, err)
	}

	// Build share message
	shareMsg := &pb.RoundMessages{
		RoundId:  round.ID,
		Messages: msgs,
	}

	// Send share message to other gateways in team, excluding self
	for _, teamId := range idList {
		teamId.SetType(id.Gateway)
		if teamId.Cmp(gw.Comms.Id) {
			continue
		}

		teamHost, exists := gw.Comms.GetHost(teamId)
		if !exists {
			return errors.Errorf("Unable to find host for message sharing: %s",
				teamId.String())
		}

		// Make the sends non-blocking
		go func(teamIdStr string) {
			err = gw.Comms.SendShareMessages(teamHost, shareMsg)
			if err != nil {
				jww.ERROR.Printf("Unable to share messages with host %s on round %d: %+v",
					teamIdStr, round.ID, err)
			}
		}(teamId.String())
	}
	return nil
}

// Reception handler for sendShareMessages. Performs auth checks for a valid gateway.
// If valid, processes and adds the messages to storage
func (gw *Instance) ShareMessages(msg *pb.RoundMessages, auth *connect.Auth) error {
	// At this point, the returned batch and its fields should be non-nil
	roundId := id.Round(msg.RoundId)

	jww.INFO.Printf("Received Share messages for Round %d", roundId)

	round, err := gw.NetInf.GetRound(roundId)
	if err != nil {
		return errors.Errorf("Unable to get round %d: %+v", roundId, err)
	}

	// Parse the round topology
	idList, err := id.NewIDListFromBytes(round.Topology)
	if err != nil {
		return errors.Errorf("Could not read topology from round %d messages: %s",
			roundId, err)
	}

	topology := connect.NewCircuit(idList)
	nodeID := auth.Sender.GetId().DeepCopy()
	nodeID.SetType(id.Node)
	myNodeID := gw.Comms.Id.DeepCopy()
	myNodeID.SetType(id.Node)

	// Auth checks required:
	// Make sure authentication is valid, this gateway is in the round,
	// the sender is the LastGateway in that round, and that the num slots
	// that sender sent less equal to the batchSize for that round
	if !(auth.IsAuthenticated && topology.GetNodeLocation(myNodeID) != -1 &&
		topology.IsLastNode(nodeID) && len(msg.Messages) <= int(round.BatchSize)) {
		return connect.AuthError(auth.Sender.GetId())
	}

	_, clientRound := gw.processMessages(msg.Messages, roundId, round)
	errMsg := gw.storage.InsertMixedMessages(clientRound)
	if errMsg != nil {
		jww.ERROR.Printf("Inserting new mixed messages failed in "+
			"ProcessCompletedBatch for round %d: %+v", roundId, errMsg)
	}
	return nil
}
