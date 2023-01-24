////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/comms/network/dataStructures"
	"gitlab.com/elixxir/crypto/cmix"
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/crypto/sih"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/elixxir/primitives/format"
	"gitlab.com/elixxir/primitives/states"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/comms/messages"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/crypto/xx"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gitlab.com/xx_network/primitives/rateLimiting"
	"google.golang.org/protobuf/proto"
	"sync"
	"sync/atomic"
	"time"
)

// Zeroed identity fingerprint identifies dummy messages
var dummyIdFp = make([]byte, format.SIHLen)
var noConnectionErr = "unable to connect to target host %s."
var noHostErr = "unable to find target host %s."

const RequestKeyThresholdMax = 3 * time.Minute
const RequestKeyThresholdMix = -3 * time.Minute
const sendTimeout = time.Duration(1.3 * float64(time.Second))
const maxManyMessages = 11

// BatchNodeRegistration is the endpoint for fast registration for the client
// It accepts a SignedClientBatchKeyRequest, and forwards the contained
// request and signature to the gateways listed in Targets, returning a list
// of responses after at most Timeout milliseconds.  The returned list will be
// the same size and in the same order as the Targets list received, with
// blank entries where not received in time
func (gw *Instance) BatchNodeRegistration(msg *pb.SignedClientBatchKeyRequest) (*pb.SignedBatchKeyResponse, error) {
	jww.INFO.Printf("Received batch node registration with %d targets", len(msg.Targets))
	request := msg.GetClientKeyRequest()
	sig := msg.GetClientKeyRequestSignature()

	responses := make([]*pb.SignedKeyResponse, len(msg.Targets))
	wg := sync.WaitGroup{}
	for i, target := range msg.Targets {
		wg.Add(1)
		timeout := time.NewTimer(time.Millisecond * time.Duration(msg.Timeout))
		internalIndex := i
		internalTarget := target

		go func() {
			respChan := make(chan *pb.SignedKeyResponse)
			go gw.proxyRegistrationHelper(internalTarget, request, msg.UseSHA, sig, respChan)
			select {
			case resp := <-respChan:
				responses[internalIndex] = resp
				timeout.Stop()
			case <-timeout.C:
				jww.ERROR.Printf("Timed out waiting for registration "+
					"response from %+v", internalTarget)
			}
			wg.Done()
		}()
	}

	wg.Wait()

	return &pb.SignedBatchKeyResponse{
		SignedKeys: responses,
	}, nil
}

// proxyRegistrationHelper accepts a target and key request info, forwards to
// the correct gateway when necessary, and returns the received response
func (gw *Instance) proxyRegistrationHelper(target, clientKeyRequest []byte,
	useSha bool, clientKeyRequestSignature *messages.RSASignature,
	respChan chan *pb.SignedKeyResponse) {
	ret := func(resp *pb.SignedKeyResponse) {
		select {
		case respChan <- resp:
		default:
			jww.ERROR.Printf("Failed to send SignedKeyResponse "+
				"for %+v to channel", target)
		}
	}

	targetID, err := id.Unmarshal(target)
	if err != nil {
		errStr := fmt.Sprintf("Could not unmarshal target bytes %+v: %+v", target, err)
		jww.DEBUG.Printf(errStr)
		ret(&pb.SignedKeyResponse{Error: errStr})
		return
	}

	targetRequest := &pb.SignedClientKeyRequest{
		ClientKeyRequest:          clientKeyRequest,
		ClientKeyRequestSignature: clientKeyRequestSignature,
		Target:                    target,
		UseSHA:                    useSha,
	}
	if !gw.Comms.GetId().Cmp(targetID) {
		// Check if the host exists and is connected
		host, exists := gw.Comms.GetHost(targetID)
		if !exists {
			errStr := errors.Errorf(noHostErr, targetID).Error()
			ret(&pb.SignedKeyResponse{Error: errStr})
			return
		}
		connected, _ := host.Connected()
		if !connected {
			errStr := errors.Errorf(noConnectionErr, targetID).Error()
			ret(&pb.SignedKeyResponse{Error: errStr})
			return
		}
		resp, err := gw.Comms.SendRequestClientKey(host, targetRequest, sendTimeout)
		if err != nil {
			errStr := fmt.Sprintf("Failed to send client key request to target: %+v", err)
			ret(&pb.SignedKeyResponse{Error: errStr})
			return
		}
		jww.DEBUG.Printf("Received node registration response %+v", resp)
		ret(resp)
		return
	} else {
		resp, err := gw.requestClientKeyHelper(targetRequest)
		if err != nil {
			errStr := fmt.Sprintf("Failed to process client key request: %+v", err)
			ret(&pb.SignedKeyResponse{Error: errStr})
			return
		}
		jww.DEBUG.Printf("Processed node registration, returning response %+v", resp)
		ret(resp)
		return
	}
}

