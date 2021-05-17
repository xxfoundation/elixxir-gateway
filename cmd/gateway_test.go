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
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/comms/gateway"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/comms/node"
	"gitlab.com/elixxir/comms/testkeys"
	"gitlab.com/elixxir/comms/testutils"
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/elixxir/primitives/format"
	"gitlab.com/elixxir/primitives/knownRounds"
	"gitlab.com/elixxir/primitives/states"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/comms/signature"
	"gitlab.com/xx_network/crypto/large"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"gitlab.com/xx_network/primitives/ndf"
	"gitlab.com/xx_network/primitives/rateLimiting"
	"gitlab.com/xx_network/primitives/utils"
)

const GW_ADDRESS = "0.0.0.0:5555"
const NODE_ADDRESS = "0.0.0.0:5556"

var gatewayInstance *Instance
var gComm *gateway.Comms
var n *node.Comms

var mockMessage *pb.Slot
var nodeIncomingBatch *pb.Batch

var gatewayCert []byte
var gatewayKey []byte

var nodeCert []byte
var nodeKey []byte

var grp map[string]string

const prime = "" +
	"9DB6FB5951B66BB6FE1E140F1D2CE5502374161FD6538DF1648218642F0B5C48" +
	"C8F7A41AADFA187324B87674FA1822B00F1ECF8136943D7C55757264E5A1A44F" +
	"FE012E9936E00C1D3E9310B01C7D179805D3058B2A9F4BB6F9716BFE6117C6B5" +
	"B3CC4D9BE341104AD4A80AD6C94E005F4B993E14F091EB51743BF33050C38DE2" +
	"35567E1B34C3D6A5C0CEAA1A0F368213C3D19843D0B4B09DCB9FC72D39C8DE41" +
	"F1BF14D4BB4563CA28371621CAD3324B6A2D392145BEBFAC748805236F5CA2FE" +
	"92B871CD8F9C36D3292B5509CA8CAA77A2ADFC7BFD77DDA6F71125A7456FEA15" +
	"3E433256A2261C6A06ED3693797E7995FAD5AABBCFBE3EDA2741E375404AE25B"
const generator = "" +
	"5C7FF6B06F8F143FE8288433493E4769C4D988ACE5BE25A0E24809670716C613" +
	"D7B0CEE6932F8FAA7C44D2CB24523DA53FBE4F6EC3595892D1AA58C4328A06C4" +
	"6A15662E7EAA703A1DECF8BBB2D05DBE2EB956C142A338661D10461C0D135472" +
	"085057F3494309FFA73C611F78B32ADBB5740C361C9F35BE90997DB2014E2EF5" +
	"AA61782F52ABEB8BD6432C4DD097BC5423B285DAFB60DC364E8161F4A2A35ACA" +
	"3A10B1C4D203CC76A470A33AFDCBDD92959859ABD8B56E1725252D78EAC66E71" +
	"BA9AE3F1DD2487199874393CD4D832186800654760E1E34C09E4D155179F9EC0" +
	"DC4473F996BDCE6EED1CABED8B6F116F7AD9CF505DF0F998E34AB27514B0FFE7"

// This sets up a dummy/mock globals instance for testing purposes
func TestMain(m *testing.M) {
	jww.SetStdoutThreshold(jww.LevelTrace)

	// Begin gateway comms
	cmixNodes := make([]string, 1)
	cmixNodes[0] = GW_ADDRESS

	gatewayCert, _ = utils.ReadFile(testkeys.GetGatewayCertPath())
	gatewayKey, _ = utils.ReadFile(testkeys.GetGatewayKeyPath())

	gComm = gateway.StartGateway(&id.TempGateway, GW_ADDRESS,
		gatewayInstance, gatewayCert, gatewayKey, gossip.DefaultManagerFlags())

	// Start mock node
	nodeHandler := buildTestNodeImpl()

	nodeCert, _ = utils.ReadFile(testkeys.GetNodeCertPath())
	nodeKey, _ = utils.ReadFile(testkeys.GetNodeKeyPath())
	n = node.StartNode(id.NewIdFromString("node", id.Node, m), NODE_ADDRESS, 0, nodeHandler, nodeCert, nodeKey)

	// Build the gateway instance
	params := Params{
		NodeAddress:    NODE_ADDRESS,
		ServerCertPath: testkeys.GetNodeCertPath(),
		CertPath:       testkeys.GetGatewayCertPath(),
		KeyPath:        testkeys.GetGatewayKeyPath(),
		DevMode:        true,
	}

	params.rateLimitParams = &rateLimiting.MapParams{
		Capacity:     10,
		LeakedTokens: 1,
		LeakDuration: 10 * time.Second,
		PollDuration: 10 * time.Second,
		BucketMaxAge: 10 * time.Second,
	}

	gatewayInstance = NewGatewayInstance(params)
	gatewayInstance.Comms = gComm
	hostParams := connect.GetDefaultHostParams()
	hostParams.MaxRetries = 0
	hostParams.AuthEnabled = false
	gatewayInstance.ServerHost, _ = connect.NewHost(id.NewIdFromString("node", id.Node, m), NODE_ADDRESS,
		nodeCert, hostParams)

	p := large.NewIntFromString(prime, 16)
	g := large.NewIntFromString(generator, 16)
	grp2 := cyclic.NewGroup(p, g)

	testNDF, _ := ndf.Unmarshal(ExampleJSON)

	// This is bad. Itgrp2 needs to be fixed (Ben's fault for not fixing correctly)
	t := testing.T{}
	var err error
	gatewayInstance.NetInf, err = network.NewInstanceTesting(gatewayInstance.Comms.ProtoComms, testNDF, testNDF, grp2, grp2, &t)
	if err != nil {
		t.Errorf("NewInstanceTesting encountered an error: %+v", err)
	}

	// build a single mock message
	msg := format.NewMessage(grp2.GetP().ByteLen())

	payloadA := make([]byte, grp2.GetP().ByteLen())
	payloadA[0] = 1
	msg.SetPayloadA(payloadA)

	testEphId, _, _, err := ephemeral.GetId(id.NewIdFromUInt(1, id.User, m),
		64, time.Now().UnixNano())
	if err != nil {
		t.Errorf("Could not create an ephemeral id: %v", err)
	}

	msg.SetEphemeralRID(testEphId[:])

	mockMessage = &pb.Slot{
		Index:    42,
		PayloadA: msg.GetPayloadA(),
		PayloadB: msg.GetPayloadB(),
	}
	defer testWrapperShutdown()
	os.Exit(m.Run())
}

func testWrapperShutdown() {
	gComm.Shutdown()
	n.Shutdown()
}

func buildTestNodeImpl() *node.Implementation {
	nodeHandler := node.NewImplementation()
	nodeHandler.Functions.GetRoundBufferInfo = func(auth *connect.Auth) (
		int, error) {
		return 1, nil
	}
	nodeHandler.Functions.PostNewBatch = func(batch *pb.Batch,
		auth *connect.Auth) error {
		nodeIncomingBatch = batch
		return nil
	}
	nodeHandler.Functions.GetCompletedBatch = func(auth *connect.Auth) (
		*pb.Batch, error) {
		// build a batch
		b := pb.Batch{
			Round: &pb.RoundInfo{
				ID: 42, // meaning of life
			},
			FromPhase: 0,
			Slots: []*pb.Slot{
				mockMessage,
			},
		}

		return &b, nil
	}

	nodeHandler.Functions.Poll = func(p *pb.ServerPoll,
		auth *connect.Auth) (*pb.ServerPollResponse,
		error) {
		netDef := pb.ServerPollResponse{}
		return &netDef, nil
	}
	return nodeHandler
}

// Tests that receiving messages and sending them to the node works
func TestGatewayImpl_SendBatch(t *testing.T) {
	gatewayInstance.InitRateLimitGossip()
	defer gatewayInstance.KillRateLimiter()

	p := large.NewIntFromString(prime, 16)
	g := large.NewIntFromString(generator, 16)
	grp2 := cyclic.NewGroup(p, g)

	data := format.NewMessage(grp2.GetP().ByteLen())
	rndId := uint64(1)

	msg := pb.Slot{
		SenderID: id.NewIdFromUInt(666, id.User, t).Marshal(),
		PayloadA: data.GetPayloadA(),
		PayloadB: data.GetPayloadB(),
	}

	slotMsg := &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
		Target:  gatewayInstance.Comms.Id.Marshal(),
	}

	// Insert client information to database
	newClient := &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.storage.UpsertClient(newClient)
	pub := testkeys.LoadFromPath(testkeys.GetNodeCertPath())
	_, err := gatewayInstance.Comms.AddHost(&id.Permissioning,
		"0.0.0.0:4200", pub, connect.GetDefaultHostParams())

	ri := &pb.RoundInfo{ID: rndId, BatchSize: 4}
	gatewayInstance.UnmixedBuffer.SetAsRoundLeader(id.Round(rndId), ri.BatchSize)

	rnd := &storage.Round{
		Id:       rndId,
		UpdateId: 0,
	}
	gatewayInstance.storage.UpsertRound(rnd)
	_, err = gatewayInstance.PutMessage(slotMsg)
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages!"+
			"Error received: %v", err)
	}

	ri = &pb.RoundInfo{ID: 1, BatchSize: 4}
	gatewayInstance.SendBatch(ri)

	time.Sleep(1 * time.Second)

	if nodeIncomingBatch == nil {
		t.Errorf("Batch not received by node!")
	} else {
		if !reflect.DeepEqual(nodeIncomingBatch.Slots[0].SenderID,
			msg.SenderID) {
			t.Errorf("Message in batch not the same as sent;"+
				"\n  Expected: %+v \n  Received: %+v", msg,
				*nodeIncomingBatch.Slots[0])
		}
	}
}

