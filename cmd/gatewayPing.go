///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////
package cmd

import (
	"github.com/pkg/errors"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/comms/messages"
	"gitlab.com/xx_network/primitives/id"
	"time"
)

// GatewayPingResponse returns to the main thread
// the process of the asynchronous gateway ping.
type GatewayPingResponse struct {
	// The Id of the gateway that has been pinged
	gwId *id.ID

	// Status of the ping. Any error sets to false
	success bool
}

// ReportGatewayPings asynchronously pings all gateways in the team (besides itself)
// It then reports the pinging results to it's node once all gateways pings have been attempted
func (gw *Instance) ReportGatewayPings(pingRequest *pb.GatewayPingRequest) (*pb.GatewayPingReport, error) {
	round, err := gw.NetInf.GetRound(id.Round(pingRequest.RoundId))
	if err != nil {
		return nil, errors.Errorf("Unable to get round: %+v", err)
	}

	// Process round topology into IDs
	idList, err := id.NewIDListFromBytes(round.Topology)
	if err != nil {
		return nil, errors.Errorf("Could not read topology from round %d: %+v", round.ID, err)
	}

	pingResponseChan := make(chan *GatewayPingResponse, len(round.Topology)-1)

	// Send gatewayPing to other gateways in team, excluding self
	for _, teamId := range idList {
		err := gw.pingGateways(teamId, pingResponseChan)
		if err != nil {
			return nil, errors.Errorf("Error pinging gateways: %v", err)
		}

	}

	report := &pb.GatewayPingReport{
		RoundId: round.ID,
	}

	// Allow time for comms to go through
	time.Sleep(100 * time.Millisecond)

	// Exhaust response channel
	done := false
	for !done {
		select {
		case response := <-pingResponseChan:
			if !response.success {
				// Mutate report message to report failed gateway
				report.FailedGateways = append(report.FailedGateways, response.gwId.Bytes())
			}
		default:
			done = true
		}
	}

	return report, nil
}

// pingGateways is a helper function which pings an individual gateway.
// Returns via the responseChan either:
//  - failure if there is any error in the comm or
//  	errors in processing the response
//  - success otherwise
func (gw *Instance) pingGateways(teamId *id.ID, responseChan chan *GatewayPingResponse) error {

	// Set the Id to a gateway (Id is defaulted to node type)
	// Skip sending to ourselves
	teamId.SetType(id.Gateway)
	if teamId.Cmp(gw.Comms.Id) {
		return nil
	}

	// Get the gateway host
	teamHost, exists := gw.Comms.GetHost(teamId)
	if !exists {
		return errors.Errorf("Unable to find host for gateway pinging: %s",
			teamId.String())
	}

	// Make the sends non-blocking
	go func(h *connect.Host) {
		// Ping the individual gateway
		pingResponse, err := gw.Comms.SendGatewayPing(h, &messages.Ping{})
		// Attempt to process the response
		failedResponse := &GatewayPingResponse{
			gwId:    h.GetId(),
			success: false,
		}

		// If comm returned error, mark as failure
		if err != nil || pingResponse == nil {
			responseChan <- failedResponse
			return
		}

		// If we cannot process the returned ID, return a failure
		responseId, err := id.Unmarshal(pingResponse.GatewayId)
		if err != nil {
			responseChan <- failedResponse
			return
		}

		// If the returned ID is not expected, return a failure
		if !h.GetId().Cmp(responseId) {
			responseChan <- failedResponse
			return
		}

		// If no errors, send a success
		responseChan <- &GatewayPingResponse{
			gwId:    h.GetId(),
			success: true,
		}
	}(teamHost)

	return nil
}
