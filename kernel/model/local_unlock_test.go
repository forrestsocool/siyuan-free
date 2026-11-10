// SiYuan - Refactor your thinking
// Copyright (c) 2020-present, b3log.org
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package model

import (
	"net/http"
	"testing"

	"github.com/siyuan-note/siyuan/kernel/conf"
	"github.com/siyuan-note/siyuan/kernel/util"
)

func withTestConf(t *testing.T) {
	t.Helper()

	oldConf := Conf
	oldReadOnly := util.ReadOnly
	Conf = NewAppConf()
	util.ReadOnly = true

	t.Cleanup(func() {
		Conf = oldConf
		util.ReadOnly = oldReadOnly
	})
}

func TestEnsureLocalUnlockUserCreatesPaidLocalUser(t *testing.T) {
	withTestConf(t)

	user := EnsureLocalUnlockUser()
	if nil == user {
		t.Fatal("expected local unlock user")
	}
	if user.UserToken != localUnlockUserToken {
		t.Fatalf("unexpected user token %q", user.UserToken)
	}
	if user.UserSiYuanProExpireTime != -1 {
		t.Fatalf("expected lifetime pro expiry, got %v", user.UserSiYuanProExpireTime)
	}
	if user.UserSiYuanOneTimePayStatus != 1 || user.UserSiYuanSubscriptionStatus != 0 {
		t.Fatalf("expected paid subscriber flags, got one-time=%v subscription=%v",
			user.UserSiYuanOneTimePayStatus, user.UserSiYuanSubscriptionStatus)
	}
	if !IsSubscriber() {
		t.Fatal("expected local unlock user to satisfy subscriber gate")
	}
	if !IsPaidUser() {
		t.Fatal("expected local unlock user to satisfy paid-user gate")
	}
	if "" == Conf.UserData {
		t.Fatal("expected encrypted user data to be persisted in memory")
	}

	loaded := loadUserFromConf()
	if nil == loaded || loaded.UserToken != localUnlockUserToken {
		t.Fatalf("expected encrypted user data to decode to local user, got %#v", loaded)
	}
}

func TestEnsureLocalUnlockUserKeepsExistingPaidUser(t *testing.T) {
	withTestConf(t)

	existing := &conf.User{
		UserName:                     "existing",
		UserSiYuanProExpireTime:      -1,
		UserSiYuanSubscriptionStatus: 0,
		UserSiYuanOneTimePayStatus:   1,
	}
	Conf.SetUser(existing)

	got := EnsureLocalUnlockUser()
	if got != existing {
		t.Fatalf("expected existing paid user to be preserved, got %#v", got)
	}
}

func TestRefreshUserUsesLocalUnlockUser(t *testing.T) {
	withTestConf(t)
	requests := 0
	mockCloudAuthResponse(t, func(request *http.Request) (*http.Response, error) {
		requests++
		return cloudAuthResponse(request, http.StatusServiceUnavailable, ""), nil
	})

	refreshed, err := RefreshUser("stale-cloud-token")
	if err != nil {
		t.Fatal(err)
	}

	user := Conf.GetUser()
	if nil == user {
		t.Fatal("expected refresh to install local unlock user")
	}
	if user.UserToken != localUnlockUserToken {
		t.Fatalf("expected local unlock token after refresh, got %q", user.UserToken)
	}
	if refreshed != user {
		t.Fatal("expected refreshed user to match the active local user")
	}
	if !IsSubscriber() {
		t.Fatal("expected refreshed local unlock user to satisfy subscriber gate")
	}
	if _, err = RefreshUser("another-expired-token"); err != nil || requests != 0 {
		t.Fatalf("local refresh contacted the community: requests=%d, err=%v", requests, err)
	}
}

func TestGetCloudUserUsesLocalUnlockUser(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(map[bool]string{false: "refresh", true: "cached"}[cached], func(t *testing.T) {
			withTestConf(t)
			if cached {
				EnsureLocalUnlockUser()
			}
			user, err := GetCloudUser("expired-cloud-token", cached)
			if err != nil || user == nil || user.UserToken != localUnlockUserToken {
				t.Fatalf("expected local user without a community request, got %#v, %v", user, err)
			}
		})
	}
}
