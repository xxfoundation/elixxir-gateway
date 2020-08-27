///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"fmt"
	"github.com/golang/protobuf/ptypes/any"
	"gitlab.com/elixxir/comms/gateway"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/comms/node"
	"gitlab.com/elixxir/comms/testkeys"
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/crypto/large"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/elixxir/primitives/format"
	"gitlab.com/elixxir/primitives/rateLimiting"
	"gitlab.com/elixxir/primitives/utils"
	"gitlab.com/xx_network/comms/connect"
	xx_pb "gitlab.com/xx_network/comms/messages"
	"gitlab.com/xx_network/crypto/signature"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/ndf"
	"google.golang.org/grpc"
	"os"
	"reflect"
	"testing"
	"time"
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

const prime = ("" +
	"9DB6FB5951B66BB6FE1E140F1D2CE5502374161FD6538DF1648218642F0B5C48" +
	"C8F7A41AADFA187324B87674FA1822B00F1ECF8136943D7C55757264E5A1A44F" +
	"FE012E9936E00C1D3E9310B01C7D179805D3058B2A9F4BB6F9716BFE6117C6B5" +
	"B3CC4D9BE341104AD4A80AD6C94E005F4B993E14F091EB51743BF33050C38DE2" +
	"35567E1B34C3D6A5C0CEAA1A0F368213C3D19843D0B4B09DCB9FC72D39C8DE41" +
	"F1BF14D4BB4563CA28371621CAD3324B6A2D392145BEBFAC748805236F5CA2FE" +
	"92B871CD8F9C36D3292B5509CA8CAA77A2ADFC7BFD77DDA6F71125A7456FEA15" +
	"3E433256A2261C6A06ED3693797E7995FAD5AABBCFBE3EDA2741E375404AE25B")
const generator = ("" +
	"5C7FF6B06F8F143FE8288433493E4769C4D988ACE5BE25A0E24809670716C613" +
	"D7B0CEE6932F8FAA7C44D2CB24523DA53FBE4F6EC3595892D1AA58C4328A06C4" +
	"6A15662E7EAA703A1DECF8BBB2D05DBE2EB956C142A338661D10461C0D135472" +
	"085057F3494309FFA73C611F78B32ADBB5740C361C9F35BE90997DB2014E2EF5" +
	"AA61782F52ABEB8BD6432C4DD097BC5423B285DAFB60DC364E8161F4A2A35ACA" +
	"3A10B1C4D203CC76A470A33AFDCBDD92959859ABD8B56E1725252D78EAC66E71" +
	"BA9AE3F1DD2487199874393CD4D832186800654760E1E34C09E4D155179F9EC0" +
	"DC4473F996BDCE6EED1CABED8B6F116F7AD9CF505DF0F998E34AB27514B0FFE7")

