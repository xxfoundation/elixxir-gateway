////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"github.com/golang/protobuf/ptypes/any"
	"gitlab.com/elixxir/comms/connect"
	"gitlab.com/elixxir/comms/gateway"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/node"
	"gitlab.com/elixxir/comms/testkeys"
	"gitlab.com/elixxir/crypto/signature"
	"gitlab.com/elixxir/crypto/signature/rsa"
	"gitlab.com/elixxir/gateway/rateLimiting"
	"gitlab.com/elixxir/primitives/format"
	"gitlab.com/elixxir/primitives/id"
	"gitlab.com/elixxir/primitives/ndf"
	"gitlab.com/elixxir/primitives/utils"
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

	gComm = gateway.StartGateway(id.NewTmpGateway().String(), GW_ADDRESS,
		gatewayInstance, gatewayCert, gatewayKey)

	//Start mock node
	nodeHandler := buildTestNodeImpl()

	nodeCert, _ = utils.ReadFile(testkeys.GetNodeCertPath())
	nodeKey, _ = utils.ReadFile(testkeys.GetNodeKeyPath())
	n = node.StartNode("node", NODE_ADDRESS, nodeHandler, nodeCert, nodeKey)

	grp = make(map[string]string)
	grp["prime"] = prime
	grp["generator"] = generator

	//Build the gateway instance
	params := Params{
		NodeAddress:    NODE_ADDRESS,
		CMixNodes:      cmixNodes,
		CmixGrp:        grp,
		ServerCertPath: testkeys.GetNodeCertPath(),
		CertPath:       testkeys.GetGatewayCertPath(),
		KeyPath:        testkeys.GetGatewayKeyPath(),
	}

	cleanPeriodDur := 3 * time.Second
	maxDurationDur := 10 * time.Second

	rlPref := "../rateLimiting/whitelists/"

	params.Params = rateLimiting.Params{
		IpLeakRate:        0.0000012,
		UserLeakRate:      0.0000012,
		IpCapacity:        1240,
		UserCapacity:      500,
		CleanPeriod:       cleanPeriodDur,
		MaxDuration:       maxDurationDur,
		IpWhitelistFile:   rlPref + "ip_whitelist2.txt",
		UserWhitelistFile: rlPref + "user_whitelist.txt",
	}

	gatewayInstance = NewGatewayInstance(params)
	gatewayInstance.Comms = gComm
	gatewayInstance.ServerHost, _ = connect.NewHost("node", NODE_ADDRESS,
		nodeCert, true, false)

	//build a single mock message
	msg := format.NewMessage()

	payloadA := make([]byte, format.PayloadLen)
	payloadA[0] = 1
	msg.SetPayloadA(payloadA)

	UserIDBytes := make([]byte, id.UserLen)
	UserIDBytes[0] = 1
	msg.AssociatedData.SetRecipientID(UserIDBytes)

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
		auth *connect.Auth) (*pb.ServerPollResponse,
		error) {
		netDef := pb.ServerPollResponse{}
		return &netDef, nil
	}
	return nodeHandler
}

//Tests that receiving messages and sending them to the node works
func TestGatewayImpl_SendBatch(t *testing.T) {
	msg := pb.Slot{SenderID: id.NewUserFromUint(666, t).Bytes()}
	err := gatewayInstance.PutMessage(&msg, "0")
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages!")
	}

	ri := &pb.RoundInfo{ID: 1, BatchSize: 4}
	gatewayInstance.SendBatchWhenReady(ri)

	time.Sleep(1 * time.Second)

	if nodeIncomingBatch == nil {
		t.Errorf("Batch not recieved by node!")
	} else {
		if !reflect.DeepEqual(nodeIncomingBatch.Slots[0].SenderID,
			msg.SenderID) {
			t.Errorf("Message in batch not the same as sent;"+
				"\n  Expected: %+v \n  Recieved: %+v", msg,
				*nodeIncomingBatch.Slots[0])
		}
	}
}

