///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////
package cmd

import (
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/xx_network/comms/messages"
	"gitlab.com/xx_network/primitives/id"
)

// GatewayPingResponse returns to the main thread
// the process of the asynchronous gateway ping.
type GatewayPingResponse struct {
	// The Id of the gateway that has been pinged
	gwId *id.ID

	// Status of the ping. Any error sets to false
	success bool
}

// checkGatewayPings asynchronously pings all gateways in the team (besides itself)
// It then reports the pinging results to it's node once all gateways pings have been attempted
func (gw *Instance) checkGatewayPings(pingRequest *pb.GatewayPingRequest) (*pb.GatewayPingReport, error) {
	// Process round topology into IDs
	idList, err := id.NewIDListFromBytes(pingRequest.Topology)
	if err != nil {
		return nil, errors.Errorf("Could not read topology from round %d: %+v", pingRequest.RoundId, err)
	}

	pingResponseChan := make(chan *GatewayPingResponse, len(pingRequest.Topology)-1)

	// Send gatewayPing to other gateways in team, excluding self
	for _, teamId := range idList {
		go func(nodeId *id.ID) {
			gw.pingGateway(nodeId, id.Round(pingRequest.RoundId), pingResponseChan)
		}(teamId)

	}

	report := &pb.GatewayPingReport{
		RoundId: pingRequest.RoundId,
	}

	// Exhaust response channel
	pingResponses := make([]*GatewayPingResponse, 0)
	done := false
	for !done {
		select {
		case response := <-pingResponseChan:
			pingResponses = append(pingResponses, response)
			// If we've gotten responses from all gateways, we are done
			done = len(pingResponses) == len(idList)
		}
	}

	// Process all the responses, sending to our node the gateway ID's
	// that we could not ping
	for _, response := range pingResponses {
		if !response.success {
			report.FailedGateways = append(report.FailedGateways, response.gwId.Bytes())
			jww.TRACE.Printf("Failed gateway: %v", response.gwId.String())
		}
	}

	jww.TRACE.Printf("Round %d had %d gateway failures", pingRequest.RoundId, len(report.FailedGateways))


	return report, nil
}

// pingGateway is a helper function which pings an individual gateway.
// Returns via the responseChan either:
//  - failure if there is any error in the comm or
//  	errors in processing the response
//  - success otherwise
func (gw *Instance) pingGateway(teamId *id.ID, roundId id.Round, responseChan chan *GatewayPingResponse) {

	// Preset a failed ping response
	failedResponse := &GatewayPingResponse{
		gwId:    teamId,
		success: false,
	}

	// Set the Id to a gateway (Id is defaulted to node type)
	// Skip sending to ourselves
	teamId.SetType(id.Gateway)
	if teamId.Cmp(gw.Comms.Id) {
		responseChan <- &GatewayPingResponse{
			gwId:    teamId,
			success: true,
		}
		return
	}

	// Get the gateway host
	teamHost, exists := gw.Comms.GetHost(teamId)
	if !exists {
		responseChan <- failedResponse
		return
	}

	// Ping the individual gateway
	jww.TRACE.Printf("Pinging gateway %v for round %d", teamId, roundId)
	pingResponse, err := gw.Comms.SendGatewayPing(teamHost, &messages.Ping{})
	// If comm returned error, mark as failure
	if err != nil || pingResponse == nil {
		responseChan <- failedResponse
		return
	}

	jww.TRACE.Printf("Successfully pinged gateway %v for round %d", teamId, roundId)

	// If we cannot process the returned ID, return a failure
	responseId, err := id.Unmarshal(pingResponse.GatewayId)
	if err != nil {
		responseChan <- failedResponse
		return
	}

	// If the returned ID is not expected, return a failure
	if !teamHost.GetId().Cmp(responseId) {
		responseChan <- failedResponse
		return
	}

	// If no errors, send a success
	responseChan <- &GatewayPingResponse{
		gwId:    teamId,
		success: true,
	}

	return
}
