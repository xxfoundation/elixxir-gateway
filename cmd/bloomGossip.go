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
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"sync"
	"sync/atomic"
	"time"
)

const errorDelimiter = "; "
const bloomUploadRetries = 5

// Initialize fields required for the gossip protocol specialized to bloom filters
func (gw *Instance) InitBloomGossip() {
	flags := gossip.DefaultProtocolFlags()
	flags.FanOut = 25
	flags.MaximumReSends = 2
	flags.NumParallelSends = 1000
	flags.Fingerprinter = func(msg *gossip.GossipMsg) gossip.Fingerprint {
		preSum := append([]byte(msg.Tag), msg.Payload...)
		return gossip.NewFingerprint(preSum)
	}
	// Register gossip protocol for bloom filters
	gw.Comms.Manager.NewGossip(BloomFilterGossip, flags,
		gw.gossipBloomFilterReceive, gw.gossipVerify, nil)
}

// GossipBloom builds a gossip message containing all the recipient IDs
// within the bloom filter and gossips it to all peers
func (gw *Instance) GossipBloom(recipients map[ephemeral.Id]interface{}, roundId id.Round, roundTimestamp int64) error {
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
	gossipMsg.Payload, err = buildGossipPayloadBloom(recipients, roundId, uint64(roundTimestamp))
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

	jww.INFO.Printf("Gossiping Blooms for round %v at ts %s", roundId,
		time.Unix(0, roundTimestamp))

	// Return any errors up the stack
	if len(errs) != 0 {
		jww.TRACE.Printf("Failed to rate limit gossip to: %v", errs)
		return errors.Errorf("Could not send to %d out of %d peers", len(errs), numPeers)
	}
	return nil
}

// Receive function for Gossip messages regarding bloom filters
func (gw *Instance) gossipBloomFilterReceive(msg *gossip.GossipMsg) error {
	gw.bloomFilterGossip.Lock()
	defer gw.bloomFilterGossip.Unlock()

	received := time.Now()

	// Unmarshal the Recipients data
	payloadMsg := &pb.Recipients{}
	err := proto.Unmarshal(msg.Payload, payloadMsg)
	if err != nil {
		return errors.Errorf("Could not unmarshal message into expected format: %s", err)
	}

	roundID := id.Round(payloadMsg.RoundID)

	var wg sync.WaitGroup

	epoch := GetEpoch(int64(payloadMsg.RoundTS), gw.period)

	totalNumAttempts := uint32(0)
	failedInsert := uint32(0)

	// Go through each of the recipients
	for _, recipient := range payloadMsg.RecipientIds {
		wg.Add(1)
		go func(localRecipient []byte) {
			defer wg.Done()
			// Marshal the id
			recipientId, localErr := ephemeral.Marshal(localRecipient)
			if localErr != nil {
				jww.WARN.Printf("Failed to unmarshal recipient %v for "+
					"bloom gossip for round %d: %s", localRecipient, roundID, localErr)
				return
			}

			localErr = errors.New("derp")
			i:=0
			for ;i<bloomUploadRetries && localErr!= nil; i++ {
				localErr = gw.UpsertFilter(recipientId, roundID, epoch)
				if err!=nil{
					jww.WARN.Printf("Failed to upsert recipient %d bloom on " +
						"round %d on attempt %d: %s", localRecipient, roundID, i, localErr.Error())
				}
			}

			atomic.AddUint32(&totalNumAttempts, 1 + uint32(i))
			if err!=nil{
				jww.ERROR.Printf("Failed to upsert recipient %d bloom on " +
					"round %d on all attemps(%d/%d): %+v", localRecipient, roundID, i, i, localErr)
				atomic.AddUint32(&failedInsert, 1)
			}
		}(recipient)
	}
	wg.Wait()

	finishedInsert := time.Now()
	averageAttempts := float32(totalNumAttempts)/float32(len(payloadMsg.RecipientIds))

	if failedInsert!=0{
		//denote the reception in known rounds
		err = gw.krw.forceCheck(roundID, gw.storage)
		if err != nil {
			jww.ERROR.Printf("Gossip received not recorded due to known rounds error for " +
				"round %d with %d recipients at %s: "+
				"\n\t inserts finished at %s, KR insert finished at %s, "+
				"\n]t round started at ts %s, average attempts: %f (total: %d): %+v", roundID,
				len(payloadMsg.RecipientIds), received, finishedInsert, time.Now(),
				time.Unix(0, int64(payloadMsg.RoundTS)), averageAttempts , totalNumAttempts, err)
		}else{
			jww.INFO.Printf("Gossip received for round %d with %d recipients at %s: "+
				"\n\t inserts finished at %s, KR insert finished at %s, "+
				"\n]t round started at ts %s, average attempts: %f (total: %d)", roundID,
				len(payloadMsg.RecipientIds), received, finishedInsert, time.Now(),
				time.Unix(0, int64(payloadMsg.RoundTS)), averageAttempts , totalNumAttempts)
		}
	}else{
		jww.ERROR.Printf("Gossip received not recorded due to bloom upsert failures for %d recipeints" +
			" for round %d with %d recipients at %s: "+
			"\n\t inserts finished at %s, KR insert finished at %s, "+
			"\n]t round started at ts %s, average attempts: %f (total: %d)", failedInsert, roundID,
			len(payloadMsg.RecipientIds), received, finishedInsert, time.Now(),
			time.Unix(0, int64(payloadMsg.RoundTS)), averageAttempts , totalNumAttempts)
	}

	return nil
}

// Helper function used to convert recipientIds into a GossipMsg payload
func buildGossipPayloadBloom(recipientIDs map[ephemeral.Id]interface{}, roundId id.Round, roundTS uint64) ([]byte, error) {
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
		RoundTS:      roundTS,
	}
	return proto.Marshal(payloadMsg)
}
