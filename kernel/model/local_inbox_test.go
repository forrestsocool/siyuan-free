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
	"os"
	"strings"
	"testing"

	"github.com/siyuan-note/siyuan/kernel/conf"
	"github.com/siyuan-note/siyuan/kernel/util"
)

func withLocalInboxTestState(t *testing.T) {
	t.Helper()

	oldConf := Conf
	oldDataDir := util.DataDir
	oldReadOnly := util.ReadOnly

	Conf = NewAppConf()
	Conf.Editor = conf.NewEditor()
	Conf.Export = conf.NewExport()
	util.DataDir = t.TempDir()
	util.ReadOnly = true

	t.Cleanup(func() {
		Conf = oldConf
		util.DataDir = oldDataDir
		util.ReadOnly = oldReadOnly
	})
}

func TestLocalInboxAddListGetAndRemove(t *testing.T) {
	withLocalInboxTestState(t)

	created, err := AddLocalShorthand("", "# Captured title\n\nBody text", "https://example.com/article")
	if err != nil {
		t.Fatal(err)
	}
	id, ok := created["oId"].(string)
	if !ok || "" == id {
		t.Fatalf("expected generated oId, got %#v", created["oId"])
	}
	if created["shorthandTitle"] != "Captured title" {
		t.Fatalf("unexpected generated title %q", created["shorthandTitle"])
	}
	if created["shorthandMd"] != "# Captured title\n\nBody text" {
		t.Fatalf("expected original markdown to be retained, got %q", created["shorthandMd"])
	}
	if !strings.Contains(created["shorthandContent"].(string), "Captured title") {
		t.Fatalf("expected rendered content, got %q", created["shorthandContent"])
	}

	list, err := GetLocalShorthands(1)
	if err != nil {
		t.Fatal(err)
	}
	data := list["data"].(map[string]any)
	shorthands := data["shorthands"].([]map[string]any)
	if len(shorthands) != 1 {
		t.Fatalf("expected one shorthand, got %d", len(shorthands))
	}
	pagination := data["pagination"].(map[string]any)
	if pagination["paginationRecordCount"] != 1 {
		t.Fatalf("unexpected record count %#v", pagination["paginationRecordCount"])
	}

	got, err := GetLocalShorthand(id)
	if err != nil {
		t.Fatal(err)
	}
	if got["shorthandURL"] != "https://example.com/article" {
		t.Fatalf("unexpected URL %q", got["shorthandURL"])
	}

	if err = RemoveLocalShorthands([]string{id}); err != nil {
		t.Fatal(err)
	}
	list, err = GetLocalShorthands(1)
	if err != nil {
		t.Fatal(err)
	}
	data = list["data"].(map[string]any)
	shorthands = data["shorthands"].([]map[string]any)
	if len(shorthands) != 0 {
		t.Fatalf("expected empty inbox after remove, got %d", len(shorthands))
	}

	raw, err := os.ReadFile(localInboxPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(raw)) != "[]" {
		t.Fatalf("expected empty inbox to persist as [], got %q", raw)
	}
}

func TestLocalInboxPaginationUsesNewestFirst(t *testing.T) {
	withLocalInboxTestState(t)

	for i := 0; i < localInboxPageSize+1; i++ {
		if _, err := AddLocalShorthand("", "item "+string(rune('A'+i)), ""); err != nil {
			t.Fatal(err)
		}
	}

	firstPage, err := GetLocalShorthands(1)
	if err != nil {
		t.Fatal(err)
	}
	firstData := firstPage["data"].(map[string]any)
	firstItems := firstData["shorthands"].([]map[string]any)
	if len(firstItems) != localInboxPageSize {
		t.Fatalf("expected first page size %d, got %d", localInboxPageSize, len(firstItems))
	}
	firstPagination := firstData["pagination"].(map[string]any)
	if firstPagination["paginationPageCount"] != 2 {
		t.Fatalf("expected two pages, got %#v", firstPagination["paginationPageCount"])
	}

	secondPage, err := GetLocalShorthands(2)
	if err != nil {
		t.Fatal(err)
	}
	secondData := secondPage["data"].(map[string]any)
	secondItems := secondData["shorthands"].([]map[string]any)
	if len(secondItems) != 1 {
		t.Fatalf("expected one item on second page, got %d", len(secondItems))
	}

	if firstItems[0]["oId"].(string) <= secondItems[0]["oId"].(string) {
		t.Fatalf("expected newest item on first page before oldest second-page item")
	}
}