func TestGatewayImpl_SendBatch_LargerBatchSize(t *testing.T) {
	// Begin gateway comms
	cmixNodes := make([]string, 1)
	cmixNodes[0] = GW_ADDRESS
	// Build the gateway instance
	params := Params{
		NodeAddress:    NODE_ADDRESS,
		ServerCertPath: testkeys.GetNodeCertPath(),
		CertPath:       testkeys.GetGatewayCertPath(),
		DevMode:        true,
	}

	params.rateLimitParams = &rateLimiting.MapParams{
		Capacity:     capacity,
		LeakedTokens: leakedTokens,
		LeakDuration: leakDuration,
		PollDuration: pollDuration,
		BucketMaxAge: bucketMaxAge,
	}

	gw := NewGatewayInstance(params)
	p := large.NewIntFromString(prime, 16)
	g := large.NewIntFromString(generator, 16)
	grp2 := cyclic.NewGroup(p, g)

	testNDF, _ := ndf.Unmarshal(ExampleJSON)

	var err error
	gw.NetInf, err = network.NewInstanceTesting(nil, testNDF, testNDF, grp2, grp2, t)
	if err != nil {
		t.Errorf("NewInstanceTesting encountered an error: %+v", err)
	}

	gw.Comms = gComm
	hostParams := connect.GetDefaultHostParams()
	hostParams.MaxRetries = 0
	hostParams.AuthEnabled = false
	gw.ServerHost, err = connect.NewHost(id.NewIdFromString("test", id.Node, t), NODE_ADDRESS,
		nodeCert, hostParams)
	if err != nil {
		t.Errorf(err.Error())
	}

	gw.InitRateLimitGossip()
	defer gw.KillRateLimiter()

	data := format.NewMessage(grp2.GetP().ByteLen())
	rndId := uint64(1)

	msg := pb.Slot{
		SenderID: id.NewIdFromUInt(666, id.User, t).Marshal(),
		PayloadA: data.GetPayloadA(),
		PayloadB: data.GetPayloadB(),
	}

	slotMsg := &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
		Target:  gw.Comms.Id.Marshal(),
	}

	// Insert client information to database
	newClient := &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gw.storage.UpsertClient(newClient)
	pub := testkeys.LoadFromPath(testkeys.GetNodeCertPath())
	_, err = gw.Comms.AddHost(&id.Permissioning,
		"0.0.0.0:4200", pub, connect.GetDefaultHostParams())

	ri := &pb.RoundInfo{ID: rndId, BatchSize: 4}
	gw.UnmixedBuffer.SetAsRoundLeader(id.Round(rndId), ri.BatchSize)

	rnd := &storage.Round{
		Id:       rndId,
		UpdateId: 0,
	}
	gw.storage.UpsertRound(rnd)
	_, err = gw.PutMessage(slotMsg)
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages!"+
			"Error received: %v", err)
	}

	si := &pb.RoundInfo{ID: 1, BatchSize: 4}
	gw.SendBatch(si)

}

func TestInstance_RequestMessages(t *testing.T) {
	// Create a message and insert them into a database
	numMessages := 5
	expectedRound := id.Round(1)
	recipientID := id.NewIdFromBytes([]byte("test"), t)
	testEphId, _, _, err := ephemeral.GetId(recipientID, 64, time.Now().UnixNano())
	if err != nil {
		t.Errorf("Could not create an ephemeral id: %v", err)
	}

	payload := "test"
	clientRound := &storage.ClientRound{
		Id:        uint64(expectedRound),
		Timestamp: time.Now(),
		Messages:  make([]storage.MixedMessage, numMessages),
	}
	for i := 0; i < numMessages; i++ {
		messageContents := []byte(payload)
		clientRound.Messages[i] = *storage.NewMixedMessage(expectedRound, testEphId, messageContents, messageContents)
	}
	err = gatewayInstance.storage.InsertMixedMessages(clientRound)
	if err != nil {
		t.Errorf(err.Error())
	}

	// Craft the request message and send
	requestMessage := &pb.GetMessages{
		ClientID: testEphId[:],
		RoundID:  uint64(1),
		Target:   gatewayInstance.Comms.Id.Marshal(),
	}

	receivedMsg, err := gatewayInstance.RequestMessages(requestMessage)
	if err != nil {
		t.Errorf("Unexpected in happy path: %v", err)
	}

	// Check that the amount of messages returned is expected
	if len(receivedMsg.Messages) != numMessages {
		t.Errorf("Messages returned is not expected."+
			"\n\tReceived: %d"+
			"\n\tExpected: %d", len(receivedMsg.Messages), numMessages)
	}

	// Check that the data within the messages is of expected value
	for i, msg := range receivedMsg.Messages {
		expectedMsg := []byte(payload)

		if !bytes.Contains(msg.PayloadA, expectedMsg) {
			t.Errorf("Received message %d did not contain expected PayloadA!"+
				"\n\tReceived: %v"+
				"\n\tExpected: %v", i, msg.PayloadA, expectedMsg)
		}

		if !bytes.Contains(msg.PayloadB, expectedMsg) {
			t.Errorf("Received message %d did not contain expected PayloadB!"+
				"\n\tReceived: %v"+
				"\n\tExpected: %v", i, msg.PayloadB, expectedMsg)
		}
	}
}

func TestInstance_RequestMessages_Proxy(t *testing.T) { // Create instances
	gw1 := makeGatewayInstance("0.0.0.0:5685", t)
	gw2 := makeGatewayInstance("0.0.0.0:5686", t)

	_, err := gw1.Comms.AddHost(gw2.Comms.Id, "0.0.0.0:5686", gatewayCert,
		connect.GetDefaultHostParams())
	if err != nil {
		t.Fatalf("Failed to add host: %+v", err)
	}

	// Create a message and insert them into a database
	numMessages := 5
	expectedRound := id.Round(1)
	recipientID := id.NewIdFromBytes([]byte("test"), t)
	testEphId, _, _, err := ephemeral.GetId(recipientID, 64, time.Now().UnixNano())
	if err != nil {
		t.Errorf("Could not create an ephemeral id: %v", err)
	}

	payload := "test"
	clientRound := &storage.ClientRound{
		Id:        uint64(expectedRound),
		Timestamp: time.Now(),
		Messages:  make([]storage.MixedMessage, numMessages),
	}
	for i := 0; i < numMessages; i++ {
		messageContents := []byte(payload)
		clientRound.Messages[i] = *storage.NewMixedMessage(expectedRound, testEphId, messageContents, messageContents)
	}
	err = gw1.storage.InsertMixedMessages(clientRound)
	if err != nil {
		t.Errorf(err.Error())
	}

	// Craft the request message and send
	requestMessage := &pb.GetMessages{
		ClientID: testEphId[:],
		RoundID:  uint64(1),
		Target:   gw2.Comms.Id.Marshal(),
	}

	receivedMsg, err := gw1.RequestMessages(requestMessage)
	if err != nil {
		t.Errorf("Unexpected in happy path: %v", err)
	}

	// Check that the amount of messages returned is expected
	if len(receivedMsg.Messages) != numMessages {
		t.Errorf("RequestMessages() did not return the expected number of messages."+
			"\nreceived: %d\nexpected: %d", len(receivedMsg.Messages), numMessages)
	}

	// Check that the data within the messages is of expected value
	for i, msg := range receivedMsg.Messages {
		expectedMsg := []byte(payload)

		if !bytes.Contains(msg.PayloadA, expectedMsg) {
			t.Errorf("Received message %d did not contain expected PayloadA!"+
				"\nreceived: %v\nexpected: %v", i, msg.PayloadA, expectedMsg)
		}

		if !bytes.Contains(msg.PayloadB, expectedMsg) {
			t.Errorf("Received message %d did not contain expected PayloadB!"+
				"\nreceived: %v\nexpected: %v", i, msg.PayloadB, expectedMsg)
		}
	}
}

// Error path: Request a round that exists in the database but the requested user
//  is not within this round
func TestInstance_RequestMessages_NoUser(t *testing.T) {
	// Create a message and insert them into a database
	numMessages := 5
	expectedRound := id.Round(0)
	recipientID := id.NewIdFromBytes([]byte("test"), t)
	testEphId, _, _, err := ephemeral.GetId(recipientID, 64, time.Now().UnixNano())
	if err != nil {
		t.Errorf("Could not create an ephemeral id: %v", err)
	}
	payload := "test"
	clientRound := &storage.ClientRound{
		Id:        uint64(expectedRound),
		Timestamp: time.Now(),
		Messages:  make([]storage.MixedMessage, numMessages),
	}
	for i := 0; i < numMessages; i++ {
		messageContents := []byte(payload)
		clientRound.Messages[i] = *storage.NewMixedMessage(expectedRound, testEphId, messageContents, messageContents)
	}
	err = gatewayInstance.storage.InsertMixedMessages(clientRound)
	if err != nil {
		t.Errorf(err.Error())
	}

	// Craft the request message with an unrecognized userID
	roundBytes := make([]byte, 8)
	badClientId := id.NewIdFromBytes([]byte("badClientId"), t)
	binary.BigEndian.PutUint64(roundBytes, uint64(expectedRound))
	badRequest := &pb.GetMessages{
		ClientID: badClientId.Bytes(),
		RoundID:  uint64(2),
		Target:   gatewayInstance.Comms.Id.Marshal(),
	}

	receivedMsg, err := gatewayInstance.RequestMessages(badRequest)
	if receivedMsg != nil && receivedMsg.HasRound {
		t.Errorf("msg.HasRound should be false")
	}

	if len(receivedMsg.Messages) != 0 {
		t.Errorf("Received messages should be zero!")
	}

}

// Error path: Request a round that doesn't exist in the database
func TestInstance_RequestMessages_NoRound(t *testing.T) {
	// Create a message and insert them into a database
	numMessages := 5
	expectedRound := id.Round(0)
	recipientID := id.NewIdFromBytes([]byte("test"), t)
	testEphId, _, _, err := ephemeral.GetId(recipientID, 64, time.Now().UnixNano())
	if err != nil {
		t.Errorf("Could not create an ephemeral id: %v", err)
	}
	payload := "test"
	clientRound := &storage.ClientRound{
		Id:        uint64(expectedRound),
		Timestamp: time.Now(),
		Messages:  make([]storage.MixedMessage, numMessages),
	}
	for i := 0; i < numMessages; i++ {
		messageContents := []byte(payload)
		clientRound.Messages[i] = *storage.NewMixedMessage(expectedRound, testEphId, messageContents, messageContents)
	}
	err = gatewayInstance.storage.InsertMixedMessages(clientRound)
	if err != nil {
		t.Errorf(err.Error())
	}

	// Craft the request message with an unknown round
	roundBytes := make([]byte, 8)
	badRoundId := id.Round(42)
	binary.BigEndian.PutUint64(roundBytes, uint64(badRoundId))
	badRequest := &pb.GetMessages{
		ClientID: recipientID.Bytes(),
		RoundID:  uint64(3),
		Target:   gatewayInstance.Comms.Id.Marshal(),
	}

	receivedMsg, err := gatewayInstance.RequestMessages(badRequest)
	if receivedMsg.HasRound {
		t.Errorf("msg.HasRound should be false")
	}

	if len(receivedMsg.Messages) != 0 {
		t.Errorf("Received messages should be zero!")
	}

}

