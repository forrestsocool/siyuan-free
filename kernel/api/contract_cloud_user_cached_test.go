// SiYuan - https://github.com/siyuan-note/siyuan
// Copyright (c) 2020-present, b3log.org
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/siyuan-note/siyuan/kernel/conf"
	"github.com/siyuan-note/siyuan/kernel/model"
	"github.com/siyuan-note/siyuan/kernel/util"
)

func TestAPIContractSettingCloudUserCached(t *testing.T) {
	previous, previousReadOnly := model.Conf, util.ReadOnly
	model.Conf = model.NewAppConf()
	util.ReadOnly = true
	t.Cleanup(func() { model.Conf, util.ReadOnly = previous, previousReadOnly })
	for _, user := range []*conf.User{nil, {UserId: "owner", UserTokenExpireTime: "0"}} {
		for _, role := range []model.Role{model.RoleAdministrator, model.RoleReader, model.RoleEditor} {
			model.Conf.SetUser(user)
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Set(model.RoleContextKey, role)
			context.Request = httptest.NewRequest("POST", "/api/setting/getCloudUser", strings.NewReader(`{"cached":true,"token":"ignored"}`))
			getCloudUser(context)
			requireAPIContract(t, "POST", "/api/setting/getCloudUser", recorder)
			var result struct {
				Code int `json:"code"`
				Data *struct {
					UserID        string  `json:"userId"`
					OneTimeStatus float64 `json:"userSiYuanOneTimePayStatus"`
				} `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Code != 0 {
				t.Fatalf("cached query failed: %s", recorder.Body.String())
			}
			if role == model.RoleAdministrator {
				if result.Data == nil {
					t.Fatal("cached account missing")
				}
				if user == nil {
					if result.Data.UserID != "0" || result.Data.OneTimeStatus != 1 {
						t.Fatal("missing cache did not restore the local paid account")
					}
				} else if result.Data.UserID != user.UserId {
					t.Fatal("existing cached account changed")
				}
			} else {
				if result.Data != nil {
					t.Fatal("unexpected account disclosure")
				}
				if model.Conf.GetUser() != user {
					t.Fatal("non-administrator request changed the cached account")
				}
			}
		}
	}
}
