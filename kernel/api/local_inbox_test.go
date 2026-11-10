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
	"github.com/siyuan-note/siyuan/kernel/apicontract"
	"github.com/siyuan-note/siyuan/kernel/conf"
	"github.com/siyuan-note/siyuan/kernel/model"
	"github.com/siyuan-note/siyuan/kernel/util"
)

func localInboxAPIState(t *testing.T) {
	t.Helper()
	previousConf, previousDir, previousReadOnly := model.Conf, util.DataDir, util.ReadOnly
	model.Conf = model.NewAppConf()
	model.Conf.Editor.Store(conf.NewEditor())
	model.Conf.Export.Store(conf.NewExport())
	util.DataDir, util.ReadOnly = t.TempDir(), true
	t.Cleanup(func() {
		model.Conf, util.DataDir, util.ReadOnly = previousConf, previousDir, previousReadOnly
	})
}

func TestAPIContractLocalInbox(t *testing.T) {
	localInboxAPIState(t)
	engine := gin.New()
	engine.POST("/api/inbox/addShorthand", addShorthand)
	engine.POST("/api/inbox/getShorthand", getShorthand)
	engine.POST("/api/inbox/getShorthands", getShorthands)
	engine.POST("/api/inbox/removeShorthands", removeShorthands)
	call := func(path, body string) []byte {
		t.Helper()
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest("POST", path, strings.NewReader(body)))
		requireAPIContract(t, "POST", path, recorder)
		return recorder.Body.Bytes()
	}
	var created struct {
		Code int
		Data *apicontract.Shorthand
	}
	if err := json.Unmarshal(call("/api/inbox/addShorthand", `{"title":"Capture","content":"# Body","url":"https://example.com"}`), &created); err != nil {
		t.Fatal(err)
	}
	if created.Code != 0 || created.Data == nil || created.Data.ID == "" || created.Data.Markdown != "# Body" {
		t.Fatalf("capture failed: %#v", created)
	}
	call("/api/inbox/getShorthand", `{"id":"`+created.Data.ID+`"}`)
	var list struct {
		Data *apicontract.ShorthandsData
	}
	if err := json.Unmarshal(call("/api/inbox/getShorthands", `{"page":1}`), &list); err != nil {
		t.Fatal(err)
	}
	if list.Data == nil || len(list.Data.Data.Shorthands) != 1 || len(list.Data.Data.Pagination.PageNums) != 1 {
		t.Fatalf("unexpected local inbox page: %#v", list.Data)
	}
	call("/api/inbox/removeShorthands", `{"ids":["`+created.Data.ID+`"]}`)
	if err := json.Unmarshal(call("/api/inbox/getShorthands", `{"page":1}`), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Data.Data.Shorthands) != 0 {
		t.Fatal("deleted item is still present")
	}
	if response := string(call("/api/inbox/addShorthand", `{"title":123}`)); !strings.Contains(response, `"code":-1`) {
		t.Fatalf("invalid title accepted: %s", response)
	}
}

func TestAPIContractLocalCloudUser(t *testing.T) {
	localInboxAPIState(t)
	engine := gin.New()
	role := model.RoleAdministrator
	engine.Use(func(c *gin.Context) { c.Set(model.RoleContextKey, role); c.Next() })
	engine.POST("/api/setting/getCloudUser", getCloudUser)
	for _, body := range []string{`{"token":"expired","cached":false}`, `{"cached":true}`} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest("POST", "/api/setting/getCloudUser", strings.NewReader(body)))
		requireAPIContract(t, "POST", "/api/setting/getCloudUser", recorder)
		if !strings.Contains(recorder.Body.String(), `"userName":"local-unlock"`) {
			t.Fatalf("expected local account: %s", recorder.Body.String())
		}
	}
	role = model.RoleReader
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest("POST", "/api/setting/getCloudUser", strings.NewReader(`{"cached":true}`)))
	requireAPIContract(t, "POST", "/api/setting/getCloudUser", recorder)
	if !strings.Contains(recorder.Body.String(), `"data":null`) {
		t.Fatalf("reader received account data: %s", recorder.Body.String())
	}
}