// Error path: Craft a nil message which should not be accepted
func TestInstance_RequestMessages_NilCheck(t *testing.T) {
	// Create a message and insert them into a database
	numMessages := 5
	expectedRound := id.Round(0)
	recipientID := id.NewIdFromBytes([]byte("test"), t)
	testEphId, _, _, err := ephemeral.GetId(recipientID, 64, time.Now().UnixNano())
	if err != nil {
		t.Errorf("Could not create an ephemeral id: %v", err)
	}
	payload := "test"
	clientRound := &storage.ClientRound{
		Id:        uint64(expectedRound),
		Timestamp: time.Now(),
		Messages:  make([]storage.MixedMessage, numMessages),
	}
	for i := 0; i < numMessages; i++ {
		messageContents := []byte(payload)
		clientRound.Messages[i] = *storage.NewMixedMessage(expectedRound, testEphId, messageContents, messageContents)
	}
	err = gatewayInstance.storage.InsertMixedMessages(clientRound)
	if err != nil {
		t.Errorf(err.Error())
	}

	// Craft the request message with a nil ClientID
	badRequest := &pb.GetMessages{
		ClientID: recipientID.Bytes(),
		RoundID:  uint64(0),
	}

	receivedMsg, err := gatewayInstance.RequestMessages(badRequest)
	if err == nil {
		t.Errorf("Error path should not have a nil error. " +
			"Asking for a nil clientID should return an error")
	}

	if len(receivedMsg.Messages) != 0 {
		t.Errorf("Received messages should be zero!")
	}

	// Craft the request message with a nil RoundID
	badRequest = &pb.GetMessages{
		ClientID: recipientID.Bytes(),
		RoundID:  uint64(0),
	}

	receivedMsg, err = gatewayInstance.RequestMessages(badRequest)
	if err == nil {
		t.Errorf("Error path should not have a nil error. " +
			"Asking for a nil clientID should return an error")
	}

	if len(receivedMsg.Messages) != 0 {
		t.Errorf("Received messages should be zero!")
	}

}

// Tests that messages can get through when its user ID bucket is not full and
// checks that they are blocked when the bucket is full.
// TODO: re-enable after user ID limiting is working
/*func TestGatewayImpl_PutMessage_UserBlock(t *testing.T) {
	errMsg := ("PutMessage: Could not put any messages user ID " +
		"should not be blocked")
	msg := pb.Slot{SenderID: id.NewUserFromUint(12, t).Bytes()}
	ok := gatewayInstance.PutMessage(&msg, "12")
	if !ok {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(234, t).Bytes()}
	ok = gatewayInstance.PutMessage(&msg, "2")
	if !ok {
		t.Errorf(errMsg)
	}

	ok = gatewayInstance.PutMessage(&msg, "2")
	if !ok {
		t.Errorf(errMsg)
	}

	ok = gatewayInstance.PutMessage(&msg, "3")
	if ok {
		t.Errorf("PutMessage: Put message when it should have" +
" been blocked based on user ID")
	}

	time.Sleep(1 * time.Second)

	ok = gatewayInstance.PutMessage(&msg, "4")
	if !ok {
		t.Errorf(errMsg)
	}

	ok = gatewayInstance.PutMessage(&msg, "4")
	if !ok {
		t.Errorf(errMsg)
	}

	ok = gatewayInstance.PutMessage(&msg, "5")
	if ok {
		t.Errorf("PutMessage: Put message when it should have" +
" been blocked based on user ID")
	}
}*/

// Tests that messages can get through even when their bucket is full.
func TestGatewayImpl_PutMessage_IpWhitelist(t *testing.T) {
	var msg pb.Slot
	var err error
	rndId := uint64(2)

	msg = pb.Slot{SenderID: id.NewIdFromUInt(128, id.User, t).Marshal()}
	slotMsg := &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
		Target:  gatewayInstance.Comms.Id.Marshal(),
	}

	// Insert client information to database
	newClient := &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}
	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.storage.UpsertClient(newClient)
	ri := &pb.RoundInfo{ID: rndId, BatchSize: 24}
	gatewayInstance.UnmixedBuffer.SetAsRoundLeader(id.Round(rndId), ri.BatchSize)

	_, err = gatewayInstance.PutMessage(slotMsg)
	errMsg := "PutMessage: Could not put any messages when IP " +
		"address should not be blocked"
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(129, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
		Target:  gatewayInstance.Comms.Id.Marshal(),
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.storage.UpsertClient(newClient)
	_, err = gatewayInstance.PutMessage(slotMsg)
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(130, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
		Target:  gatewayInstance.Comms.Id.Marshal(),
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.storage.UpsertClient(newClient)
	_, err = gatewayInstance.PutMessage(slotMsg)
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(131, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
		Target:  gatewayInstance.Comms.Id.Marshal(),
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.storage.UpsertClient(newClient)
	_, err = gatewayInstance.PutMessage(slotMsg)
	if err != nil {
		t.Errorf(errMsg)
	}

	time.Sleep(1 * time.Second)

	msg = pb.Slot{SenderID: id.NewIdFromUInt(132, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
		Target:  gatewayInstance.Comms.Id.Marshal(),
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.storage.UpsertClient(newClient)
	_, err = gatewayInstance.PutMessage(slotMsg)
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages when " +
			"IP bucket is full but message IP is on whitelist")
	}
}

// Tests the proxy path.
func TestGatewayImpl_PutMessage_Proxy(t *testing.T) {
	// Create instances
	gw1 := makeGatewayInstance("0.0.0.0:5679", t)
	gw2 := makeGatewayInstance("0.0.0.0:5680", t)

	_, err := gw1.Comms.AddHost(gw2.Comms.Id, "0.0.0.0:5680", gatewayCert,
		connect.GetDefaultHostParams())
	if err != nil {
		t.Fatalf("Failed to add host: %+v", err)
	}

	rndId := uint64(20)

	msg := pb.Slot{SenderID: id.NewIdFromUInt(128, id.User, t).Marshal()}
	slotMsg := &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
		Target:  gw2.Comms.Id.Marshal(),
	}

	// Insert client information to database
	newClient := &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}
	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	err = gatewayInstance.storage.UpsertClient(newClient)
	if err != nil {
		t.Errorf("Failed to upsert client: %+v", err)
	}
	ri := &pb.RoundInfo{ID: rndId, BatchSize: 24}
	gatewayInstance.UnmixedBuffer.SetAsRoundLeader(id.Round(rndId), ri.BatchSize)

	receivedMsg, err := gatewayInstance.PutMessage(slotMsg)
	if err != nil {
		t.Errorf("PutMessage returned an error: %+v", err)
	}

	expectedMsg := &pb.GatewaySlotResponse{
		Accepted: true,
		RoundID:  rndId,
	}

	if !reflect.DeepEqual(expectedMsg, receivedMsg) {
		t.Errorf("PutMessage did not return the expected response."+
			"\nexpected: %+v\nreceived: %+v", expectedMsg, receivedMsg)
	}

}

// Error path: Test that a message is denied when the batch is full
func TestInstance_PutMessage_FullRound(t *testing.T) {
	// Business logic to set up test
	var msg pb.Slot
	rndId := uint64(3)
	batchSize := 4
	msg = pb.Slot{SenderID: id.NewIdFromUInt(128, id.User, t).Marshal()}
	slotMsg := &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
		Target:  gatewayInstance.Comms.Id.Marshal(),
	}

	// Insert client information to database
	newClient := &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}
	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.storage.UpsertClient(newClient)

	// End of business logic

	// Mark this as a round in which the gateway is the leader
	ri := &pb.RoundInfo{ID: rndId, BatchSize: uint32(batchSize)}
	gatewayInstance.UnmixedBuffer.SetAsRoundLeader(id.Round(rndId), ri.BatchSize)

	// Put a message in the same round to fill up the batch size
	for i := 0; i < batchSize; i++ {
		_, err := gatewayInstance.PutMessage(slotMsg)
		if err != nil {
			t.Errorf("Failed to put message number %d into gateway's buffer: %v", i, err)
		}
	}

	_, err := gatewayInstance.PutMessage(slotMsg)
	if err == nil {
		t.Errorf("Expected error path. Should not be able to put a message into a full round!")

	}
}

// Error path: Test that when the gateway is not the entry point for the round
// that the message is rejected
func TestInstance_PutMessage_NonLeader(t *testing.T) {
	// Business logic to set up test
	var msg pb.Slot
	rndId := uint64(4)
	msg = pb.Slot{SenderID: id.NewIdFromUInt(128, id.User, t).Marshal()}
	slotMsg := &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
		Target:  gatewayInstance.Comms.Id.Marshal(),
	}

	// Insert client information to database
	newClient := &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}
	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.storage.UpsertClient(newClient)

	// End of business logic

	// Commented out explicitly. For this test we do NOT want the
	// gateway to be the leader for this round
	// ri := &pb.RoundInfo{ID:(rndId), BatchSize:uint32(batchSize)}
	// gatewayInstance.UnmixedBuffer.SetAsRoundLeader(id.Round(rndId), ri.BatchSize)

	_, err := gatewayInstance.PutMessage(slotMsg)
	if err == nil {
		t.Errorf("Expected error path. Should not be able to put a message into a round when not the leader!")

	}
}