// RequestClientKey is the endpoint for a client trying to register with a node.
// It checks if the request made is valid. If valid, it sends the request to
// the node for further checks and cryptographic operations. The node's response
// is then handled by the gateway, storing the relationship key and clearing it
// to ensure a proxy does not get that information.
func (gw *Instance) RequestClientKey(msg *pb.SignedClientKeyRequest) (*pb.SignedKeyResponse, error) {
	// If the target is nil or empty, consider the target itself
	if msg.GetTarget() != nil && len(msg.GetTarget()) > 0 {
		// Unmarshal target ID
		targetID, err := id.Unmarshal(msg.GetTarget())
		if err != nil {
			return nil, errors.Errorf("failed to unmarshal target ID: %+v", err)
		}

		// Check if the target is not itself
		if !gw.Comms.GetId().Cmp(targetID) {
			// Check if the host exists and is connected
			host, exists := gw.Comms.GetHost(targetID)
			if !exists {
				return nil, errors.Errorf(noHostErr, targetID)
			}
			connected, _ := host.Connected()
			if !connected {
				return nil, errors.Errorf(noConnectionErr, targetID)
			}

			return gw.Comms.SendRequestClientKey(host, msg, sendTimeout)
		}
	}

	return gw.requestClientKeyHelper(msg)
}

func (gw *Instance) requestClientKeyHelper(msg *pb.SignedClientKeyRequest) (*pb.SignedKeyResponse, error) {
	// Parse serialized data into message
	request := &pb.ClientKeyRequest{}
	err := proto.Unmarshal(msg.ClientKeyRequest, request)
	if err != nil {
		return nil, errors.Errorf("Couldn't parse client key request: %v", err)
	}

	// Check if the request timestamp is within a valid threshold
	requestTime := time.Unix(0, request.RequestTimestamp)
	if time.Since(requestTime) > RequestKeyThresholdMax ||
		time.Until(requestTime) < RequestKeyThresholdMix {
		errMsg := errors.New("Request timestamp is beyond acceptable threshold")
		return &pb.SignedKeyResponse{Error: errMsg.Error()}, errMsg
	}

	jww.INFO.Print("Passing on client key request")

	// Send request to the node
	resp, err := gw.Comms.SendRequestClientKeyMessage(gw.ServerHost, msg)
	if err != nil {
		return &pb.SignedKeyResponse{Error: err.Error()}, err
	}

	opts := rsa.NewDefaultOptions()
	h := opts.Hash.New()

	// Hash serialized response
	h.Reset()
	h.Write(resp.KeyResponse)
	hashedResponse := h.Sum(nil)

	// Sign the response
	signedResponse, err := rsa.Sign(rand.Reader,
		gw.Comms.GetPrivateKey(), opts.Hash, hashedResponse, opts)
	if err != nil {
		errMsg := errors.Errorf("Could not sign key response: %v", err)
		return &pb.SignedKeyResponse{Error: errMsg.Error()}, errMsg
	}

	// Populate signed response with signature by gateway
	// so client can verify it. Client does not have a node's key information,
	// so signature must be performed by gateway.
	resp.KeyResponseSignedByGateway = &messages.RSASignature{Signature: signedResponse}

	// Unmarshal response
	keyResp := &pb.ClientKeyResponse{}
	err = proto.Unmarshal(resp.KeyResponse, keyResp)
	if err != nil {
		errMsg := errors.Errorf("Failed to unmarshal response from node: %v", err)
		return &pb.SignedKeyResponse{Error: errMsg.Error()}, errMsg
	}

	// Parse serialized transmission confirmation into message
	clientTransmissionConfirmation := &pb.ClientRegistrationConfirmation{}
	err = proto.Unmarshal(request.ClientTransmissionConfirmation.
		ClientRegistrationConfirmation, clientTransmissionConfirmation)
	if err != nil {
		errMsg := errors.Errorf("Couldn't parse client registration confirmation: %v", err)
		return &pb.SignedKeyResponse{Error: errMsg.Error()}, errMsg
	}

	// Extract RSA pubkey
	clientRsaPub := clientTransmissionConfirmation.RSAPubKey

	// Assemble Client public key into rsa.PublicKey
	userPublicKey, err := rsa.LoadPublicKeyFromPem([]byte(clientRsaPub))
	if err != nil {
		errMsg := errors.Errorf("Unable to decode client RSA Pub Key: %+v", err)
		return &pb.SignedKeyResponse{Error: errMsg.Error()}, errMsg
	}

	// Generate UserID
	userId, err := xx.NewID(userPublicKey, request.GetSalt(), id.User)
	if err != nil {
		errMsg := errors.Errorf("Failed to generate new ID: %+v", err)

		return &pb.SignedKeyResponse{Error: errMsg.Error()}, errMsg
	}

	// Insert client information to database
	newClient := &storage.Client{
		Id:  userId.Bytes(),
		Key: resp.ClientGatewayKey,
	}

	// Clear client gateway key so the proxy gateway cannot see it
	resp.ClientGatewayKey = make([]byte, 0)

	err = gw.storage.UpsertClient(newClient)
	if err != nil {
		return resp, nil
	}

	return resp, nil
}

