// SiYuan - https://github.com/siyuan-note/siyuan
// Copyright (c) 2020-present, b3log.org
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"bytes"
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
			requests := 0
			mockCloudAuthResponse(t, func(request *http.Request) (*http.Response, error) {
				requests++
				return cloudAuthResponse(request, http.StatusServiceUnavailable, ""), nil
			})
			repoKey := []byte("existing-repository-key")
			Conf.Repo.Store(&conf.Repo{Key: append([]byte(nil), repoKey...)})
			user, err := GetCloudUser("expired-cloud-token", cached)
			if err != nil || user == nil || user.UserToken != localUnlockUserToken {
				t.Fatalf("expected local user without a community request, got %#v, %v", user, err)
			}
			if requests != 0 || !IsPaidUser() {
				t.Fatalf("local account recovery failed: community requests=%d, paid=%t", requests, IsPaidUser())
			}
			if !bytes.Equal(Conf.Repo.Load().Key, repoKey) {
				t.Fatal("recovering the local account changed the repository key")
			}
			cachedUser, err := GetCloudUser("ignored-token", true)
			if err != nil || cachedUser != user {
				t.Fatal("cached account recovery did not retain the active user")
			}
		})
	}
}

func TestGetCloudUserUsesLocalUnlockUserPreservesCachedAccount(t *testing.T) {
	withTestConf(t)
	existing := &conf.User{UserId: "existing-account", UserTokenExpireTime: "0"}
	Conf.SetUser(existing)
	requests := 0
	mockCloudAuthResponse(t, func(request *http.Request) (*http.Response, error) {
		requests++
		return cloudAuthResponse(request, http.StatusServiceUnavailable, ""), nil
	})

	got, err := GetCloudUser("ignored-token", true)
	if err != nil || got != existing || requests != 0 {
		t.Fatalf("cached account was replaced or refreshed: same=%t, requests=%d, err=%v", got == existing, requests, err)
	}
}