// Tests that messages can get through even when their bucket is full.
func TestGatewayImpl_PutMessage_UserWhitelist(t *testing.T) {
	var msg pb.Slot
	var err error
	rndId := uint64(5)
	msg = pb.Slot{SenderID: id.NewIdFromUInt(174, id.User, t).Marshal()}
	slotMsg := &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
		Target:  gatewayInstance.Comms.Id.Marshal(),
	}

	ri := &pb.RoundInfo{ID: rndId, BatchSize: 24}
	gatewayInstance.UnmixedBuffer.SetAsRoundLeader(id.Round(rndId), ri.BatchSize)

	// Insert client information to database
	newClient := &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.storage.UpsertClient(newClient)

	_, err = gatewayInstance.PutMessage(slotMsg)
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages when " +
			"IP address should not be blocked")
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(174, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
		Target:  gatewayInstance.Comms.Id.Marshal(),
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.storage.UpsertClient(newClient)
	_, err = gatewayInstance.PutMessage(slotMsg)
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages when " +
			"IP address should not be blocked")
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(174, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
		Target:  gatewayInstance.Comms.Id.Marshal(),
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.storage.UpsertClient(newClient)
	_, err = gatewayInstance.PutMessage(slotMsg)
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages when user " +
			"ID bucket is full but user ID is on whitelist")
	}
}

// Tests the proxy path.
func TestGatewayImpl_RequestNonce_Proxy(t *testing.T) {
	// Create instances
	gw1 := makeGatewayInstance("0.0.0.0:5681", t)
	gw2 := makeGatewayInstance("0.0.0.0:5682", t)

	_, err := gw1.Comms.AddHost(gw2.Comms.Id, "0.0.0.0:5682", gatewayCert,
		connect.GetDefaultHostParams())
	if err != nil {
		t.Fatalf("Failed to add host: %+v", err)
	}

	msg := &pb.NonceRequest{
		Target: gw2.Comms.Id.Marshal(),
	}

	_, err = gatewayInstance.RequestNonce(msg)
	if err != nil {
		t.Errorf("RequestNonce returned an error: %+v", err)
	}
}

// Tests the proxy path.
func TestGatewayImpl_ConfirmNonce_Proxy(t *testing.T) {
	// Create instances
	gw1 := makeGatewayInstance("0.0.0.0:5683", t)
	gw2 := makeGatewayInstance("0.0.0.0:5684", t)

	_, err := gw1.Comms.AddHost(gw2.Comms.Id, "0.0.0.0:5684", gatewayCert,
		connect.GetDefaultHostParams())
	if err != nil {
		t.Fatalf("Failed to add host: %+v", err)
	}

	msg := &pb.RequestRegistrationConfirmation{
		UserID: id.NewIdFromString("user ID", id.User, t).Marshal(),
		Target: gw1.Comms.Id.Marshal(),
	}

	_, err = gatewayInstance.ConfirmNonce(msg)
	if err != nil {
		t.Errorf("ConfirmNonce returned an error: %+v", err)
	}
}

func buildMockNdf(nodeId *id.ID, nodeAddress, gwAddress string, cert,
	key []byte) *ndf.NetworkDefinition {
	node := ndf.Node{
		ID:             nodeId.Bytes(),
		TlsCertificate: string(cert),
		Address:        nodeAddress,
	}
	gw := ndf.Gateway{
		Address:        gwAddress,
		TlsCertificate: string(cert),
	}
	testNdf := &ndf.NetworkDefinition{
		Timestamp: time.Now(),
		Nodes:     []ndf.Node{node},
		Gateways:  []ndf.Gateway{gw},
		E2E: ndf.Group{
			Prime:      "123",
			SmallPrime: "456",
			Generator:  "2",
		},
		CMIX: ndf.Group{
			Prime:      "123",
			SmallPrime: "456",
			Generator:  "2",
		},
		UDB: ndf.UDB{},
	}
	return testNdf
}

// TestCreateNetworkInstance tests that, without an NDF, we can
// build a netinf object in the instance object
func TestCreateNetworkInstance(t *testing.T) {
	pub := testkeys.LoadFromPath(testkeys.GetNodeCertPath())
	_, err := gatewayInstance.Comms.AddHost(&id.Permissioning,
		"0.0.0.0:4200", pub, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Failed to add permissioning host: %+v", err)
	}

	nodeB := []byte{'n', 'o', 'd', 'e'}
	nodeId := id.NewIdFromBytes(nodeB, t)
	testNdf := buildMockNdf(nodeId, NODE_ADDRESS, GW_ADDRESS, nodeCert, nodeKey)

	ndfBytes, err := testNdf.Marshal()
	if err != nil {
		t.Errorf("%v", err)
	}
	ndfMsg := &pb.NDF{
		Ndf: ndfBytes,
	}
	pKey, err := rsa.LoadPrivateKeyFromPem(nodeKey)
	if err != nil {
		t.Errorf("%v", err)
	}
	err = signature.SignRsa(ndfMsg, pKey)
	if err != nil {
		t.Errorf("%v", err)
	}

	netInst, err := CreateNetworkInstance(
		gatewayInstance.Comms, ndfMsg, ndfMsg, &storage.Storage{})

	gatewayInstance.NetInf = netInst
	if err != nil {
		t.Errorf("%v", err)
	}
	if netInst == nil {
		t.Errorf("Could not create network instance!")
	}

}

// Tests happy path of Instance.SaveKnownRounds and Instance.LoadKnownRounds.
func TestInstance_SaveKnownRounds_LoadKnownRounds(t *testing.T) {
	// Build the gateway instance
	params := Params{DevMode: true}

	// Create new gateway instance and modify knownRounds
	gw := NewGatewayInstance(params)
	_ = gw.InitNetwork()
	gw.knownRound.Check(4)
	expectedData := gw.knownRound.Marshal()

	// Attempt to save knownRounds to file
	if err := gw.SaveKnownRounds(); err != nil {
		t.Errorf("SaveKnownRounds() produced an error: %v", err)
	}

	// Attempt to load knownRounds from file
	if err := gw.LoadKnownRounds(); err != nil {
		t.Errorf("LoadKnownRounds() produced an error: %v", err)
	}

	// Ensure that the data loaded from file matches the expected data
	testData := gw.knownRound.Marshal()
	if !reflect.DeepEqual(expectedData, testData) {
		t.Errorf("Failed to load correct KnownRounds."+
			"\n\texpected: %s\n\treceived: %s", expectedData, testData)
	}
}

// Tests that Instance.LoadKnownRounds returns nil if the file does not exist.
func TestInstance_LoadKnownRounds_UnmarshalError(t *testing.T) {
	// Build the gateway instance
	params := Params{DevMode: true}

	// Create new gateway instance and modify knownRounds
	gw := NewGatewayInstance(params)
	gw.knownRound.Check(67)

	if err := gw.SaveKnownRounds(); err != nil {
		t.Fatalf("SaveKnownRounds() produced an error: %v", err)
	}

	gw.knownRound = knownRounds.NewKnownRound(1)

	err := gw.LoadKnownRounds()
	if err == nil {
		t.Error("LoadKnownRounds() did not return an error when unmarshalling " +
			"should have failed.")
	}
}

// Tests happy path of Instance.SaveLastUpdateID and Instance.LoadLastUpdateID.
func TestInstance_SaveLastUpdateID_LoadLastUpdateID(t *testing.T) {
	// Build the gateway instance

	params := Params{DevMode: true}

	// Create new gateway instance and modify lastUpdate
	gw := NewGatewayInstance(params)
	expectedLastUpdate := rand.Uint64()
	gw.lastUpdate = expectedLastUpdate

	err := gw.SaveLastUpdateID()
	if err != nil {
		t.Errorf("SaveLastUpdateID() produced an error: %v", err)
	}

	gw.lastUpdate = rand.Uint64()

	err = gw.LoadLastUpdateID()
	if err != nil {
		t.Errorf("LoadLastUpdateID() produced an error: %+v", err)
	}

	if expectedLastUpdate != gw.lastUpdate {
		t.Errorf("Failed to save/load lastUpdate correctly."+
			"\n\texpected: %d\n\treceived: %d",
			expectedLastUpdate, gw.lastUpdate)
	}

	err = gw.LoadLastUpdateID()
	if err != nil {
		t.Errorf("LoadLastUpdateID() produced an error: %v", err)
	}

	if expectedLastUpdate != gw.lastUpdate {
		t.Errorf("Failed to save/load lastUpdate correctly."+
			"\n\texpected: %d\n\treceived: %d",
			expectedLastUpdate, gw.lastUpdate)
	}
}

func TestInstance_ClearOldStorage(t *testing.T) {
	gw := NewGatewayInstance(Params{
		cleanupInterval: 250 * time.Millisecond,
		retentionPeriod: retentionPeriodDefault,
		DevMode:         true,
	})

	gw.period = 7

	oldTimestamp := time.Now().Add(-5 * retentionPeriodDefault)
	rndId := uint64(1)

	testId := id.NewIdFromBytes([]byte("Frodo"), t)
	recipientId, _, _, err := ephemeral.GetId(testId, 64, 300000)
	if err != nil {
		t.Errorf("Could not make a mock ephemeral Id: %v", err)
	}
	msg := storage.NewMixedMessage(id.Round(rndId), recipientId, []byte("test"), []byte("message"))
	// Build a ClientRound object around the client messages
	clientRound := &storage.ClientRound{
		Id:        rndId,
		Timestamp: oldTimestamp,
		Messages:  []storage.MixedMessage{*msg},
	}

	err = gw.storage.InsertMixedMessages(clientRound)
	if err != nil {
		t.Errorf("Could not insert mock message")
	}

	var wg sync.WaitGroup
	go func() {
		wg.Add(1)

		gw.clearOldStorage(time.Now().Add(-retentionPeriodDefault))

		wg.Done()
	}()

	wg.Wait()
	time.Sleep(2 * time.Second)

	// Test that messages were cleared
	// NOTE: Rounds not tested as insertion explicitly sets
	// insert time to time.Now()
	msgs, _, err := gw.storage.GetMixedMessages(recipientId, id.Round(rndId))
	if len(msgs) != 0 {
		t.Errorf("Message expected to be cleared after clearOldStorage")
	}

}

// Happy path
// Can't test panic paths, obviously
func TestGetEpoch(t *testing.T) {
	ts := int64(300000)
	period := int64(5000)
	expected := uint32(60)
	result := GetEpoch(ts, period)
	if result != expected {
		t.Errorf("Invalid GetEpoch result: Got %d Expected %d", result, expected)
	}
}

