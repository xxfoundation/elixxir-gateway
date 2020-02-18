////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package notifications

import (
	"bytes"
	"gitlab.com/elixxir/primitives/id"
	"testing"
)

// unit test for notify function
func TestUserNotifications_Notify(t *testing.T) {
	un := UserNotifications{}
	un.Notify(id.NewUserFromBytes([]byte("test")))
	if len(un.ids) != 1 && bytes.Compare(un.ids[0].Bytes(), []byte("test")) != 0 {
		t.Errorf("Failed to properly add user notification")
	}

	un.Notify(id.NewUserFromBytes([]byte("test")))
	if len(un.ids) != 1 {
		t.Errorf("Number of ids should still be one, since id is the same")
	}
}

// Unit test for notified function
func TestUserNotifications_Notified(t *testing.T) {
	un := UserNotifications{}
	un.Notify(id.NewUserFromBytes([]byte("test")))
	ret := un.Notified()
	if len(ret) != 1 && ret[0] != "test" {
		t.Error("Did not properly return list of ids")
	}
	if un.ids != nil {
		t.Error("Did not clear IDs after returning")
	}
}
