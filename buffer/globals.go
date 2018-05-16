////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// global variable for the message buffer
package buffer

// Global instance of the in-memory Message Buffer
var GlobalMessageBuffer MessageBuffer = newMessageBuffer()

// The Address of the cMix nodes to communicate with
var CMIX_NODES []string

// The address of my cMix Node
var GATEWAY_NODE string

// The batch size of the cMix network
var BATCH_SIZE uint64