// Various happy paths
func TestGetEpochTimestamp(t *testing.T) {
	epoch := uint32(60)
	period := int64(5000)
	expected := int64(300000)
	result := GetEpochTimestamp(epoch, period)
	if result != expected {
		t.Errorf("Invalid GetEpochTimestamp result: Got %d Expected %d", result, expected)
	}

	period = 0
	expected = 0
	result = GetEpochTimestamp(epoch, period)
	if result != expected {
		t.Errorf("Invalid GetEpochTimestamp result: Got %d Expected %d", result, expected)
	}

	period = -5000
	expected = -300000
	result = GetEpochTimestamp(epoch, period)
	if result != expected {
		t.Errorf("Invalid GetEpochTimestamp result: Got %d Expected %d", result, expected)
	}
}

// Happy path
func TestInstance_SetPeriod(t *testing.T) {
	gw := NewGatewayInstance(Params{DevMode: true})
	err := gw.SetPeriod()
	if err != nil {
		t.Errorf("Unable to set period: %+v", err)
	}
	if gw.period != period {
		t.Errorf("Period set incorrectly, got %d", gw.period)
	}
}

// Handle existing period in storage path
func TestInstance_SetPeriodExisting(t *testing.T) {
	gw := NewGatewayInstance(Params{DevMode: true})
	expected := int64(50)

	err := gw.storage.UpsertState(&storage.State{
		Key:   storage.PeriodKey,
		Value: strconv.FormatInt(expected, 10),
	})
	if err != nil {
		t.Errorf("Unable to pre-set period: %+v", err)
	}

	err = gw.SetPeriod()
	if err != nil {
		t.Errorf("Unable to set period: %+v", err)
	}
	if gw.period != expected {
		t.Errorf("Period set incorrectly, got %d", gw.period)
	}
}

func TestInstance_shareMessages(t *testing.T) {
	var err error
	//Build the gateway instance
	params := Params{
		NodeAddress:           NODE_ADDRESS,
		ServerCertPath:        testkeys.GetNodeCertPath(),
		CertPath:              testkeys.GetGatewayCertPath(),
		KeyPath:               testkeys.GetGatewayKeyPath(),
		PermissioningCertPath: testkeys.GetNodeCertPath(),
		DevMode:               true,
	}

	gw := NewGatewayInstance(params)
	gw2 := NewGatewayInstance(params)

	p := large.NewIntFromString(prime, 16)
	g := large.NewIntFromString(generator, 16)
	grp2 := cyclic.NewGroup(p, g)
	addr := "0.0.0.0:1234"
	gw.Comms = gateway.StartGateway(&id.TempGateway, addr, gw,
		gatewayCert, gatewayKey, gossip.DefaultManagerFlags())

	addr2 := "0.0.0.0:5678"
	gw2.Comms = gateway.StartGateway(&id.TempGateway, addr2, gw2,
		gatewayCert, gatewayKey, gossip.DefaultManagerFlags())

	testNDF, _ := ndf.Unmarshal(ExampleJSON)

	gw.NetInf, err = network.NewInstanceTesting(gw.Comms.ProtoComms, testNDF, testNDF, grp2, grp2, t)
	if err != nil {
		t.Errorf("NewInstanceTesting encountered an error: %+v", err)
	}

	gw2.NetInf, err = network.NewInstanceTesting(gw.Comms.ProtoComms, testNDF, testNDF, grp2, grp2, t)
	if err != nil {
		t.Errorf("NewInstanceTesting encountered an error: %+v", err)
	}

	// Add permissioning as a host
	pub := testkeys.LoadFromPath(testkeys.GetNodeCertPath())
	_, err = gw.Comms.AddHost(&id.Permissioning,
		"0.0.0.0:4200", pub, connect.GetDefaultHostParams())

	// Init comms and host
	_, err = gw.Comms.AddHost(gw.Comms.Id, addr, gatewayCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Unable to add test host: %+v", err)
	}
	_, err = gw.Comms.AddHost(gw2.Comms.Id, addr, gatewayCert, connect.GetDefaultHostParams())
	if err != nil {
		t.Errorf("Unable to add test host: %+v", err)
	}

	// Build a mock node ID for a topology
	nodeID := gw.Comms.Id.DeepCopy()
	nodeID.SetType(id.Node)
	nodeId2 := gw2.Comms.Id.DeepCopy()
	nodeId2.SetType(id.Node)
	topology := [][]byte{nodeID.Bytes(), nodeId2.Bytes()}
	// Create a fake round info to store
	roundId := uint64(10)
	ri := &pb.RoundInfo{
		ID:         roundId,
		UpdateID:   10,
		Topology:   topology,
		Timestamps: make([]uint64, states.NUM_STATES),
	}

	// Sign the round info with the mock permissioning private key
	err = testutils.SignRoundInfoRsa(ri, t)
	if err != nil {
		t.Errorf("Error signing round info: %s", err)
	}

	// Insert the mock round into the network instance
	err = gw.NetInf.RoundUpdate(ri)
	if err != nil {
		t.Errorf("Could not place mock round: %v", err)
	}
	err = gw2.NetInf.RoundUpdate(ri)
	if err != nil {
		t.Errorf("Could not place mock round: %v", err)
	}

	// build a single mock message
	data := format.NewMessage(grp2.GetP().ByteLen())
	senderId := id.NewIdFromUInt(666, id.User, t).Marshal()
	msg := &pb.Slot{
		SenderID: senderId,
		PayloadA: data.GetPayloadA(),
		PayloadB: data.GetPayloadB(),
	}

	err = gw.sendShareMessages([]*pb.Slot{msg}, ri)
	if err != nil {
		t.Errorf("Error with send share message: %v", err)
	}

}

// TODO: Readd test
//func TestInstance_ShareMessages(t *testing.T) {
//	var err error
//	//Build the gateway instance
//	params := Params{
//		NodeAddress:           NODE_ADDRESS,
//		ServerCertPath:        testkeys.GetNodeCertPath(),
//		CertPath:              testkeys.GetGatewayCertPath(),
//		KeyPath:               testkeys.GetGatewayKeyPath(),
//		PermissioningCertPath: testkeys.GetNodeCertPath(),
//	}
//
//	gw := NewGatewayInstance(params)
//	gw2 := NewGatewayInstance(params)
//
//	p := large.NewIntFromString(prime, 16)
//	g := large.NewIntFromString(generator, 16)
//	grp2 := cyclic.NewGroup(p, g)
//	addr := "0.0.0.0:3333"
//	gw.Comms = gateway.StartGateway(&id.TempGateway, addr, gw,
//		gatewayCert, gatewayKey, gossip.DefaultManagerFlags())
//
//	addr2 := "0.0.0.0:4444"
//	gw2Id := id.NewIdFromBytes([]byte("gw2"), t)
//	gw2.Comms = gateway.StartGateway(gw2Id, addr2, gw2,
//		gatewayCert, gatewayKey, gossip.DefaultManagerFlags())
//
//	testNDF, _ := ndf.Unmarshal(ExampleJSON)
//
//	gw.NetInf, err = network.NewInstanceTesting(gw.Comms.ProtoComms, testNDF, testNDF, grp2, grp2, t)
//	if err != nil {
//		t.Errorf("NewInstanceTesting encountered an error: %+v", err)
//	}
//
//	gw2.NetInf, err = network.NewInstanceTesting(gw.Comms.ProtoComms, testNDF, testNDF, grp2, grp2, t)
//	if err != nil {
//		t.Errorf("NewInstanceTesting encountered an error: %+v", err)
//	}
//
//
//	// Add permissioning as a host
//	pub := testkeys.LoadFromPath(testkeys.GetNodeCertPath())
//	_, err = gw.Comms.AddHost(&id.Permissioning,
//		"0.0.0.0:4200", pub, connect.GetDefaultHostParams())
//
//	// Init comms and host
//	_, err = gw.Comms.AddHost(gw.Comms.Id, addr, gatewayCert, connect.GetDefaultHostParams())
//	if err != nil {
//		t.Errorf("Unable to add test host: %+v", err)
//	}
//	h, err := gw.Comms.AddHost(gw2.Comms.Id, addr, gatewayCert, connect.GetDefaultHostParams())
//	if err != nil {
//		t.Errorf("Unable to add test host: %+v", err)
//	}
//
//	// Build a mock node ID for a topology
//	nodeID := gw.Comms.Id.DeepCopy()
//	nodeID.SetType(id.Node)
//	nodeId2 := gw2.Comms.Id.DeepCopy()
//	nodeId2.SetType(id.Node)
//	topology := [][]byte{nodeID.Bytes(), nodeId2.Bytes()}
//	// Create a fake round info to store
//	roundId := uint64(10)
//	ri := &pb.RoundInfo{
//		ID:         roundId,
//		UpdateID:   10,
//		Topology:   topology,
//		Timestamps: make([]uint64, states.NUM_STATES),
//	}
//
//	// Sign the round info with the mock permissioning private key
//	err = signRoundInfo(ri)
//	if err != nil {
//		t.Errorf("Error signing round info: %s", err)
//	}
//
//	// Insert the mock round into the network instance
//	err = gw.NetInf.RoundUpdate(ri)
//	if err != nil {
//		t.Errorf("Could not place mock round: %v", err)
//	}
//	err = gw2.NetInf.RoundUpdate(ri)
//	if err != nil {
//		t.Errorf("Could not place mock round: %v", err)
//	}
//
//
//	// build a single mock message
//	data := format.NewMessage(grp2.GetP().ByteLen())
//	data.SetIdentityFP([]byte("I am the length of a FP!!"))
//	senderId := id.NewIdFromUInt(666, id.User, t).Marshal()
//	msg := &pb.Slot{
//		SenderID: senderId,
//		PayloadA: data.GetPayloadA(),
//		PayloadB: data.GetPayloadB(),
//	}
//
//	roundMessage := &pb.RoundMessages{
//		RoundId:  roundId,
//		Messages: []*pb.Slot{msg},
//	}
//
//	auth := &connect.Auth{
//		IsAuthenticated: true,
//		Sender:          h,
//	}
//
//	err = gw.ShareMessages(roundMessage, auth)
//	if err != nil {
//		t.Errorf("Failed happy path: %v", err)
//	}
//
//	// Pull message from storage
//	serialMsg := format.NewMessage(grp2.GetP().ByteLen())
//	recipIdBytes := serialMsg.GetEphemeralRID()
//	recipientId, err := ephemeral.Marshal(recipIdBytes)
//	if err != nil {
//		t.Errorf("Unable to marshal ID: %+v", err)
//	}
//
//	retrieved, _, err := gw.storage.GetMixedMessages(recipientId, id.Round(roundId))
//
//	if len(retrieved) == 0 {
//		t.Errorf("Message from storage should not be empty")
//	}
//}