// This sets up a dummy/mock globals instance for testing purposes
func TestMain(m *testing.M) {

	//Begin gateway comms
	cmixNodes := make([]string, 1)
	cmixNodes[0] = GW_ADDRESS

	gatewayCert, _ = utils.ReadFile(testkeys.GetGatewayCertPath())
	gatewayKey, _ = utils.ReadFile(testkeys.GetGatewayKeyPath())

	gComm = gateway.StartGateway(&id.TempGateway, GW_ADDRESS,
		gatewayInstance, gatewayCert, gatewayKey)

	//Start mock node
	nodeHandler := buildTestNodeImpl()

	nodeCert, _ = utils.ReadFile(testkeys.GetNodeCertPath())
	nodeKey, _ = utils.ReadFile(testkeys.GetNodeKeyPath())
	n = node.StartNode(id.NewIdFromString("node", id.Node, m), NODE_ADDRESS, nodeHandler, nodeCert, nodeKey)

	grp = make(map[string]string)
	grp["prime"] = prime
	grp["generator"] = generator

	// Construct the rate limiting params
	bucketMapParams := &rateLimiting.MapParams{
		Capacity:     capacity,
		LeakedTokens: leakedTokens,
		LeakDuration: leakDuration,
		PollDuration: pollDuration,
		BucketMaxAge: bucketMaxAge,
	}

	//Build the gateway instance
	params := Params{
		NodeAddress:       NODE_ADDRESS,
		ServerCertPath:    testkeys.GetNodeCertPath(),
		CertPath:          testkeys.GetGatewayCertPath(),
		KeyPath:           testkeys.GetGatewayKeyPath(),
		MessageTimeout:    10 * time.Minute,
		rateLimiterParams: bucketMapParams,
	}

	gatewayInstance = NewGatewayInstance(params)
	gatewayInstance.Comms = gComm
	gatewayInstance.ServerHost, _ = connect.NewHost(id.NewIdFromString("node", id.Node, m), NODE_ADDRESS,
		nodeCert, true, false)
	gatewayInstance.setupGossiper()

	p := large.NewIntFromString(prime, 16)
	g := large.NewIntFromString(generator, 16)
	grp2 := cyclic.NewGroup(p, g)

	testNDF, _, _ := ndf.DecodeNDF(ExampleJSON + "\n" + ExampleSignature)

	// This is bad. It needs to be fixed (Ben's fault for not fixing correctly)
	t := testing.T{}
	var err error
	gatewayInstance.NetInf, err = network.NewInstanceTesting(gatewayInstance.Comms.ProtoComms, testNDF, testNDF, grp2, grp2, &t)
	if err != nil {
		t.Errorf("NewInstanceTesting encountered an error: %+v", err)
	}

	// build a single mock message
	msg := format.NewMessage()

	payloadA := make([]byte, format.PayloadLen)
	payloadA[0] = 1
	msg.SetPayloadA(payloadA)

	recipientID := id.NewIdFromUInt(1, id.User, m).Marshal()
	msg.AssociatedData.SetRecipientID(recipientID[:len(recipientID)-1])

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
		//build a batch
		b := pb.Batch{
			Round: &pb.RoundInfo{
				ID: 42, //meaning of life
			},
			FromPhase: 0,
			Slots: []*pb.Slot{
				mockMessage,
			},
		}

		return &b, nil
	}

	nodeHandler.Functions.Poll = func(p *pb.ServerPoll,
		auth *connect.Auth, gatewayAddress string) (*pb.ServerPollResponse,
		error) {
		netDef := pb.ServerPollResponse{}
		return &netDef, nil
	}
	return nodeHandler
}