// RequestMessages Client -> Gateway handler. Looks up messages based on a userID and a roundID.
// If the gateway participated in this round, and the requested client had messages in that round,
// we return these message(s) to the requester
func (gw *Instance) RequestMessages(req *pb.GetMessages) (*pb.GetMessagesResponse, error) {
	// Error check for an invalidly crafted message
	if req == nil || req.ClientID == nil || req.RoundID == 0 {
		return &pb.GetMessagesResponse{}, errors.New("Could not parse message! " +
			"Please try again with a properly crafted message!")
	}

	// If the target is nil or empty, consider the target itself
	if req.GetTarget() != nil && len(req.GetTarget()) > 0 {
		// Unmarshal target ID
		targetID, err := id.Unmarshal(req.GetTarget())
		if err != nil {
			return nil, errors.Errorf("failed to unmarshal target ID: %+v", err)
		}

		// Check if the target is not itself
		if !gw.Comms.GetId().Cmp(targetID) {
			// Check if the host exists and is connected
			host, exists := gw.Comms.GetHost(targetID)
			if !exists {
				return nil, errors.Errorf(noHostErr, targetID)
			}
			connected, _ := host.Connected()
			if !connected {
				return nil, errors.Errorf(noConnectionErr, targetID)
			}

			return gw.Comms.SendRequestMessages(host, req, sendTimeout)
		}
	}

	// Parse the requested clientID within the message for the database request
	userId, err := ephemeral.Marshal(req.ClientID)
	if err != nil {
		return &pb.GetMessagesResponse{}, errors.Errorf("Could not parse requested user ID: %+v", err)
	}

	// Parse the roundID within the message
	roundID := id.Round(req.RoundID)

	// Search the database for the requested messages
	msgs, hasRound, err := gw.storage.GetMixedMessages(userId, roundID)
	if err != nil {
		jww.WARN.Printf("Could not find any MixedMessages with "+
			"recipient ID %v and round ID %v: %+v", userId, roundID, err)
		return &pb.GetMessagesResponse{HasRound: hasRound},
			errors.Errorf("Could not find any MixedMessages with recipient "+
				"ID %d and round ID %v: %+v", userId.Int64(), roundID, err)
	} else if len(msgs) == 0 {
		jww.WARN.Printf("A client (%v) has requested messages for a "+
			"round (%v) which is not recorded with messages", userId, roundID)
		return &pb.GetMessagesResponse{HasRound: hasRound}, nil
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
		jww.DEBUG.Printf("Message Retrieved for: %d", userId.Int64())

		slots = append(slots, data)
	}

	// Return all messages to the requester
	return &pb.GetMessagesResponse{
		Messages: slots,
		HasRound: hasRound,
	}, nil

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
	retrievedRounds, err := gw.storage.RetrieveMany(roundIds)
	if err != nil {
		return &pb.HistoricalRoundsResponse{}, errors.New("Could not look up rounds requested.")
	}

	// Return the retrievedRounds
	return &pb.HistoricalRoundsResponse{
		Rounds: retrievedRounds,
	}, nil

}