// TestUpdateInstance tests that the instance updates itself appropriately
// FIXME: This test cannot test the Ndf functionality, since we don't have
//        signable ndf function that would enforce correctness, so not useful
//        at the moment.
// FIXME: the following will fail with a nil pointer deref if the
//        CreateNetworkInstance test doesn't run....
// func TestUpdateInstance(t *testing.T) {
//	nodeId := id.NewIdFromString("node", id.Node, t)
//	testNdf := buildMockNdf(nodeId, NODE_ADDRESS, GW_ADDRESS, nodeCert, nodeKey)
//
//	ndfBytes, err := testNdf.Marshal()
//	if err != nil {
//		t.Errorf("%v", err)
//	}
//	ndfMsg := &pb.NDF{
//		Ndf: ndfBytes,
//	}
//	pKey, err := rsa.LoadPrivateKeyFromPem(nodeKey)
//	if err != nil {
//		t.Errorf("%v", err)
//	}
//	err = signature.Sign(ndfMsg, pKey)
//	if err != nil {
//		t.Errorf("%v", err)
//	}
//
//	// FIXME: the following will fail with a nil pointer deref if the
//	//        CreateNetworkInstance test doesn't run....
//	ri := &pb.RoundInfo{
//		ID:        uint64(1),
//		UpdateID:  uint64(1),
//		State:     6,
//		BatchSize: 8,
//	}
//	err = signature.Sign(ri, pKey)
//	roundUpdates := []*pb.RoundInfo{ri}
//	data := format.NewMessage()
//	msg := &pb.Slot{
//		SenderID: id.NewIdFromUInt(666, id.User, t).Marshal(),
//		PayloadA: data.GetPayloadA(),
//		PayloadB: data.GetPayloadB(),
//	}
//
//	serialmsg := format.NewMessage()
//	serialmsg.SetPayloadB(msg.PayloadB)
//	userId, err := serialmsg.GetRecipient()
//
//	slots := []*pb.Slot{msg}
//
//	update := &pb.ServerPollResponse{
//		FullNDF:      ndfMsg,
//		PartialNDF:   ndfMsg,
//		Updates:      roundUpdates,
//		BatchRequest: ri,
//		Slots:        slots,
//	}
//
//	err = gatewayInstance.UpdateInstance(update)
//	if err != nil {
//		t.Errorf("UpdateInstance() produced an error: %+v", err)
//	}
//
//	// Check that updates made it
//	r, err := gatewayInstance.NetInf.GetRoundUpdate(1)
//	if err != nil || r == nil {
//		t.Errorf("Failed to retrieve round update: %+v", err)
//	}
//
//	// Check that mockMessage made it
//	mockMsgUserId := id.NewIdFromUInt(1, id.User, t)
//	msgTst, err := gatewayInstance.database.GetMixedMessages(userId, id.Round(1))
//	if err != nil {
//		t.Errorf("%+v", err)
//		msgIDs, _ := gatewayInstance.database.GetMixedMessages(
//			mockMsgUserId, id.Round(1))
//		for i := 0; i < len(msgIDs); i++ {
//			print("%s", msgIDs[i])
//		}
//	}
//	if msgTst == nil {
//		t.Errorf("Did not return mock message!")
//	}
//
//	// Check that batchRequest was sent
//	if len(nodeIncomingBatch.Slots) != 8 {
//		t.Errorf("Did not send batch: %d", len(nodeIncomingBatch.Slots))
//	}
//
// }

func makeGatewayInstance(addr string, t *testing.T) *Instance {
	var err error

	// Build the gateway instance
	params := Params{
		NodeAddress:           NODE_ADDRESS,
		ServerCertPath:        testkeys.GetNodeCertPath(),
		CertPath:              testkeys.GetGatewayCertPath(),
		KeyPath:               testkeys.GetGatewayKeyPath(),
		PermissioningCertPath: testkeys.GetNodeCertPath(),
		DevMode:               true,
	}

	gw := NewGatewayInstance(params)
	grp := cyclic.NewGroup(large.NewIntFromString(prime, 16),
		large.NewIntFromString(generator, 16))
	testNDF, _ := ndf.Unmarshal(ExampleJSON)

	gw.Comms = gateway.StartGateway(&id.TempGateway, addr, gw, gatewayCert,
		gatewayKey, gossip.DefaultManagerFlags())

	gw.NetInf, err = network.NewInstanceTesting(gw.Comms.ProtoComms, testNDF,
		testNDF, grp, grp, t)
	if err != nil {
		t.Fatalf("Failed to create new test Instance: %+v", err)
	}

	nodeCert, _ := utils.ReadFile(testkeys.GetNodeCertPath())
	hostParams := connect.GetDefaultHostParams()
	hostParams.MaxRetries = 0
	hostParams.AuthEnabled = false
	gw.ServerHost, _ = connect.NewHost(id.NewIdFromString("node", id.Node, t),
		NODE_ADDRESS, nodeCert, hostParams)

	return gw
}