//Tests that receiving messages and sending them to the node works
func TestGatewayImpl_SendBatch(t *testing.T) {
	data := format.NewMessage()
	rndId := uint64(1)

	msg := pb.Slot{
		SenderID: id.NewIdFromUInt(666, id.User, t).Marshal(),
		PayloadA: data.GetPayloadA(),
		PayloadB: data.GetPayloadB(),
	}

	slotMsg := &pb.GatewaySlot{RoundID: rndId,
		Message: &msg,
	}

	// Insert client information to database
	newClient := &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.database.InsertClient(newClient)
	pub := testkeys.LoadFromPath(testkeys.GetNodeCertPath())
	_, err := gatewayInstance.Comms.AddHost(&id.Permissioning,
		"0.0.0.0:4200", pub, false, true)

	ri := &pb.RoundInfo{ID: (rndId), BatchSize: 4}
	gatewayInstance.UnmixedBuffer.SetAsRoundLeader(id.Round(rndId), ri.BatchSize)

	rnd := &storage.Round{
		Id:       rndId,
		UpdateId: 0,
	}
	gatewayInstance.database.UpsertRound(rnd)
	_, err = gatewayInstance.PutMessage(slotMsg, "0")
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages!"+
			"Error received: %v", err)
	}

	ri = &pb.RoundInfo{ID: 1, BatchSize: 4}
	gatewayInstance.SendBatchWhenReady(ri)

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
	//Begin gateway comms
	cmixNodes := make([]string, 1)
	cmixNodes[0] = GW_ADDRESS

	// Construct the rate limiting params
	bucketMapParams := &rateLimiting.MapParams{
		Capacity:     capacity,
		LeakedTokens: leakedTokens,
		LeakDuration: leakDuration,
		PollDuration: pollDuration,
		BucketMaxAge: bucketMaxAge,
	}

	//Build the gateway instance
	params := Params{
		NodeAddress:       NODE_ADDRESS,
		ServerCertPath:    testkeys.GetNodeCertPath(),
		CertPath:          testkeys.GetGatewayCertPath(),
		MessageTimeout:    10 * time.Minute,
		rateLimiterParams: bucketMapParams,
	}

	fmt.Println("making instance")

	gw := NewGatewayInstance(params)
	p := large.NewIntFromString(prime, 16)
	g := large.NewIntFromString(generator, 16)
	grp2 := cyclic.NewGroup(p, g)

	testNDF, _, _ := ndf.DecodeNDF(ExampleJSON + "\n" + ExampleSignature)

	var err error
	gw.NetInf, err = network.NewInstanceTesting(nil, testNDF, testNDF, grp2, grp2, t)
	if err != nil {
		t.Errorf("NewInstanceTesting encountered an error: %+v", err)
	}
	fmt.Println("made new instance")
	gw.Comms = gComm
	gw.ServerHost, err = connect.NewHost(id.NewIdFromString("test", id.Node, t), NODE_ADDRESS,
		nodeCert, true, false)
	if err != nil {
		t.Errorf(err.Error())
	}

	data := format.NewMessage()
	rndId := uint64(1)

	msg := pb.Slot{
		SenderID: id.NewIdFromUInt(666, id.User, t).Marshal(),
		PayloadA: data.GetPayloadA(),
		PayloadB: data.GetPayloadB(),
	}

	slotMsg := &pb.GatewaySlot{RoundID: rndId,
		Message: &msg,
	}

	// Insert client information to database
	newClient := &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}
	fmt.Println("initializng business logic")
	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gw.database.InsertClient(newClient)
	pub := testkeys.LoadFromPath(testkeys.GetNodeCertPath())
	_, err = gw.Comms.AddHost(&id.Permissioning,
		"0.0.0.0:4200", pub, false, true)

	ri := &pb.RoundInfo{ID: (rndId), BatchSize: 4}
	gw.UnmixedBuffer.SetAsRoundLeader(id.Round(rndId), ri.BatchSize)

	rnd := &storage.Round{
		Id:       rndId,
		UpdateId: 0,
	}
	gw.database.UpsertRound(rnd)
	_, err = gw.PutMessage(slotMsg, "0")
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages!"+
			"Error received: %v", err)
	}

	gw.setupGossiper()
	fmt.Printf("setting up largerBatch")
	si := &pb.RoundInfo{ID: 1, BatchSize: 4}
	gw.SendBatchWhenReady(si)

}

// Calling InitNetwork after starting a node should cause
// gateway to connect to the node
func TestInitNetwork_ConnectsToNode(t *testing.T) {
	defer disconnectServers()

	disablePermissioning = true

	err := gatewayInstance.InitNetwork()
	if err != nil {
		t.Errorf(err.Error())
	}

	ctx, cancel := connect.MessagingContext()
	gatewayInstance.ServerHost, _ = connect.NewHost(id.NewIdFromString("node", id.Node, t), NODE_ADDRESS,
		nodeCert, true, false)

	_, err = gatewayInstance.Comms.Send(gatewayInstance.ServerHost, func(
		conn *grpc.ClientConn) (*any.
		Any, error) {
		_, err = pb.NewNodeClient(conn).AskOnline(ctx, &xx_pb.Ping{})

		// Make sure there are no errors with sending the message
		if err != nil {
			return nil, err
		}
		return nil, nil
	})
	if err != nil {
		t.Errorf(err.Error())
	}

	disconnectServers()
	cancel()

}

func disconnectServers() {
	gatewayInstance.Comms.DisconnectAll()
	n.Manager.DisconnectAll()
	n.DisconnectAll()
}

