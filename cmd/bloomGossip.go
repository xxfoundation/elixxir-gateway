///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// Contains gossip methods specific to bloom filter gossiping

package cmd

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/comms/network/dataStructures"
	"gitlab.com/elixxir/primitives/states"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"strings"
	"sync"
	"time"
)

const errorDelimiter = "; "

// Initialize fields required for the gossip protocol specialized to bloom filters
func (gw *Instance) InitBloomGossip() {
	flags := gossip.DefaultProtocolFlags()
	// Register gossip protocol for bloom filters
	gw.Comms.Manager.NewGossip(BloomFilterGossip, flags,
		gw.gossipBloomFilterReceive, gw.gossipVerify, nil)
}

// GossipBloom builds a gossip message containing all of the recipient IDs
// within the bloom filter and gossips it to all peers
func (gw *Instance) GossipBloom(recipients map[ephemeral.Id]interface{}, roundId id.Round) error {
	var err error

	// Retrieve gossip protocol
	gossipProtocol, ok := gw.Comms.Manager.Get(BloomFilterGossip)
	if !ok {
		return errors.Errorf("Unable to get gossip protocol.")
	}

	// Build the message
	gossipMsg := &gossip.GossipMsg{
		Tag:    BloomFilterGossip,
		Origin: gw.Comms.Id.Marshal(),
	}

	// Add the GossipMsg payload
	gossipMsg.Payload, err = buildGossipPayloadBloom(recipients, roundId)
	if err != nil {
		return errors.Errorf("Unable to build gossip payload: %+v", err)
	}

	// Add the GossipMsg signature
	gossipMsg.Signature, err = buildGossipSignature(gossipMsg, gw.Comms.GetPrivateKey())
	if err != nil {
		return errors.Errorf("Unable to build gossip signature: %+v", err)
	}

	// Gossip the message
	numPeers, errs := gossipProtocol.Gossip(gossipMsg)

	jww.INFO.Printf("Gossiping Blooms for round: %v", roundId)

	// Return any errors up the stack
	if len(errs) != 0 {
		jww.TRACE.Printf("Failed to rate limit gossip to: %v", errs)
		return errors.Errorf("Could not send to %d out of %d peers", len(errs), numPeers)
	}
	return nil
}

// Verify bloom is some additional logic to determine if the
// gateway gossiping a message is responsible for the round
// it has gossiped about.
func verifyBloom(msg *gossip.GossipMsg, origin *id.ID, instance *network.Instance) error {
	jww.DEBUG.Printf("Verifying gossip message from %+v", origin)

	// Parse the payload message
	payloadMsg := &pb.Recipients{}
	err := proto.Unmarshal(msg.Payload, payloadMsg)
	if err != nil {
		return errors.Errorf("Could not unmarshal message into expected "+
			"format: %s", err)
	}

	// Create channel to receive round info when it is available
	r := id.Round(payloadMsg.RoundID)
	var ri *pb.RoundInfo
	ri, err = instance.GetRound(r)
	//if we cant find the round, wait unill we have an update
	//fixme: there is a potential race codnition if the round comes in after the
	//above call and before the below
	if err != nil {
		eventChan := make(chan dataStructures.EventReturn, 1)
		instance.GetRoundEvents().AddRoundEventChan(r,
			eventChan, 60*time.Second, states.PRECOMPUTING, states.QUEUED,
			states.REALTIME, states.COMPLETED, states.FAILED)

		// Check if we recognize the round
		event := <-eventChan

		if event.RoundInfo == nil {
			jww.WARN.Printf("Bailing on processing bloom goosip %d network "+
				"round lookup returned nil without na error", r)
			return errors.Errorf("Round %v sent out by gossip message failed lookup", payloadMsg.RoundID)
		}

		if event.TimedOut {
			return errors.Errorf("Failed to lookup round %v sent out by gossip message.", payloadMsg.RoundID)
		} else if states.Round(event.RoundInfo.State) == states.FAILED {
			return errors.Errorf("Round %v sent out by gossip message failed.", payloadMsg.RoundID)
		}

		ri = event.RoundInfo
	} else if ri == nil {
		jww.WARN.Printf("Bailing on processing bloom goosip %d ram "+
			"round lookup returned nil without an error", r)
		return errors.Errorf("Round %v sent out by gossip message failed lookup", payloadMsg.RoundID)
	}

	// Parse the round topology
	idList, err := id.NewIDListFromBytes(ri.Topology)
	if err != nil {
		return errors.Errorf("Could not read topology from gossip message: %s",
			err)
	}
	topology := connect.NewCircuit(idList)

	// Check if the sender is in the round that we have tracked
	senderIdCopy := origin.DeepCopy()
	senderIdCopy.SetType(id.Node)
	if topology.GetNodeLocation(senderIdCopy) < 0 {
		return errors.Errorf("Origin gateway (%s) is not in round "+
			"it's gossiping about (rid: %d)", senderIdCopy, ri.ID)
	}
	jww.DEBUG.Printf("Verified gossip message from %+v", origin)

	return nil
}