var (
	ExampleJSON = []byte(`{
	"Timestamp": "2019-06-04T20:48:48-07:00",
	"gateways": [
		{
			"Id": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1],
			"Address": "2.5.3.122",
			"Tls_certificate": "-----BEGIN CERTIFICATE-----\nMIIDgTCCAmmgAwIBAgIJAKLdZ8UigIAeMA0GCSqGSIb3DQEBBQUAMG8xCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlDbGFyZW1vbnQx\nGzAZBgNVBAoMElByaXZhdGVncml0eSBDb3JwLjEaMBgGA1UEAwwRZ2F0ZXdheSou\nY21peC5yaXAwHhcNMTkwMzA1MTgzNTU0WhcNMjkwMzAyMTgzNTU0WjBvMQswCQYD\nVQQGEwJVUzETMBEGA1UECAwKQ2FsaWZvcm5pYTESMBAGA1UEBwwJQ2xhcmVtb250\nMRswGQYDVQQKDBJQcml2YXRlZ3JpdHkgQ29ycC4xGjAYBgNVBAMMEWdhdGV3YXkq\nLmNtaXgucmlwMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA9+AaxwDP\nxHbhLmn4HoZu0oUM48Qufc6T5XEZTrpMrqJAouXk+61Jc0EFH96/sbj7VyvnXPRo\ngIENbk2Y84BkB9SkRMIXya/gh9dOEDSgnvj/yg24l3bdKFqBMKiFg00PYB30fU+A\nbe3OI/le0I+v++RwH2AV0BMq+T6PcAGjCC1Q1ZB0wP9/VqNMWq5lbK9wD46IQiSi\n+SgIQeE7HoiAZXrGO0Y7l9P3+VRoXjRQbqfn3ETNL9ZvQuarwAYC9Ix5MxUrS5ag\nOmfjc8bfkpYDFAXRXmdKNISJmtCebX2kDrpP8Bdasx7Fzsx59cEUHCl2aJOWXc7R\n5m3juOVL1HUxjQIDAQABoyAwHjAcBgNVHREEFTATghFnYXRld2F5Ki5jbWl4LnJp\ncDANBgkqhkiG9w0BAQUFAAOCAQEAMu3xoc2LW2UExAAIYYWEETggLNrlGonxteSu\njuJjOR+ik5SVLn0lEu22+z+FCA7gSk9FkWu+v9qnfOfm2Am+WKYWv3dJ5RypW/hD\nNXkOYxVJNYFxeShnHohNqq4eDKpdqSxEcuErFXJdLbZP1uNs4WIOKnThgzhkpuy7\ntZRosvOF1X5uL1frVJzHN5jASEDAa7hJNmQ24kh+ds/Ge39fGD8pK31CWhnIXeDo\nvKD7wivi/gSOBtcRWWLvU8SizZkS3hgTw0lSOf5geuzvasCEYlqrKFssj6cTzbCB\nxy3ra3WazRTNTW4TmkHlCUC9I3oWTTxw5iQxF/I2kQQnwR7L3w==\n-----END CERTIFICATE-----"
		},
		{
			"Id": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1],
			"Address": "2.2.58.38",
			"Tls_certificate": "-----BEGIN CERTIFICATE-----\nMIIDgTCCAmmgAwIBAgIJAKLdZ8UigIAeMA0GCSqGSIb3DQEBBQUAMG8xCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlDbGFyZW1vbnQx\nGzAZBgNVBAoMElByaXZhdGVncml0eSBDb3JwLjEaMBgGA1UEAwwRZ2F0ZXdheSou\nY21peC5yaXAwHhcNMTkwMzA1MTgzNTU0WhcNMjkwMzAyMTgzNTU0WjBvMQswCQYD\nVQQGEwJVUzETMBEGA1UECAwKQ2FsaWZvcm5pYTESMBAGA1UEBwwJQ2xhcmVtb250\nMRswGQYDVQQKDBJQcml2YXRlZ3JpdHkgQ29ycC4xGjAYBgNVBAMMEWdhdGV3YXkq\nLmNtaXgucmlwMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA9+AaxwDP\nxHbhLmn4HoZu0oUM48Qufc6T5XEZTrpMrqJAouXk+61Jc0EFH96/sbj7VyvnXPRo\ngIENbk2Y84BkB9SkRMIXya/gh9dOEDSgnvj/yg24l3bdKFqBMKiFg00PYB30fU+A\nbe3OI/le0I+v++RwH2AV0BMq+T6PcAGjCC1Q1ZB0wP9/VqNMWq5lbK9wD46IQiSi\n+SgIQeE7HoiAZXrGO0Y7l9P3+VRoXjRQbqfn3ETNL9ZvQuarwAYC9Ix5MxUrS5ag\nOmfjc8bfkpYDFAXRXmdKNISJmtCebX2kDrpP8Bdasx7Fzsx59cEUHCl2aJOWXc7R\n5m3juOVL1HUxjQIDAQABoyAwHjAcBgNVHREEFTATghFnYXRld2F5Ki5jbWl4LnJp\ncDANBgkqhkiG9w0BAQUFAAOCAQEAMu3xoc2LW2UExAAIYYWEETggLNrlGonxteSu\njuJjOR+ik5SVLn0lEu22+z+FCA7gSk9FkWu+v9qnfOfm2Am+WKYWv3dJ5RypW/hD\nNXkOYxVJNYFxeShnHohNqq4eDKpdqSxEcuErFXJdLbZP1uNs4WIOKnThgzhkpuy7\ntZRosvOF1X5uL1frVJzHN5jASEDAa7hJNmQ24kh+ds/Ge39fGD8pK31CWhnIXeDo\nvKD7wivi/gSOBtcRWWLvU8SizZkS3hgTw0lSOf5geuzvasCEYlqrKFssj6cTzbCB\nxy3ra3WazRTNTW4TmkHlCUC9I3oWTTxw5iQxF/I2kQQnwR7L3w==\n-----END CERTIFICATE-----"
		},
		{
			"Id": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1],
			"Address": "52.41.80.104",
			"Tls_certificate": "-----BEGIN CERTIFICATE-----\nMIIDgTCCAmmgAwIBAgIJAKLdZ8UigIAeMA0GCSqGSIb3DQEBBQUAMG8xCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlDbGFyZW1vbnQx\nGzAZBgNVBAoMElByaXZhdGVncml0eSBDb3JwLjEaMBgGA1UEAwwRZ2F0ZXdheSou\nY21peC5yaXAwHhcNMTkwMzA1MTgzNTU0WhcNMjkwMzAyMTgzNTU0WjBvMQswCQYD\nVQQGEwJVUzETMBEGA1UECAwKQ2FsaWZvcm5pYTESMBAGA1UEBwwJQ2xhcmVtb250\nMRswGQYDVQQKDBJQcml2YXRlZ3JpdHkgQ29ycC4xGjAYBgNVBAMMEWdhdGV3YXkq\nLmNtaXgucmlwMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA9+AaxwDP\nxHbhLmn4HoZu0oUM48Qufc6T5XEZTrpMrqJAouXk+61Jc0EFH96/sbj7VyvnXPRo\ngIENbk2Y84BkB9SkRMIXya/gh9dOEDSgnvj/yg24l3bdKFqBMKiFg00PYB30fU+A\nbe3OI/le0I+v++RwH2AV0BMq+T6PcAGjCC1Q1ZB0wP9/VqNMWq5lbK9wD46IQiSi\n+SgIQeE7HoiAZXrGO0Y7l9P3+VRoXjRQbqfn3ETNL9ZvQuarwAYC9Ix5MxUrS5ag\nOmfjc8bfkpYDFAXRXmdKNISJmtCebX2kDrpP8Bdasx7Fzsx59cEUHCl2aJOWXc7R\n5m3juOVL1HUxjQIDAQABoyAwHjAcBgNVHREEFTATghFnYXRld2F5Ki5jbWl4LnJp\ncDANBgkqhkiG9w0BAQUFAAOCAQEAMu3xoc2LW2UExAAIYYWEETggLNrlGonxteSu\njuJjOR+ik5SVLn0lEu22+z+FCA7gSk9FkWu+v9qnfOfm2Am+WKYWv3dJ5RypW/hD\nNXkOYxVJNYFxeShnHohNqq4eDKpdqSxEcuErFXJdLbZP1uNs4WIOKnThgzhkpuy7\ntZRosvOF1X5uL1frVJzHN5jASEDAa7hJNmQ24kh+ds/Ge39fGD8pK31CWhnIXeDo\nvKD7wivi/gSOBtcRWWLvU8SizZkS3hgTw0lSOf5geuzvasCEYlqrKFssj6cTzbCB\nxy3ra3WazRTNTW4TmkHlCUC9I3oWTTxw5iQxF/I2kQQnwR7L3w==\n-----END CERTIFICATE-----"
		}
	],
	"nodes": [
		{
			"Id": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2],
			"Address": "18.237.147.105",
			"Tls_certificate": "-----BEGIN CERTIFICATE-----\nMIIDbDCCAlSgAwIBAgIJAOUNtZneIYECMA0GCSqGSIb3DQEBBQUAMGgxCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlDbGFyZW1vbnQx\nGzAZBgNVBAoMElByaXZhdGVncml0eSBDb3JwLjETMBEGA1UEAwwKKi5jbWl4LnJp\ncDAeFw0xOTAzMDUxODM1NDNaFw0yOTAzMDIxODM1NDNaMGgxCzAJBgNVBAYTAlVT\nMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlDbGFyZW1vbnQxGzAZBgNV\nBAoMElByaXZhdGVncml0eSBDb3JwLjETMBEGA1UEAwwKKi5jbWl4LnJpcDCCASIw\nDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAPP0WyVkfZA/CEd2DgKpcudn0oDh\nDwsjmx8LBDWsUgQzyLrFiVigfUmUefknUH3dTJjmiJtGqLsayCnWdqWLHPJYvFfs\nWYW0IGF93UG/4N5UAWO4okC3CYgKSi4ekpfw2zgZq0gmbzTnXcHF9gfmQ7jJUKSE\ntJPSNzXq+PZeJTC9zJAb4Lj8QzH18rDM8DaL2y1ns0Y2Hu0edBFn/OqavBJKb/uA\nm3AEjqeOhC7EQUjVamWlTBPt40+B/6aFJX5BYm2JFkRsGBIyBVL46MvC02MgzTT9\nbJIJfwqmBaTruwemNgzGu7Jk03hqqS1TUEvSI6/x8bVoba3orcKkf9HsDjECAwEA\nAaMZMBcwFQYDVR0RBA4wDIIKKi5jbWl4LnJpcDANBgkqhkiG9w0BAQUFAAOCAQEA\nneUocN4AbcQAC1+b3To8u5UGdaGxhcGyZBlAoenRVdjXK3lTjsMdMWb4QctgNfIf\nU/zuUn2mxTmF/ekP0gCCgtleZr9+DYKU5hlXk8K10uKxGD6EvoiXZzlfeUuotgp2\nqvI3ysOm/hvCfyEkqhfHtbxjV7j7v7eQFPbvNaXbLa0yr4C4vMK/Z09Ui9JrZ/Z4\ncyIkxfC6/rOqAirSdIp09EGiw7GM8guHyggE4IiZrDslT8V3xIl985cbCxSxeW1R\ntgH4rdEXuVe9+31oJhmXOE9ux2jCop9tEJMgWg7HStrJ5plPbb+HmjoX3nBO04E5\n6m52PyzMNV+2N21IPppKwA==\n-----END CERTIFICATE-----"
		},
		{
			"Id": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2],
			"Address": "52.11.136.238",
			"Tls_certificate": "-----BEGIN CERTIFICATE-----\nMIIDbDCCAlSgAwIBAgIJAOUNtZneIYECMA0GCSqGSIb3DQEBBQUAMGgxCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlDbGFyZW1vbnQx\nGzAZBgNVBAoMElByaXZhdGVncml0eSBDb3JwLjETMBEGA1UEAwwKKi5jbWl4LnJp\ncDAeFw0xOTAzMDUxODM1NDNaFw0yOTAzMDIxODM1NDNaMGgxCzAJBgNVBAYTAlVT\nMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlDbGFyZW1vbnQxGzAZBgNV\nBAoMElByaXZhdGVncml0eSBDb3JwLjETMBEGA1UEAwwKKi5jbWl4LnJpcDCCASIw\nDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAPP0WyVkfZA/CEd2DgKpcudn0oDh\nDwsjmx8LBDWsUgQzyLrFiVigfUmUefknUH3dTJjmiJtGqLsayCnWdqWLHPJYvFfs\nWYW0IGF93UG/4N5UAWO4okC3CYgKSi4ekpfw2zgZq0gmbzTnXcHF9gfmQ7jJUKSE\ntJPSNzXq+PZeJTC9zJAb4Lj8QzH18rDM8DaL2y1ns0Y2Hu0edBFn/OqavBJKb/uA\nm3AEjqeOhC7EQUjVamWlTBPt40+B/6aFJX5BYm2JFkRsGBIyBVL46MvC02MgzTT9\nbJIJfwqmBaTruwemNgzGu7Jk03hqqS1TUEvSI6/x8bVoba3orcKkf9HsDjECAwEA\nAaMZMBcwFQYDVR0RBA4wDIIKKi5jbWl4LnJpcDANBgkqhkiG9w0BAQUFAAOCAQEA\nneUocN4AbcQAC1+b3To8u5UGdaGxhcGyZBlAoenRVdjXK3lTjsMdMWb4QctgNfIf\nU/zuUn2mxTmF/ekP0gCCgtleZr9+DYKU5hlXk8K10uKxGD6EvoiXZzlfeUuotgp2\nqvI3ysOm/hvCfyEkqhfHtbxjV7j7v7eQFPbvNaXbLa0yr4C4vMK/Z09Ui9JrZ/Z4\ncyIkxfC6/rOqAirSdIp09EGiw7GM8guHyggE4IiZrDslT8V3xIl985cbCxSxeW1R\ntgH4rdEXuVe9+31oJhmXOE9ux2jCop9tEJMgWg7HStrJ5plPbb+HmjoX3nBO04E5\n6m52PyzMNV+2N21IPppKwA==\n-----END CERTIFICATE-----"
		},
		{
			"Id": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2],
			"Address": "34.213.79.31",
			"Tls_certificate": "-----BEGIN CERTIFICATE-----\nMIIDbDCCAlSgAwIBAgIJAOUNtZneIYECMA0GCSqGSIb3DQEBBQUAMGgxCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlDbGFyZW1vbnQx\nGzAZBgNVBAoMElByaXZhdGVncml0eSBDb3JwLjETMBEGA1UEAwwKKi5jbWl4LnJp\ncDAeFw0xOTAzMDUxODM1NDNaFw0yOTAzMDIxODM1NDNaMGgxCzAJBgNVBAYTAlVT\nMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlDbGFyZW1vbnQxGzAZBgNV\nBAoMElByaXZhdGVncml0eSBDb3JwLjETMBEGA1UEAwwKKi5jbWl4LnJpcDCCASIw\nDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAPP0WyVkfZA/CEd2DgKpcudn0oDh\nDwsjmx8LBDWsUgQzyLrFiVigfUmUefknUH3dTJjmiJtGqLsayCnWdqWLHPJYvFfs\nWYW0IGF93UG/4N5UAWO4okC3CYgKSi4ekpfw2zgZq0gmbzTnXcHF9gfmQ7jJUKSE\ntJPSNzXq+PZeJTC9zJAb4Lj8QzH18rDM8DaL2y1ns0Y2Hu0edBFn/OqavBJKb/uA\nm3AEjqeOhC7EQUjVamWlTBPt40+B/6aFJX5BYm2JFkRsGBIyBVL46MvC02MgzTT9\nbJIJfwqmBaTruwemNgzGu7Jk03hqqS1TUEvSI6/x8bVoba3orcKkf9HsDjECAwEA\nAaMZMBcwFQYDVR0RBA4wDIIKKi5jbWl4LnJpcDANBgkqhkiG9w0BAQUFAAOCAQEA\nneUocN4AbcQAC1+b3To8u5UGdaGxhcGyZBlAoenRVdjXK3lTjsMdMWb4QctgNfIf\nU/zuUn2mxTmF/ekP0gCCgtleZr9+DYKU5hlXk8K10uKxGD6EvoiXZzlfeUuotgp2\nqvI3ysOm/hvCfyEkqhfHtbxjV7j7v7eQFPbvNaXbLa0yr4C4vMK/Z09Ui9JrZ/Z4\ncyIkxfC6/rOqAirSdIp09EGiw7GM8guHyggE4IiZrDslT8V3xIl985cbCxSxeW1R\ntgH4rdEXuVe9+31oJhmXOE9ux2jCop9tEJMgWg7HStrJ5plPbb+HmjoX3nBO04E5\n6m52PyzMNV+2N21IPppKwA==\n-----END CERTIFICATE-----"
		}
	],
	"registration": {
		"Address": "92.42.125.61",
		"Tls_certificate": "-----BEGIN CERTIFICATE-----\nMIIDkDCCAnigAwIBAgIJAJnjosuSsP7gMA0GCSqGSIb3DQEBBQUAMHQxCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlDbGFyZW1vbnQx\nGzAZBgNVBAoMElByaXZhdGVncml0eSBDb3JwLjEfMB0GA1UEAwwWcmVnaXN0cmF0\naW9uKi5jbWl4LnJpcDAeFw0xOTAzMDUyMTQ5NTZaFw0yOTAzMDIyMTQ5NTZaMHQx\nCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlDbGFy\nZW1vbnQxGzAZBgNVBAoMElByaXZhdGVncml0eSBDb3JwLjEfMB0GA1UEAwwWcmVn\naXN0cmF0aW9uKi5jbWl4LnJpcDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC\nggEBAOQKvqjdh35o+MECBhCwopJzPlQNmq2iPbewRNtI02bUNK3kLQUbFlYdzNGZ\nS4GYXGc5O+jdi8Slx82r1kdjz5PPCNFBARIsOP/L8r3DGeW+yeJdgBZjm1s3ylka\nmt4Ajiq/bNjysS6L/WSOp+sVumDxtBEzO/UTU1O6QRnzUphLaiWENmErGvsH0CZV\nq38Ia58k/QjCAzpUcYi4j2l1fb07xqFcQD8H6SmUM297UyQosDrp8ukdIo31Koxr\n4XDnnNNsYStC26tzHMeKuJ2Wl+3YzsSyflfM2YEcKE31sqB9DS36UkJ8J84eLsHN\nImGg3WodFAviDB67+jXDbB30NkMCAwEAAaMlMCMwIQYDVR0RBBowGIIWcmVnaXN0\ncmF0aW9uKi5jbWl4LnJpcDANBgkqhkiG9w0BAQUFAAOCAQEAF9mNzk+g+o626Rll\nt3f3/1qIyYQrYJ0BjSWCKYEFMCgZ4JibAJjAvIajhVYERtltffM+YKcdE2kTpdzJ\n0YJuUnRfuv6sVnXlVVugUUnd4IOigmjbCdM32k170CYMm0aiwGxl4FrNa8ei7AIa\nx/s1n+sqWq3HeW5LXjnoVb+s3HeCWIuLfcgrurfye8FnNhy14HFzxVYYefIKm0XL\n+DPlcGGGm/PPYt3u4a2+rP3xaihc65dTa0u5tf/XPXtPxTDPFj2JeQDFxo7QRREb\nPD89CtYnwuP937CrkvCKrL0GkW1FViXKqZY9F5uhxrvLIpzhbNrs/EbtweY35XGL\nDCCMkg==\n-----END CERTIFICATE-----",
		"EllipticPubKey":"MqaJJ3GjFisNRM6LRedRnooi14gepMaQxyWctXVU/w4="
	},
	"notification": {
		"Address": "notification.default.cmix.rip",
		"Tls_certificate": "-----BEGIN CERTIFICATE-----\nMIIDkDCCAnigAwIBAgIJAJnjosuSsP7gMA0GCSqGSIb3DQEBBQUAMHQxCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlDbGFyZW1vbnQx\nGzAZBgNVBAoMElByaXZhdGVncml0eSBDb3JwLjEfMB0GA1UEAwwWcmVnaXN0cmF0\naW9uKi5jbWl4LnJpcDAeFw0xOTAzMDUyMTQ5NTZaFw0yOTAzMDIyMTQ5NTZaMHQx\nCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlDbGFy\nZW1vbnQxGzAZBgNVBAoMElByaXZhdGVncml0eSBDb3JwLjEfMB0GA1UEAwwWcmVn\naXN0cmF0aW9uKi5jbWl4LnJpcDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC\nggEBAOQKvqjdh35o+MECBhCwopJzPlQNmq2iPbewRNtI02bUNK3kLQUbFlYdzNGZ\nS4GYXGc5O+jdi8Slx82r1kdjz5PPCNFBARIsOP/L8r3DGeW+yeJdgBZjm1s3ylka\nmt4Ajiq/bNjysS6L/WSOp+sVumDxtBEzO/UTU1O6QRnzUphLaiWENmErGvsH0CZV\nq38Ia58k/QjCAzpUcYi4j2l1fb07xqFcQD8H6SmUM297UyQosDrp8ukdIo31Koxr\n4XDnnNNsYStC26tzHMeKuJ2Wl+3YzsSyflfM2YEcKE31sqB9DS36UkJ8J84eLsHN\nImGg3WodFAviDB67+jXDbB30NkMCAwEAAaMlMCMwIQYDVR0RBBowGIIWcmVnaXN0\ncmF0aW9uKi5jbWl4LnJpcDANBgkqhkiG9w0BAQUFAAOCAQEAF9mNzk+g+o626Rll\nt3f3/1qIyYQrYJ0BjSWCKYEFMCgZ4JibAJjAvIajhVYERtltffM+YKcdE2kTpdzJ\n0YJuUnRfuv6sVnXlVVugUUnd4IOigmjbCdM32k170CYMm0aiwGxl4FrNa8ei7AIa\nx/s1n+sqWq3HeW5LXjnoVb+s3HeCWIuLfcgrurfye8FnNhy14HFzxVYYefIKm0XL\n+DPlcGGGm/PPYt3u4a2+rP3xaihc65dTa0u5tf/XPXtPxTDPFj2JeQDFxo7QRREb\nPD89CtYnwuP937CrkvCKrL0GkW1FViXKqZY9F5uhxrvLIpzhbNrs/EbtweY35XGL\nDCCMkg==\n-----END CERTIFICATE-----"
	},
	"udb": {
		"Id": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3]
	},
	"E2e": {
		"Prime": "FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3DC2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F83655D23DCA3AD961C62F356208552BB9ED529077096966D670C354E4ABC9804F1746C08CA18217C32905E462E36CE3BE39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9DE2BCBF6955817183995497CEA956AE515D2261898FA051015728E5A8AACAA68FFFFFFFFFFFFFFFF",
		"Small_prime": "7FFFFFFFFFFFFFFFE487ED5110B4611A62633145C06E0E68948127044533E63A0105DF531D89CD9128A5043CC71A026EF7CA8CD9E69D218D98158536F92F8A1BA7F09AB6B6A8E122F242DABB312F3F637A262174D31BF6B585FFAE5B7A035BF6F71C35FDAD44CFD2D74F9208BE258FF324943328F6722D9EE1003E5C50B1DF82CC6D241B0E2AE9CD348B1FD47E9267AFC1B2AE91EE51D6CB0E3179AB1042A95DCF6A9483B84B4B36B3861AA7255E4C0278BA3604650C10BE19482F23171B671DF1CF3B960C074301CD93C1D17603D147DAE2AEF837A62964EF15E5FB4AAC0B8C1CCAA4BE754AB5728AE9130C4C7D02880AB9472D455655347FFFFFFFFFFFFFFF",
		"Generator": "02"
	},
	"Cmix": {
		"Prime": "FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3DC2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F83655D23DCA3AD961C62F356208552BB9ED529077096966D670C354E4ABC9804F1746C08CA18217C32905E462E36CE3BE39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9DE2BCBF6955817183995497CEA956AE515D2261898FA051015728E5A8AACAA68FFFFFFFFFFFFFFFF",
		"Small_prime": "7FFFFFFFFFFFFFFFE487ED5110B4611A62633145C06E0E68948127044533E63A0105DF531D89CD9128A5043CC71A026EF7CA8CD9E69D218D98158536F92F8A1BA7F09AB6B6A8E122F242DABB312F3F637A262174D31BF6B585FFAE5B7A035BF6F71C35FDAD44CFD2D74F9208BE258FF324943328F6722D9EE1003E5C50B1DF82CC6D241B0E2AE9CD348B1FD47E9267AFC1B2AE91EE51D6CB0E3179AB1042A95DCF6A9483B84B4B36B3861AA7255E4C0278BA3604650C10BE19482F23171B671DF1CF3B960C074301CD93C1D17603D147DAE2AEF837A62964EF15E5FB4AAC0B8C1CCAA4BE754AB5728AE9130C4C7D02880AB9472D455655347FFFFFFFFFFFFFFF",
		"Generator": "02"
	}
}`)
)
