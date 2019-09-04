////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"fmt"
	"github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/comms/connect"
	"gitlab.com/elixxir/comms/gateway"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/node"
	"gitlab.com/elixxir/comms/testkeys"
	"gitlab.com/elixxir/primitives/format"
	"gitlab.com/elixxir/primitives/id"
	"gitlab.com/elixxir/primitives/utils"
	"os"
	"reflect"
	"testing"
	"time"
)

const GW_ADDRESS = "0.0.0.0:5555"
const NODE_ADDRESS = "0.0.0.0:5556"

var gatewayInstance *Instance
var gComm *gateway.GatewayComms
var n *node.NodeComms

var mockMessage *pb.Slot

var nodeIncomingBatch *pb.Batch

// This sets up a dummy/mock globals instance for testing purposes
func TestMain(m *testing.M) {

	//Begin gateway comms
	cmixNodes := make([]string, 1)
	cmixNodes[0] = GW_ADDRESS

	gComm = gateway.StartGateway(GW_ADDRESS, gatewayInstance, nil, nil)

	//Start mock node
	nodeHandler := buildTestNodeImpl()
	n = node.StartNode(NODE_ADDRESS, nodeHandler, nil, nil)

	//Connect gateway comms to node
	err := gComm.ConnectToRemote(connectionID(NODE_ADDRESS), NODE_ADDRESS, nil, true)
	if err != nil {
		fmt.Println("Could not connect to node")
	}

	grp := make(map[string]string)
	grp["prime"] = "9DB6FB5951B66BB6FE1E140F1D2CE5502374161FD6538DF1648218642F0B5C48" +
		"C8F7A41AADFA187324B87674FA1822B00F1ECF8136943D7C55757264E5A1A44F" +
		"FE012E9936E00C1D3E9310B01C7D179805D3058B2A9F4BB6F9716BFE6117C6B5" +
		"B3CC4D9BE341104AD4A80AD6C94E005F4B993E14F091EB51743BF33050C38DE2" +
		"35567E1B34C3D6A5C0CEAA1A0F368213C3D19843D0B4B09DCB9FC72D39C8DE41" +
		"F1BF14D4BB4563CA28371621CAD3324B6A2D392145BEBFAC748805236F5CA2FE" +
		"92B871CD8F9C36D3292B5509CA8CAA77A2ADFC7BFD77DDA6F71125A7456FEA15" +
		"3E433256A2261C6A06ED3693797E7995FAD5AABBCFBE3EDA2741E375404AE25B"
	grp["generator"] = "5C7FF6B06F8F143FE8288433493E4769C4D988ACE5BE25A0E24809670716C613" +
		"D7B0CEE6932F8FAA7C44D2CB24523DA53FBE4F6EC3595892D1AA58C4328A06C4" +
		"6A15662E7EAA703A1DECF8BBB2D05DBE2EB956C142A338661D10461C0D135472" +
		"085057F3494309FFA73C611F78B32ADBB5740C361C9F35BE90997DB2014E2EF5" +
		"AA61782F52ABEB8BD6432C4DD097BC5423B285DAFB60DC364E8161F4A2A35ACA" +
		"3A10B1C4D203CC76A470A33AFDCBDD92959859ABD8B56E1725252D78EAC66E71" +
		"BA9AE3F1DD2487199874393CD4D832186800654760E1E34C09E4D155179F9EC0" +
		"DC4473F996BDCE6EED1CABED8B6F116F7AD9CF505DF0F998E34AB27514B0FFE7"
	grp["smallprime"] = "F2C3119374CE76C9356990B465374A17F23F9ED35089BD969F61C6DDE9998C1F"

	//Build the gateway instance
	params := Params{
		BatchSize:   1,
		GatewayNode: NODE_ADDRESS,
		CMixNodes:   cmixNodes,
		CmixGrp:     grp,
	}
	gatewayInstance = NewGatewayInstance(params)
	gatewayInstance.Comms = gComm

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

	defer gComm.Shutdown()
	defer n.Shutdown()
	os.Exit(m.Run())
}