// PutMessage adds a message to the outgoing queue
func (gw *Instance) PutMessage(msg *pb.GatewaySlot, ipAddr string) (*pb.GatewaySlotResponse, error) {

	// Reject messages with too many ephemeral keys
	numEphemeral := 0
	for _, isEphemeral := range msg.Message.EphemeralKeys {
		if isEphemeral {
			numEphemeral += 1
		}
	}
	threshold := 3
	if len(msg.Message.EphemeralKeys) > 0 && len(msg.Message.EphemeralKeys)-numEphemeral < threshold {
		return nil, errors.Errorf("Too many ephemeral keys in message (%d/%d)", numEphemeral, len(msg.Message.EphemeralKeys))
	}

	// If the target is nil or empty, consider the target itself
	if msg.GetTarget() != nil && len(msg.GetTarget()) > 0 {
		// Unmarshal target ID
		targetID, err := id.Unmarshal(msg.GetTarget())
		if err != nil {
			return nil, errors.Errorf("failed to unmarshal target ID: %+v", err)
		}

		// Check if the target is not itself (ie this gateway is a proxy to the
		// intended recipient)
		if !gw.Comms.GetId().Cmp(targetID) {
			// Check if the host exists and is connected
			host, exists := gw.Comms.GetHost(targetID)
			if !exists {
				return nil, errors.Errorf(noHostErr, targetID)
			}
			connected, _ := host.Connected()
			if !connected {
				return nil, errors.Errorf(noConnectionErr, targetID)
			}

			msg.IpAddr = ipAddr

			resp, err := gw.Comms.SendPutMessageProxy(host, msg, sendTimeout)
			if err != nil {
				if connect.IsAuthError(err) {
					return nil, errors.Errorf(noConnectionErr, targetID)
				}
				return nil, err
			}

			return resp, nil
		}
	}

	return gw.handlePutMessage(msg, ipAddr)

}

// PutMessageProxy is the function which handles a PutMessage proxy from another gateway
func (gw *Instance) PutMessageProxy(msg *pb.GatewaySlot, auth *connect.Auth) (*pb.GatewaySlotResponse, error) {
	// Ensure poller is properly authenticated
	if !auth.IsAuthenticated {
		return nil, connect.AuthError(auth.Sender.GetId())
	}

	return gw.handlePutMessage(msg, msg.IpAddr)
}

// Helper function which handles the logic for a put message request
func (gw *Instance) handlePutMessage(msg *pb.GatewaySlot, ipAddr string) (*pb.GatewaySlotResponse, error) {
	if result, err := gw.processPutMessage(msg); err != nil {
		return result, err
	}

	thisRound := id.Round(msg.RoundID)

	// Rate limit messages
	senderId, err := id.Unmarshal(msg.GetMessage().GetSenderID())
	if err != nil {
		return nil, errors.Errorf("Unable to unmarshal sender ID: %+v", err)
	}

	capacity, leaked, duration := gw.GetRateLimitParams()

	// Check if either the ID or IP is whitelisted
	isIpAddrSuccess, isIpAddrWhitelisted := gw.ipAddrRateLimiting.LookupBucket(ipAddr).AddWithExternalParams(1, capacity, leaked, duration)
	idBucketSuccess, isIdWhitelisted := gw.idRateLimiting.LookupBucket(senderId.String()).AddWithExternalParams(1, capacity, leaked, duration)
	if !(isIpAddrWhitelisted || isIdWhitelisted) &&
		!(isIpAddrSuccess && idBucketSuccess) {
		return nil, errors.Errorf(rateLimiting.ClientRateLimitErr, senderId.String(), ipAddr)
	}

	if err := gw.UnmixedBuffer.AddUnmixedMessage(msg.Message, senderId, ipAddr, thisRound); err != nil {
		return &pb.GatewaySlotResponse{Accepted: false},
			errors.WithMessage(err, "could not add to round. "+
				"Please try a different round.")
	}

	if jww.GetLogThreshold() <= jww.LevelDebug {
		msgFmt := format.NewMessage(gw.NetInf.GetCmixGroup().GetP().ByteLen())
		msgFmt.SetPayloadA(msg.Message.PayloadA)
		msgFmt.SetPayloadB(msg.Message.PayloadB)
		jww.DEBUG.Printf("Putting message from user %s (msgDigest: %s) "+
			"in outgoing queue for round %d...", senderId.String(),
			msgFmt.Digest(), thisRound)
	}

	return &pb.GatewaySlotResponse{
		Accepted: true,
		RoundID:  msg.GetRoundID(),
	}, nil
}

// PutManyMessages adds many messages to the outgoing queue
func (gw *Instance) PutManyMessages(messages *pb.GatewaySlots, ipAddr string) (*pb.GatewaySlotResponse, error) {
	if messages == nil || messages.GetMessages() == nil || len(messages.GetMessages()) == 0 {
		return nil, errors.Errorf("Malformed message object received: %+v", messages)
	}
	// If the target is nil or empty, consider the target itself
	if messages.GetMessages()[0].GetTarget() != nil && len(messages.GetTarget()) > 0 {
		// Unmarshal target ID
		targetID, err := id.Unmarshal(messages.GetTarget())
		if err != nil {
			return nil, errors.Errorf("failed to unmarshal target ID: %+v", err)
		}

		// Check if the target is not itself
		if !gw.Comms.GetId().Cmp(targetID) {
			// Check if the host exists and is connected
			host, exists := gw.Comms.GetHost(targetID)
			if !exists {
				return nil, errors.Errorf(noHostErr, targetID)
			}
			connected, _ := host.Connected()
			if !connected {
				return nil, errors.Errorf(noConnectionErr, targetID)
			}

			messages.IpAddr = ipAddr

			return gw.Comms.SendPutManyMessagesProxy(host, messages, sendTimeout)
		}
	}

	return gw.handlePutManyMessage(messages, ipAddr)
}

