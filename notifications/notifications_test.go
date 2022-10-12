////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package notifications

import (
	"bytes"
	"gitlab.com/xx_network/primitives/id"
	"testing"
)

// unit test for notify function
func TestUserNotifications_Notify(t *testing.T) {
	un := UserNotifications{}
	un.Notify(id.NewIdFromBytes([]byte("test"), t))
	if len(un.ids) != 1 && bytes.Compare(un.ids[0].Bytes(), []byte("test")) != 0 {
		t.Errorf("Failed to properly add user notification")
	}

	un.Notify(id.NewIdFromBytes([]byte("test"), t))
	if len(un.ids) != 1 {
		t.Errorf("Number of ids should still be one, since id is the same")
	}
}

// Unit test for notified function
func TestUserNotifications_Notified(t *testing.T) {
	un := UserNotifications{}
	un.Notify(id.NewIdFromBytes([]byte("test"), t))
	ret := un.Notified()
	if len(ret) != 1 && ret[0] != id.NewIdFromBytes([]byte("test"), t) {
		t.Error("Did not properly return list of ids")
	}
	if un.ids != nil {
		t.Error("Did not clear IDs after returning")
	}
}