// Receive function for Gossip messages regarding bloom filters
func (gw *Instance) gossipBloomFilterReceive(msg *gossip.GossipMsg) error {
	gw.bloomFilterGossip.Lock()
	defer gw.bloomFilterGossip.Unlock()

	// Unmarshal the Recipients data
	payloadMsg := &pb.Recipients{}
	err := proto.Unmarshal(msg.Payload, payloadMsg)
	if err != nil {
		return errors.Errorf("Could not unmarshal message into expected format: %s", err)
	}

	var errs []string
	var wg sync.WaitGroup

	roundID := id.Round(payloadMsg.RoundID)
	jww.INFO.Printf("Gossip received for round %d", roundID)
	round, err := gw.NetInf.GetRound(roundID)

	//in the event that rounds are too early, wait until we get the update
	if err != nil || states.Round(round.State) < states.QUEUED {
		eventChan := make(chan dataStructures.EventReturn, 1)
		gw.NetInf.GetRoundEvents().AddRoundEventChan(roundID,
			eventChan, 3*time.Minute, states.QUEUED, states.REALTIME,
			states.COMPLETED, states.FAILED)

		// Check if we recognize the round
		event := <-eventChan
		round = event.RoundInfo
	}

	//check that we have a valid round
	if round == nil {
		// handle the potential race condition between the previous two round
		// calls which would result in not getting the round
		round, err = gw.NetInf.GetRound(roundID)
		if err != nil {
			return errors.WithMessagef(err, "Failed to find round %d to "+
				"process gossip", roundID)
		} else if states.Round(round.State) < states.QUEUED {
			return errors.WithMessagef(err, "Failed to find round %d with "+
				"enough info to process gossip", roundID)
		}
	}

	epoch := GetEpoch(int64(round.Timestamps[states.QUEUED]), gw.period)

	// Go through each of the recipients
	for _, recipient := range payloadMsg.RecipientIds {
		wg.Add(1)
		go func(localRecipient []byte) {
			// Marshal the id
			recipientId, err := ephemeral.Marshal(localRecipient)
			if err != nil {
				errs = append(errs, err.Error())
				return
			}

			err = gw.UpsertFilter(recipientId, roundID, epoch)
			if err != nil {
				errs = append(errs, err.Error())
			}
			wg.Done()
		}(recipient)

	}
	wg.Wait()

	//denote the reception in known rounds
	gw.knownRound.ForceCheck(roundID)
	if err := gw.SaveKnownRounds(); err != nil {
		jww.ERROR.Printf("Failed to store updated known rounds: %s", err)
	}

	// Parse through the errors returned from the worker pool
	var errReturn error
	if len(errs) > 0 {
		errReturn = errors.New(strings.Join(errs, errorDelimiter))
	}

	return errReturn
}

// Helper function used to convert recipientIds into a GossipMsg payload
func buildGossipPayloadBloom(recipientIDs map[ephemeral.Id]interface{}, roundId id.Round) ([]byte, error) {
	// Iterate over the map, placing keys back in a list
	// without any duplicates
	i := 0
	recipients := make([][]byte, len(recipientIDs))
	for key := range recipientIDs {
		recipients[i] = make([]byte, len(key))
		copy(recipients[i], key[:])
		i++
	}

	// Build the message payload and return
	payloadMsg := &pb.Recipients{
		RecipientIds: recipients,
		RoundID:      uint64(roundId),
	}
	return proto.Marshal(payloadMsg)
}