// PutManyMessagesProxy is the function which handles a PutManyMessage proxy from another gateway
func (gw *Instance) PutManyMessagesProxy(msg *pb.GatewaySlots, auth *connect.Auth) (*pb.GatewaySlotResponse, error) {

	// Ensure poller is properly authenticated
	if !auth.IsAuthenticated {
		return nil, connect.AuthError(auth.Sender.GetId())
	}

	return gw.handlePutManyMessage(msg, msg.IpAddr)
}

func (gw *Instance) handlePutManyMessage(messages *pb.GatewaySlots, ipAddr string) (*pb.GatewaySlotResponse, error) {
	if len(messages.Messages) > maxManyMessages {
		return nil, errors.Errorf("Cannot process PutManyMessages with "+
			"more than %d messages, recevied %d", maxManyMessages, len(messages.Messages))
	}

	// Report message addition to log (on DEBUG)
	senderId, err := id.Unmarshal(messages.Messages[0].GetMessage().GetSenderID())
	if err != nil {
		return nil, errors.Errorf("Unable to unmarshal sender ID: %+v", err)
	}

	// Process all messages to be queued
	for i := 0; i < len(messages.Messages); i++ {
		if result, err := gw.processPutMessage(messages.Messages[i]); err != nil {
			return result, err
		}
	}

	// Check if either the ID or IP is whitelisted
	capacity, leaked, duration := gw.GetRateLimitParams()
	isIpAddrSuccess, isIpAddrWhitelisted := gw.ipAddrRateLimiting.LookupBucket(ipAddr).AddWithExternalParams(uint32(len(messages.Messages)), capacity, leaked, duration)
	idBucketSuccess, isIdWhitelisted := gw.idRateLimiting.LookupBucket(senderId.String()).AddWithExternalParams(uint32(len(messages.Messages)), capacity, leaked, duration)
	if !(isIpAddrWhitelisted || isIdWhitelisted) &&
		!(isIpAddrSuccess && idBucketSuccess) {
		return nil, errors.Errorf(rateLimiting.ClientRateLimitErr,
			senderId.String(), ipAddr)
	}

	// Add messages to buffer
	thisRound := id.Round(messages.RoundID)
	err = gw.UnmixedBuffer.AddManyUnmixedMessages(messages.Messages, senderId, ipAddr, thisRound)
	if err != nil {
		return &pb.GatewaySlotResponse{Accepted: false},
			errors.WithMessage(err, "could not add to round. "+
				"Please try a different round.")
	}

	// Print out message if in debug mode
	for i := 0; i < len(messages.Messages); i++ {
		msg := messages.Messages[i]

		if jww.GetLogThreshold() <= jww.LevelDebug {
			msgFmt := format.NewMessage(gw.NetInf.GetCmixGroup().GetP().ByteLen())
			msgFmt.SetPayloadA(msg.Message.PayloadA)
			msgFmt.SetPayloadB(msg.Message.PayloadB)
			jww.DEBUG.Printf("Putting message from user %s (msgDigest: %s) "+
				"in outgoing queue for round %d...", senderId.String(),
				msgFmt.Digest(), thisRound)
		}
	}

	return &pb.GatewaySlotResponse{
		Accepted: true,
		RoundID:  messages.GetRoundID(),
	}, nil

}