func TestGatewayImpl_SendBatch_LargerBatchSize(t *testing.T) {
	msg := pb.Slot{SenderID: id.NewUserFromUint(666, t).Bytes()}
	err := gatewayInstance.PutMessage(&msg, "0")
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages!")
	}

	//Begin gateway comms
	cmixNodes := make([]string, 1)
	cmixNodes[0] = GW_ADDRESS
	//Build the gateway instance
	params := Params{
		NodeAddress:    NODE_ADDRESS,
		CMixNodes:      cmixNodes,
		CmixGrp:        grp,
		ServerCertPath: testkeys.GetNodeCertPath(),
		CertPath:       testkeys.GetGatewayCertPath(),
	}

	cleanPeriodDur := 3 * time.Second
	maxDurationDur := 10 * time.Second

	rlPref := "../rateLimiting/whitelists/"
	params.Params = rateLimiting.Params{
		IpLeakRate:        0.0000012,
		UserLeakRate:      0.0000012,
		IpCapacity:        1240,
		UserCapacity:      500,
		CleanPeriod:       cleanPeriodDur,
		MaxDuration:       maxDurationDur,
		IpWhitelistFile:   rlPref + "ip_whitelist2.txt",
		UserWhitelistFile: rlPref + "user_whitelist.txt",
	}

	gw := NewGatewayInstance(params)

	gw.Comms = gComm
	gw.ServerHost, err = connect.NewHost("test", NODE_ADDRESS,
		nodeCert, true, false)
	if err != nil {
		t.Errorf(err.Error())
	}

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
	gatewayInstance.ServerHost, _ = connect.NewHost("node", NODE_ADDRESS,
		nodeCert, true, false)

	_, err = gatewayInstance.Comms.Send(gatewayInstance.ServerHost, func(
		conn *grpc.ClientConn) (*any.
		Any, error) {
		_, err = pb.NewNodeClient(conn).AskOnline(ctx, &pb.Ping{})

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

// Calling initNetwork with permissioning enabled should get signed certs
func TestInitNetwork_GetSignedCert(t *testing.T) {
	defer disconnectServers()

	disablePermissioning = false
	noTLS = false

	ctx, cancel := connect.MessagingContext()

	_, err := gatewayInstance.Comms.Send(gatewayInstance.ServerHost, func(
		conn *grpc.ClientConn) (*any.
		Any, error) {
		_, err := pb.NewNodeClient(conn).AskOnline(ctx, &pb.Ping{})

		// Make sure there are no errors with sending the message
		if err != nil {
			return nil, err
		}
		return nil, nil
	})
	if err != nil {
		t.Errorf(err.Error())
	}

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

	msg := pb.Slot{SenderID: id.NewUserFromUint(255, t).Bytes()}
	err := gatewayInstance.PutMessage(&msg, "0")
	errMsg := ("PutMessage: Could not put any messages when IP address " +
		"should not be blocked")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(67, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "0")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(34, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "0")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(0, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "0")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(0, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "1")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(0, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "0")
	if err == nil {
		t.Errorf(errMsg)
	}

	time.Sleep(1 * time.Second)

	msg = pb.Slot{SenderID: id.NewUserFromUint(34, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "0")
	if err != nil {
		t.Errorf(errMsg)
	}

	time.Sleep(1 * time.Second)

	msg = pb.Slot{SenderID: id.NewUserFromUint(0, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "0")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(0, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "1")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(0, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "0")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(0, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "0")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(0, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "0")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(0, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "0")
	if err == nil {
		t.Errorf(errMsg)
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

	msg = pb.Slot{SenderID: id.NewUserFromUint(128, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "158.85.140.178")
	errMsg := ("PutMessage: Could not put any messages when IP " +
		"address should not be blocked")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(129, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "158.85.140.178")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(130, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "158.85.140.178")
	if err != nil {
		t.Errorf(errMsg)
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(131, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "158.85.140.178")
	if err != nil {
		t.Errorf(errMsg)
	}

	time.Sleep(1 * time.Second)

	msg = pb.Slot{SenderID: id.NewUserFromUint(132, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "158.85.140.178")
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages when " +
			"IP bucket is full but message IP is on whitelist")
	}
}

// Tests that messages can get through even when their bucket is full.
func TestGatewayImpl_PutMessage_UserWhitelist(t *testing.T) {
	var msg pb.Slot
	var err error

	msg = pb.Slot{SenderID: id.NewUserFromUint(174, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "aa")
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages when " +
			"IP address should not be blocked")
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(174, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "bb")
	if err != nil {
		t.Errorf("PutMessage: Could not put any messages when " +
			"IP address should not be blocked")
	}

	msg = pb.Slot{SenderID: id.NewUserFromUint(174, t).Bytes()}
	err = gatewayInstance.PutMessage(&msg, "cc")
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

func buildMockNdf(nodeId *id.Node, nodeAddress, gwAddress string, cert,
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
	_, err := gatewayInstance.Comms.AddHost(id.PERMISSIONING,
		"0.0.0.0:4200", pub, false, true)
	if err != nil {
		t.Errorf("Failed to add permissioning host: %+v", err)
	}

	nodeB := []byte{'n', 'o', 'd', 'e'}
	nodeId := id.NewNodeFromBytes(nodeB)
	ndf := buildMockNdf(nodeId, NODE_ADDRESS, GW_ADDRESS, nodeCert, nodeKey)

	ndfBytes, err := ndf.Marshal()
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
	nodeB := []byte{'n', 'o', 'd', 'e'}
	nodeId := id.NewNodeFromBytes(nodeB)
	ndf := buildMockNdf(nodeId, NODE_ADDRESS, GW_ADDRESS, nodeCert, nodeKey)

	ndfBytes, err := ndf.Marshal()
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

	slots := []*pb.Slot{mockMessage}

	update := &pb.ServerPollResponse{
		FullNDF:      ndfMsg,
		PartialNDF:   ndfMsg,
		Updates:      roundUpdates,
		BatchRequest: ri,
		Slots:        slots,
	}

	gatewayInstance.UpdateInstance(update)

	// Check that updates made it
	r, err := gatewayInstance.NetInf.GetRoundUpdate(1)
	if err != nil || r == nil {
		t.Errorf("Failed to retrieve round update: %+v", err)
	}

	// Check that mockMessage made it
	mockmsgId := "sHge1HVeKNiroYklqduPLYcy0TaSl3FVm/97P7ZhoLE="
	UserIDBytes := make([]byte, id.UserLen)
	UserIDBytes[0] = 1
	mockMsgUserId := id.NewUserFromBytes(UserIDBytes)
	msgTst, err := gatewayInstance.MixedBuffer.GetMixedMessage(mockMsgUserId,
		mockmsgId)
	if err != nil {
		t.Errorf("%v", err)
	}
	if msgTst == nil {
		t.Errorf("Did not return mock message!")
	}

	// Check that batchRequest was sent
	if len(nodeIncomingBatch.Slots) != 8 {
		t.Errorf("Did not send batch: %d", len(nodeIncomingBatch.Slots))
	}

}