// Tests that messages can get through when its IP address bucket is not full
// and checks that they are blocked when the bucket is full.
func TestGatewayImpl_PutMessage_IpBlock(t *testing.T) {
	time.Sleep(2 * time.Second)
	rndId := uint64(1)
	data := format.NewMessage()
	msg := pb.Slot{
		SenderID: id.NewIdFromUInt(666, id.User, t).Marshal(),
		PayloadA: data.GetPayloadA(),
		PayloadB: data.GetPayloadB(),
	}
	slotMsg := &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}

	// Insert client information to database
	newClient := &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	ri := &pb.RoundInfo{ID: (rndId), BatchSize: 24}
	gatewayInstance.UnmixedBuffer.SetAsRoundLeader(id.Round(rndId), ri.BatchSize)

	gatewayInstance.database.InsertClient(newClient)

	_, err := gatewayInstance.PutMessage(slotMsg, "0")
	errMsg := ("PutMessage: Could not put any messages when IP address " +
		"should not be blocked")
	if err != nil {
		t.Errorf("%s: %v", errMsg, err.Error())
	}

	gatewayInstance.database.InsertClient(newClient)

	msg = pb.Slot{SenderID: id.NewIdFromUInt(67, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}

	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.database.InsertClient(newClient)
	gatewayInstance.database.InsertClient(newClient)

	_, err = gatewayInstance.PutMessage(slotMsg, "0")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)

		t.Errorf("%s: %v", errMsg, err.Error())
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(34, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.database.InsertClient(newClient)

	_, err = gatewayInstance.PutMessage(slotMsg, "0")
	if err != nil {
		t.Errorf("%s: %v", errMsg, err.Error())
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(0, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.database.InsertClient(newClient)

	_, err = gatewayInstance.PutMessage(slotMsg, "0")
	if err != nil {
		t.Errorf("%s: %v", errMsg, err.Error())
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(0, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.database.InsertClient(newClient)

	_, err = gatewayInstance.PutMessage(slotMsg, "1")
	if err != nil {
		t.Errorf("%s: %v", errMsg, err.Error())
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(0, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)

	// fixme: rate limiting checks aren't properly done
	//  until the storage interface is implemented, so messages won't be denied until then
	//_, err = gatewayInstance.PutMessage(slotMsg, "0")
	//if err == nil {
	//	t.Errorf(errMsg + " no error")
	//}

	time.Sleep(1 * time.Second)

	msg = pb.Slot{SenderID: id.NewIdFromUInt(34, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)

	_, err = gatewayInstance.PutMessage(slotMsg, "0")
	if err != nil {
		t.Errorf(errMsg + ": " + err.Error())
	}

	time.Sleep(1 * time.Second)

	msg = pb.Slot{SenderID: id.NewIdFromUInt(0, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)

	_, err = gatewayInstance.PutMessage(slotMsg, "0")
	if err != nil {
		t.Errorf(errMsg + ": " + err.Error())
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(0, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)

	_, err = gatewayInstance.PutMessage(slotMsg, "1")
	if err != nil {
		t.Errorf(errMsg + ": " + err.Error())
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(0, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)

	_, err = gatewayInstance.PutMessage(slotMsg, "0")
	if err != nil {
		t.Errorf(errMsg + ": " + err.Error())
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(0, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)

	_, err = gatewayInstance.PutMessage(slotMsg, "0")
	if err != nil {
		t.Errorf(errMsg + ": " + err.Error())
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(0, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)

	_, err = gatewayInstance.PutMessage(slotMsg, "0")
	if err != nil {
		t.Errorf(errMsg + ": " + err.Error())
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(0, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)

	// fixme: rate limiting checks aren't properly done
	//  until the storage interface is implemented, so messages won't be denied until then
	//_, err = gatewayInstance.PutMessage(slotMsg, "0")
	//if err == nil {
	//	t.Errorf(errMsg)
	//}
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
	rndId := uint64(1)

	msg = pb.Slot{SenderID: id.NewIdFromUInt(128, id.User, t).Marshal()}
	slotMsg := &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}

	// Insert client information to database
	newClient := &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}
	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.database.InsertClient(newClient)
	ri := &pb.RoundInfo{ID: (rndId), BatchSize: 24}
	gatewayInstance.UnmixedBuffer.SetAsRoundLeader(id.Round(rndId), ri.BatchSize)

	_, err = gatewayInstance.PutMessage(slotMsg, "158.85.140.178")
	errMsg := ("PutMessage: Could not put any messages when IP " +
		"address should not be blocked")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(129, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.database.InsertClient(newClient)
	_, err = gatewayInstance.PutMessage(slotMsg, "158.85.140.178")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(130, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.database.InsertClient(newClient)
	_, err = gatewayInstance.PutMessage(slotMsg, "158.85.140.178")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(131, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.database.InsertClient(newClient)
	_, err = gatewayInstance.PutMessage(slotMsg, "158.85.140.178")
	if err != nil {
		t.Errorf(errMsg)
	}

	time.Sleep(1 * time.Second)

	msg = pb.Slot{SenderID: id.NewIdFromUInt(132, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.database.InsertClient(newClient)
	_, err = gatewayInstance.PutMessage(slotMsg, "158.85.140.178")
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages when " +
			"IP bucket is full but message IP is on whitelist")
	}
}

// Error path: Test that a message is denied when the batch is full
func TestInstance_PutMessage_FullRound(t *testing.T) {
	// Business logic to set up test
	var msg pb.Slot
	rndId := uint64(1)
	batchSize := 4
	msg = pb.Slot{SenderID: id.NewIdFromUInt(128, id.User, t).Marshal()}
	slotMsg := &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}

	// Insert client information to database
	newClient := &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}
	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.database.InsertClient(newClient)

	// End of business logic

	// Mark this as a round in which the gateway is the leader
	ri := &pb.RoundInfo{ID: (rndId), BatchSize: uint32(batchSize)}
	gatewayInstance.UnmixedBuffer.SetAsRoundLeader(id.Round(rndId), ri.BatchSize)

	// Put a message in the same round to fill up the batch size
	for i := 0; i < batchSize; i++ {
		_, err := gatewayInstance.PutMessage(slotMsg, "0")
		if err != nil {
			t.Errorf("Failed to put message number %d into gateway's buffer: %v", i, err)
		}
	}

	_, err := gatewayInstance.PutMessage(slotMsg, "0")
	if err == nil {
		t.Errorf("Expected error path. Should not be able to put a message into a full round!")

	}
}

// Error path: Test that when the gateway is not the entry point for the round
// that the message is rejected
func TestInstance_PutMessage_NonLeader(t *testing.T) {
	// Business logic to set up test
	var msg pb.Slot
	rndId := uint64(1)
	msg = pb.Slot{SenderID: id.NewIdFromUInt(128, id.User, t).Marshal()}
	slotMsg := &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}

	// Insert client information to database
	newClient := &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}
	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.database.InsertClient(newClient)

	// End of business logic

	// Commented out explicitly. For this test we do NOT want the
	// gateway to be the leader for this round
	//ri := &pb.RoundInfo{ID:(rndId), BatchSize:uint32(batchSize)}
	//gatewayInstance.UnmixedBuffer.SetAsRoundLeader(id.Round(rndId), ri.BatchSize)

	_, err := gatewayInstance.PutMessage(slotMsg, "0")
	if err == nil {
		t.Errorf("Expected error path. Should not be able to put a message into a round when not the leader!")

	}
}

// Tests that messages can get through even when their bucket is full.
func TestGatewayImpl_PutMessage_UserWhitelist(t *testing.T) {
	var msg pb.Slot
	var err error
	rndId := uint64(1)
	msg = pb.Slot{SenderID: id.NewIdFromUInt(174, id.User, t).Marshal()}
	slotMsg := &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}

	ri := &pb.RoundInfo{ID: (rndId), BatchSize: 24}
	gatewayInstance.UnmixedBuffer.SetAsRoundLeader(id.Round(rndId), ri.BatchSize)

	// Insert client information to database
	newClient := &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.database.InsertClient(newClient)

	_, err = gatewayInstance.PutMessage(slotMsg, "aa")
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages when " +
			"IP address should not be blocked")
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(174, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.database.InsertClient(newClient)
	_, err = gatewayInstance.PutMessage(slotMsg, "bb")
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages when " +
			"IP address should not be blocked")
	}

	msg = pb.Slot{SenderID: id.NewIdFromUInt(174, id.User, t).Marshal()}
	slotMsg = &pb.GatewaySlot{
		RoundID: rndId,
		Message: &msg,
	}
	// Insert client information to database
	newClient = &storage.Client{
		Id:  msg.SenderID,
		Key: []byte("test"),
	}

	slotMsg.MAC = generateClientMac(newClient, slotMsg)
	gatewayInstance.database.InsertClient(newClient)
	_, err = gatewayInstance.PutMessage(slotMsg, "cc")
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages when user " +
			"ID bucket is full but user ID is on whitelist")
	}
}

// TestPollServer tests that the message is properly formed when sent to the
// connection manager.
func TestPollServer(t *testing.T) {
	// FIXME: This does nothing since the SendPoll in gateway comms uses
	//        a type and not the interface (so we can't do a unit test)
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
		"0.0.0.0:4200", pub, false, true)
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
	err = signature.Sign(ndfMsg, pKey)
	if err != nil {
		t.Errorf("%v", err)
	}

	netInst, err := CreateNetworkInstance(
		gatewayInstance.Comms, ndfMsg, ndfMsg)

	gatewayInstance.NetInf = netInst
	if err != nil {
		t.Errorf("%v", err)
	}
	if netInst == nil {
		t.Errorf("Could not create network instance!")
	}

}

// TestUpdateInstance tests that the instance updates itself appropriately
// FIXME: This test cannot test the Ndf functionality, since we don't have
//        signable ndf function that would enforce correctness, so not useful
//        at the moment.
func TestUpdateInstance(t *testing.T) {
	nodeId := id.NewIdFromString("node", id.Node, t)
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
	err = signature.Sign(ndfMsg, pKey)
	if err != nil {
		t.Errorf("%v", err)
	}

	// FIXME: the following will fail with a nil pointer deref if the
	//        CreateNetworkInstance test doesn't run....
	ri := &pb.RoundInfo{
		ID:        uint64(1),
		UpdateID:  uint64(1),
		State:     6,
		BatchSize: 8,
	}
	err = signature.Sign(ri, pKey)
	roundUpdates := []*pb.RoundInfo{ri}
	data := format.NewMessage()
	msg := &pb.Slot{
		SenderID: id.NewIdFromUInt(666, id.User, t).Marshal(),
		PayloadA: data.GetPayloadA(),
		PayloadB: data.GetPayloadB(),
	}

	serialmsg := format.NewMessage()
	serialmsg.SetPayloadB(msg.PayloadB)
	userId, err := serialmsg.GetRecipient()

	slots := []*pb.Slot{msg}

	update := &pb.ServerPollResponse{
		FullNDF:      ndfMsg,
		PartialNDF:   ndfMsg,
		Updates:      roundUpdates,
		BatchRequest: ri,
		Slots:        slots,
	}

	err = gatewayInstance.UpdateInstance(update)
	if err != nil {
		t.Errorf("UpdateInstance() produced an error: %+v", err)
	}

	// Check that updates made it
	r, err := gatewayInstance.NetInf.GetRoundUpdate(1)
	if err != nil || r == nil {
		t.Errorf("Failed to retrieve round update: %+v", err)
	}

	// Check that mockMessage made it
	mockMsgUserId := id.NewIdFromUInt(1, id.User, t)
	msgTst, err := gatewayInstance.database.GetMixedMessages(userId, id.Round(1))
	if err != nil {
		t.Errorf("%+v", err)
		msgIDs, _ := gatewayInstance.database.GetMixedMessages(
			mockMsgUserId, id.Round(1))
		for i := 0; i < len(msgIDs); i++ {
			print("%s", msgIDs[i])
		}
	}
	if msgTst == nil {
		t.Errorf("Did not return mock message!")
	}

	// Check that batchRequest was sent
	if len(nodeIncomingBatch.Slots) != 8 {
		t.Errorf("Did not send batch: %d", len(nodeIncomingBatch.Slots))
	}

}

// Smoke test for gossiper
func TestGossip(t *testing.T) {
	gatewayInstance.setupGossiper()
	// Ensure that gossiper is set up
	if gatewayInstance.gossiper == nil {
		t.Errorf("Gossiper not initialized!")
	}

	_, ok := gatewayInstance.gossiper.Get("batch")
	if !ok {
		t.Errorf("Could not retrieve default gossip protocol")
	}

}

var (
	ExampleJSON = `{
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
		"Tls_certificate": "-----BEGIN CERTIFICATE-----\nMIIDkDCCAnigAwIBAgIJAJnjosuSsP7gMA0GCSqGSIb3DQEBBQUAMHQxCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlDbGFyZW1vbnQx\nGzAZBgNVBAoMElByaXZhdGVncml0eSBDb3JwLjEfMB0GA1UEAwwWcmVnaXN0cmF0\naW9uKi5jbWl4LnJpcDAeFw0xOTAzMDUyMTQ5NTZaFw0yOTAzMDIyMTQ5NTZaMHQx\nCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRIwEAYDVQQHDAlDbGFy\nZW1vbnQxGzAZBgNVBAoMElByaXZhdGVncml0eSBDb3JwLjEfMB0GA1UEAwwWcmVn\naXN0cmF0aW9uKi5jbWl4LnJpcDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC\nggEBAOQKvqjdh35o+MECBhCwopJzPlQNmq2iPbewRNtI02bUNK3kLQUbFlYdzNGZ\nS4GYXGc5O+jdi8Slx82r1kdjz5PPCNFBARIsOP/L8r3DGeW+yeJdgBZjm1s3ylka\nmt4Ajiq/bNjysS6L/WSOp+sVumDxtBEzO/UTU1O6QRnzUphLaiWENmErGvsH0CZV\nq38Ia58k/QjCAzpUcYi4j2l1fb07xqFcQD8H6SmUM297UyQosDrp8ukdIo31Koxr\n4XDnnNNsYStC26tzHMeKuJ2Wl+3YzsSyflfM2YEcKE31sqB9DS36UkJ8J84eLsHN\nImGg3WodFAviDB67+jXDbB30NkMCAwEAAaMlMCMwIQYDVR0RBBowGIIWcmVnaXN0\ncmF0aW9uKi5jbWl4LnJpcDANBgkqhkiG9w0BAQUFAAOCAQEAF9mNzk+g+o626Rll\nt3f3/1qIyYQrYJ0BjSWCKYEFMCgZ4JibAJjAvIajhVYERtltffM+YKcdE2kTpdzJ\n0YJuUnRfuv6sVnXlVVugUUnd4IOigmjbCdM32k170CYMm0aiwGxl4FrNa8ei7AIa\nx/s1n+sqWq3HeW5LXjnoVb+s3HeCWIuLfcgrurfye8FnNhy14HFzxVYYefIKm0XL\n+DPlcGGGm/PPYt3u4a2+rP3xaihc65dTa0u5tf/XPXtPxTDPFj2JeQDFxo7QRREb\nPD89CtYnwuP937CrkvCKrL0GkW1FViXKqZY9F5uhxrvLIpzhbNrs/EbtweY35XGL\nDCCMkg==\n-----END CERTIFICATE-----"
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
}`
	ExampleSignature = `gkh98J10rQiuVsEXd6xe8IeCINplnD93CFpXZFNjT1CgNMxgsHumiC5HsctjnF0xTxDPq3hn3/J0s+eblSVyGMMszTIoWNINVSS1fkm0EGkKafC1vKTZMmc9ivsWL7oY`
)