// Helper function which processes a single gateway slot. Checks the mac for
// a singular message
func (gw *Instance) processPutMessage(message *pb.GatewaySlot) (*pb.GatewaySlotResponse, error) {

	// Construct Client ID for database lookup
	clientID, err := id.Unmarshal(message.Message.SenderID)
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
	clientMac := generateClientMac(cl, message)
	if !bytes.Equal(clientMac, message.MAC) {
		return &pb.GatewaySlotResponse{
				Accepted: false,
			}, errors.Errorf("Could not authenticate client. Is the "+
				"client registered with this node (%s)?",
				gw.ServerHost.GetId())
	}

	// fixme: enable once gossip is not broken
	/*if !gw.Params.DisableGossip {
		err = gw.FilterMessage(senderId)
		if err != nil {
			jww.INFO.Printf("Rate limiting check failed on send message from "+
				"%v", msg.Message.GetSenderID())
			return &pb.GatewaySlotResponse{
				Accepted: false,
			}, err
		}
	}*/

	return nil, nil
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

// GenJunkMsg generates a junk message using the gateway's client key
func GenJunkMsg(grp *cyclic.Group, numNodes int, msgNum uint32, roundID id.Round) *pb.Slot {

	baseKey := grp.NewIntFromBytes(id.DummyUser[:])
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

	ephId, _, _, err := ephemeral.GetId(&id.DummyUser, 64, time.Now().UnixNano())
	if err != nil {
		jww.FATAL.Panicf("Could not get ID: %+v", err)
	}
	msg.SetEphemeralRID(ephId[:])
	msg.SetSIH(dummyIdFp)

	ecrMsg := cmix.ClientEncrypt(grp, msg, salt, baseKeys, roundID)

	h, err := hash.NewCMixHash()
	if err != nil {
		jww.FATAL.Printf("Could not get hash: %+v", err)
	}

	KMACs := cmix.GenerateKMACs(salt, baseKeys, roundID, h)
	ephKeys := make([]bool, len(KMACs))
	return &pb.Slot{
		PayloadB:      ecrMsg.GetPayloadB(),
		PayloadA:      ecrMsg.GetPayloadA(),
		Salt:          salt,
		SenderID:      id.DummyUser.Marshal(),
		KMACs:         KMACs,
		EphemeralKeys: ephKeys,
	}
}

// UploadUnmixedBatch polls sends whatever messages are in the batch associated with the
// requested round to the server
func (gw *Instance) UploadUnmixedBatch(roundInfo *pb.RoundInfo) {

	batchSize := uint64(roundInfo.BatchSize)
	if batchSize == 0 {
		jww.WARN.Printf("Server sent empty roundBufferSize!")
		return
	}

	rid := id.Round(roundInfo.ID)

	batch, senders, ips := gw.UnmixedBuffer.PopRound(rid)

	if batch == nil {
		jww.ERROR.Printf("Batch for %v not found!", roundInfo.ID)
		return
	}

	batch.Round = roundInfo

	jww.INFO.Printf("Sending batch for round %d with %d messages...",
		roundInfo.ID, len(batch.Slots))

	numNodes := len(roundInfo.GetTopology())

	if numNodes == 0 {
		jww.ERROR.Println("Round topology empty, sending bad messages!")
	}

	header := pb.BatchInfo{
		BatchSize: uint32(batchSize),
		Round:     roundInfo,
		FromPhase: batch.FromPhase,
	}

	// Now fill with junk and send
	for i := uint64(len(batch.Slots)); i < batchSize; i++ {
		junkMsg := GenJunkMsg(gw.NetInf.GetCmixGroup(), numNodes,
			uint32(i), rid)
		batch.Slots = append(batch.Slots, junkMsg)
	}

	jww.INFO.Printf("Uploading batch to server")
	err := gw.Comms.UploadUnmixedBatch(gw.ServerHost, header, batch)
	if err != nil {
		// TODO: handle failure sending batch
		jww.WARN.Printf("Error streaming unmixed batch: %v", err)
	}
	jww.INFO.Printf("Upload complete")

	if !gw.Params.DisableGossip {
		// Gossip senders included in the batch to other gateways
		go func() {
			err = gw.GossipBatch(rid, senders, ips)
			if err != nil {
				jww.ERROR.Printf("Unable to rate limit gossip batch "+
					"information for round %d: %+v", rid, err)
			}
			jww.INFO.Printf("Sent rate limit gossip for round %d,"+
				" with %d ips and %d senders", rid,
				len(ips), len(senders))
		}()

	}
}

// Amount of time process batch will wait until round data is available
// Will bail otherwise
const roundLookupTimeout = 3 * time.Second

// ProcessCompletedBatch handles messages coming out of the mixnet
func (gw *Instance) ProcessCompletedBatch(msgs []*pb.Slot, roundID id.Round) error {
	if len(msgs) == 0 {
		return nil
	}

	// get the round for processing
	round, err := gw.NetInf.GetRound(roundID)
	if err != nil {
		jww.ERROR.Printf("ProcessCompleted - Unable to get "+
			"round %d: %+v", roundID, err)
	}
	// if the round was not retrieved, wait for it to become available up to 3 seconds
	if round == nil || states.Round(round.State) < states.QUEUED {
		if round == nil {
			jww.WARN.Printf("Failed to get the data about round %d for storage and gossip, "+
				"waiting to %s for data ", roundID, roundLookupTimeout)
		} else {
			jww.WARN.Printf("Failed to up to date data about round %d for storage and gossip, "+
				"round in incorrect state (%s vs %s), waiting to %s for data ", roundID,
				states.Round(round.State), states.QUEUED, roundLookupTimeout)
		}
		roundUpdateCh := make(chan dataStructures.EventReturn)

		// use round events to wait for the update
		gw.NetInf.GetRoundEvents().AddRoundEventChan(roundID, roundUpdateCh, roundLookupTimeout,
			states.QUEUED, states.REALTIME, states.COMPLETED)
		roundEvent := <-roundUpdateCh
		round = roundEvent.RoundInfo
		if roundEvent.TimedOut || round == nil {
			return errors.Errorf("Failed to get round %d after %s second wait, "+
				"cannot process batch, timed out: %t", roundID, roundLookupTimeout,
				roundEvent.TimedOut)
		}
	}

	// process the messages
	recipients, clientRound, notifications := gw.processMessages(msgs, roundID, round)

	// upsert messages to the database
	errMsg := gw.storage.InsertMixedMessages(clientRound)
	if errMsg != nil {
		jww.ERROR.Printf("Inserting new mixed messages failed in "+
			"ProcessCompletedBatch for round %d: %+v", roundID, errMsg)
	}

	// Gossip recipients included in the completed batch to other gateways
	// in a new thread
	if !gw.Params.DisableGossip {
		jww.INFO.Printf("Sending bloom gossip (source thread) for round %d", roundID)
		go func() {
			jww.INFO.Printf("Sending bloom gossip (new thread) for round %d", roundID)
			errGossip := gw.GossipBloom(recipients, roundID, int64(round.Timestamps[states.QUEUED]))
			if err != nil {
				jww.ERROR.Printf("Unable to gossip bloom information "+
					"for round %d: %+v", roundID, errGossip)
			}
			jww.INFO.Printf("Sent bloom gossip for round %d", roundID)
		}()

		received := time.Now()

		// WARNING: this needs function IDENTICALLY to the code in gossipBloomFilterReceive in
		// bloomGossip.go, but due to this being hot code, has subtle differences which
		// lead to a different implementation
		//Go through each of the recipients
		go func() {
			gw.bloomFilterGossip.Lock()
			defer gw.bloomFilterGossip.Unlock()
			// Get epoch information
			roundTimestamp := round.Timestamps[states.QUEUED]
			epoch := GetEpoch(int64(roundTimestamp), gw.period)

			var wg sync.WaitGroup

			totalNumAttempts := uint32(0)
			failedInsert := uint32(0)

			for recipient, _ := range recipients {
				wg.Add(1)
				go func(recipientId ephemeral.Id) {
					defer wg.Done()

					// retry insertion into the database in the event that there is an
					// insertion on the same ephemeral id by multiple rounds at the same
					// time, in which case all but one will fail
					i := 0
					var localErr error
					for ; i < bloomUploadRetries && (localErr != nil || i == 0); i++ {
						localErr = gw.UpsertFilter(recipientId, roundID, epoch)
						if localErr != nil {
							jww.WARN.Printf("Failed to upsert recipient %d bloom as team member on "+
								"round %d on attempt %d: %s", recipientId, roundID, i, localErr.Error())
						}
					}

					atomic.AddUint32(&totalNumAttempts, uint32(i))
					if localErr != nil {
						jww.ERROR.Printf("Failed to upsert recipient %d bloom as team member on "+
							"round %d on all attemps(%d/%d): %+v", recipientId, roundID, i, i, localErr)
						atomic.AddUint32(&failedInsert, 1)
					}
				}(recipient)
			}
			wg.Wait()

			finishedInsert := time.Now()
			averageAttempts := float32(totalNumAttempts) / float32(len(recipients))

			if failedInsert == 0 {
				//denote the reception in known rounds
				err = gw.krw.forceCheck(roundID)
				if err != nil {
					jww.ERROR.Printf("Local round data not recorded due to known rounds error for "+
						"round %d with %d recipients at %s: "+
						"\n\t inserts finished at %s, KR insert finished at %s, "+
						"\n]t round started at ts %s, average attempts: %f (total: %d): %+v", roundID,
						len(recipients), received, finishedInsert, time.Now(),
						time.Unix(0, int64(roundTimestamp)), averageAttempts, totalNumAttempts, err)
				} else {
					jww.INFO.Printf("Local round data for round %d with %d recipients at %s: "+
						"\n\t inserts finished at %s, KR insert finished at %s, "+
						"\n]t round started at ts %s, average attempts: %f (total: %d)", roundID,
						len(recipients), received, finishedInsert, time.Now(),
						time.Unix(0, int64(roundTimestamp)), averageAttempts, totalNumAttempts)
				}
			} else {
				jww.ERROR.Printf("Gossip received not recorded due to bloom upsert failures for %d recipeints"+
					" for round %d with %d recipients at %s: "+
					"\n\t inserts finished at %s, KR insert finished at %s, "+
					"\n]t round started at ts %s, average attempts: %f (total: %d)", failedInsert, roundID,
					len(recipients), received, finishedInsert, time.Now(),
					time.Unix(0, int64(roundTimestamp)), averageAttempts, totalNumAttempts)
			}
		}()
	}

	go PrintProfilingStatistics()

	// Send notification data to notification bot
	if gw.NetInf.GetFullNdf().Get().Notification.Address != "" {
		go func(notificationBatch *pb.NotificationBatch, round *pb.RoundInfo) {
			host, exists := gw.Comms.GetHost(&id.NotificationBot)
			if !exists {
				jww.WARN.Printf("Unable to find host for notification bot: %s",
					id.NotificationBot)
				return
			}

			err := gw.Comms.SendNotificationBatch(host, notificationBatch)
			if err != nil {
				jww.ERROR.Printf("Unable to send notification data %s: %+v", notificationBatch, err)
			}
		}(notifications, round)
	} else {
		jww.INFO.Print("Notification bot not found in NDF. Skipping sending of " +
			"notifications.")
	}
	return nil
}

// Helper function which takes passed in messages from a round and
// stores these as mixedMessages
func (gw *Instance) processMessages(msgs []*pb.Slot, roundID id.Round,
	round *pb.RoundInfo) (map[ephemeral.Id]interface{}, *storage.ClientRound, *pb.NotificationBatch) {
	numReal := 0

	// Build a ClientRound object around the client messages
	clientRound := &storage.ClientRound{
		Id:        uint64(roundID),
		Timestamp: time.Unix(0, int64(round.Timestamps[states.QUEUED])),
		Messages:  make([]storage.MixedMessage, 0, len(msgs)),
	}
	recipients := make(map[ephemeral.Id]interface{})
	notifications := &pb.NotificationBatch{
		RoundID:       uint64(roundID),
		Notifications: make([]*pb.NotificationData, 0, len(msgs)),
	}
	// Process the messages into the ClientRound object
	for _, msg := range msgs {
		serialMsg := format.NewMessage(gw.NetInf.GetCmixGroup().GetP().ByteLen())
		serialMsg.SetPayloadA(msg.GetPayloadA())
		serialMsg.SetPayloadB(msg.GetPayloadB())

		// If IdentityFP is not zeroed, the message is not a dummy
		if !bytes.Equal(serialMsg.GetSIH(), dummyIdFp) {
			recipIdBytes := serialMsg.GetEphemeralRID()
			recipientId, err := ephemeral.Marshal(recipIdBytes)
			if err != nil {
				jww.ERROR.Printf("Unable to marshal ID: %+v", err)
				continue
			}

			// Clear random bytes from recipient ID and add to map
			recipientId = recipientId.Clear(uint(round.AddressSpaceSize))
			if recipientId.Int64() != 0 {
				recipients[recipientId] = nil
			}

			// Only print debug statement if debug logging is enabled to avoid
			// wasted resources calculating debug print
			if jww.GetStdoutThreshold() <= jww.LevelDebug {
				jww.DEBUG.Printf("Message received for: %d[%d] in "+
					"round: %d, msgDigest: %s", recipientId.Int64(),
					round.AddressSpaceSize, roundID, serialMsg.Digest())
			}

			// Create new message and add it to the list for insertion
			newMixedMessage := *storage.NewMixedMessage(roundID, recipientId, msg.PayloadA, msg.PayloadB)
			clientRound.Messages = append(clientRound.Messages, newMixedMessage)

			numReal++

			// Add new NotificationData for the message
			notifications.Notifications = append(notifications.Notifications, &pb.NotificationData{
				EphemeralID: recipientId.Int64(),
				IdentityFP:  serialMsg.GetSIH(),
				MessageHash: sih.GetMessageHash(serialMsg.GetContents()),
			})
		}
	}

	jww.INFO.Printf("Round %d received, %d real messages "+
		"processed, %d dummies ignored", clientRound.Id, numReal,
		len(msgs)-numReal)

	return recipients, clientRound, notifications
}

// FilterMessage determines if the message should be kept or discarded based on
// the capacity of the sender's ID bucket.
// todo: determine whether to keep this logic for ratelimiting
//func (gw *Instance) FilterMessage(userId *id.ID) error {
//	// If the user ID bucket is full AND the message's user ID is not on the
//	// whitelist, then reject the message
//	if !gw.rateLimit.LookupBucket(userId.String()).Add(1) {
//		return errors.New("Rate limit exceeded. Try again later.")
//	}
//
//	// Otherwise, if the user ID bucket has room then let the message through
//	return nil
//}
