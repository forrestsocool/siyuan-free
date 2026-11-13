// SiYuan - https://github.com/siyuan-note/siyuan
// Copyright (c) 2020-present, b3log.org
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"testing"
	"time"

	"github.com/siyuan-note/siyuan/kernel/conf"
)

func TestCloudUserCachedDuringSync(t *testing.T) {
	withTestConf(t)
	for _, user := range []*conf.User{nil, {UserId: "owner", UserTokenExpireTime: "0"}} {
		Conf.SetUser(user)
		Conf.UserData = "preserved-account"
		release := lockAssetSourceChange()
		done := make(chan struct{})
		var got *conf.User
		var err error
		go func() {
			got, err = GetCloudUser("ignored-token", true)
			close(done)
		}()
		select {
		case <-done:
			release()
		case <-time.After(3 * time.Second):
			release()
			<-done
			t.Fatal("cached account query waited for synchronization")
		}
		if err != nil {
			t.Fatal(err)
		}
		if user == nil {
			if got == nil || got.UserToken != localUnlockUserToken || Conf.GetUser() != got {
				t.Fatal("cached query did not recover the missing local account during synchronization")
			}
		} else if got != user || Conf.GetUser() != user || Conf.UserData != "preserved-account" {
			t.Fatal("cached query changed the account or attempted to refresh it")
		}
	}
}