func buildTestNodeImpl() *node.Implementation {
	nodeHandler := node.NewImplementation()
	nodeHandler.Functions.GetRoundBufferInfo = func() (int, error) {
		return 1, nil
	}
	nodeHandler.Functions.PostNewBatch = func(batch *pb.Batch) error {
		nodeIncomingBatch = batch
		return nil
	}
	nodeHandler.Functions.GetCompletedBatch = func() (*pb.Batch, error) {
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

	return nodeHandler
}

//Tests that receiving messages and sending them to the node works
func TestGatewayImpl_SendBatch(t *testing.T) {
	msg := pb.Slot{SenderID: id.NewUserFromUint(666, t).Bytes()}

	ok := gatewayInstance.PutMessage(&msg)
	if !ok {
		t.Errorf("PutMessage: Could not put any messages!")
	}

	junkMsg := GenJunkMsg(gatewayInstance.CmixGrp, 1)
	t.Logf("kmac for junk: %v", junkMsg.KMACs)
	gatewayInstance.SendBatchWhenReady(1, junkMsg)

	time.Sleep(1 * time.Second)

	if nodeIncomingBatch == nil {
		t.Errorf("Batch not recieved by node!")
	} else {
		t.Logf("kmac is the following: %v", nodeIncomingBatch.Slots[1].KMACs)

		if !reflect.DeepEqual(nodeIncomingBatch.Slots[0].SenderID, msg.SenderID) {
			t.Errorf("Message in batch not the same as sent;"+
				"\n  Expected: %+v \n  Recieved: %+v", msg, *nodeIncomingBatch.Slots[0])
		}
	}
}

func TestGatewayImpl_PollForBatch(t *testing.T) {
	// Call PollForBatch and make sure it doesn't explode... setup done in main
	gatewayInstance.PollForBatch()
}

// Calling InitNetwork after starting a node should cause
// gateway to connect to the node
func TestInitNetwork_ConnectsToNode(t *testing.T) {

	const gwPort = 6555
	const nodeAddress = "0.0.0.0:6556"
	disablePermissioning = true
	serverCertPath := testkeys.GetNodeCertPath()
	grp := make(map[string]string)
	grp["prime"] = "9DB6FB5951B66BB6FE1E140F1D2CE5502374161FD6538DF1648218642F0B5C48" +
		"C8F7A41AADFA187324B87674FA1822B00F1ECF8136943D7C55757264E5A1A44F" +
		"FE012E9936E00C1D3E9310B01C7D179805D3058B2A9F4BB6F9716BFE6117C6B5" +
		"B3CC4D9BE341104AD4A80AD6C94E005F4B993E14F091EB51743BF33050C38DE2" +
		"35567E1B34C3D6A5C0CEAA1A0F368213C3D19843D0B4B09DCB9FC72D39C8DE41" +
		"F1BF14D4BB4563CA28371621CAD3324B6A2D392145BEBFAC748805236F5CA2FE" +
		"92B871CD8F9C36D3292B5509CA8CAA77A2ADFC7BFD77DDA6F71125A7456FEA15" +
		"3E433256A2261C6A06ED3693797E7995FAD5AABBCFBE3EDA2741E375404AE25B"
	grp["generator"] = "5C7FF6B06F8F143FE8288433493E4769C4D988ACE5BE25A0E24809670716C613" +
		"D7B0CEE6932F8FAA7C44D2CB24523DA53FBE4F6EC3595892D1AA58C4328A06C4" +
		"6A15662E7EAA703A1DECF8BBB2D05DBE2EB956C142A338661D10461C0D135472" +
		"085057F3494309FFA73C611F78B32ADBB5740C361C9F35BE90997DB2014E2EF5" +
		"AA61782F52ABEB8BD6432C4DD097BC5423B285DAFB60DC364E8161F4A2A35ACA" +
		"3A10B1C4D203CC76A470A33AFDCBDD92959859ABD8B56E1725252D78EAC66E71" +
		"BA9AE3F1DD2487199874393CD4D832186800654760E1E34C09E4D155179F9EC0" +
		"DC4473F996BDCE6EED1CABED8B6F116F7AD9CF505DF0F998E34AB27514B0FFE7"
	grp["smallprime"] = "F2C3119374CE76C9356990B465374A17F23F9ED35089BD969F61C6DDE9998C1F"

	params := Params{
		Port:           gwPort,
		CertPath:       "",
		KeyPath:        "",
		GatewayNode:    (connectionID)(nodeAddress),
		ServerCertPath: serverCertPath,
		CmixGrp:        grp,
	}

	gw := NewGatewayInstance(params)

	cert, err := utils.ReadFile(testkeys.GetNodeCertPath())
	if err != nil {
		t.Errorf("Failed to read cert file: %+v", err)
	}
	key, err := utils.ReadFile(testkeys.GetNodeKeyPath())
	if err != nil {
		t.Errorf("Failed to read key file: %+v", err)
	}

	_ = node.StartNode(nodeAddress, node.NewImplementation(),
		cert, key)

	gw.InitNetwork()

	connId := connectionID(nodeAddress)
	nodeComms := gw.Comms.GetNodeConnection(connId)

	ctx, cancel := connect.MessagingContext()

	_, err = nodeComms.AskOnline(ctx, &pb.Ping{}, grpc_retry.WithMax(connect.DefaultMaxRetries))

	// Make sure there are no errors with sending the message
	if err != nil {
		err = errors.New(err.Error())
		jww.ERROR.Printf("AskOnline: Error received: %+v", err)
	}

	cancel()

}
