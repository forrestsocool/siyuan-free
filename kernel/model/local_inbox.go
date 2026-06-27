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
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/88250/gulu"
	"github.com/88250/lute/parse"
	"github.com/siyuan-note/siyuan/kernel/util"
)

const localInboxPageSize = 32

var localInboxLock = sync.Mutex{}

type LocalShorthand struct {
	OID              string `json:"oId"`
	ShorthandTitle   string `json:"shorthandTitle"`
	ShorthandDesc    string `json:"shorthandDesc"`
	ShorthandContent string `json:"shorthandContent"`
	ShorthandMd      string `json:"shorthandMd"`
	ShorthandURL     string `json:"shorthandURL"`
	ShorthandFrom    int    `json:"shorthandFrom"`
	Created          int64  `json:"created"`
}

func AddLocalShorthand(title, md, url string) (ret map[string]any, err error) {
	localInboxLock.Lock()
	defer localInboxLock.Unlock()

	items, err := loadLocalShorthands()
	if err != nil {
		return
	}

	title = strings.TrimSpace(title)
	md = strings.TrimSpace(md)
	url = strings.TrimSpace(url)
	if "" == title {
		title = localShorthandTitle(md, url)
	}

	now := util.CurrentTimeMillis()
	id := strconv.FormatInt(now, 10)
	for localShorthandExists(items, id) {
		now++
		id = strconv.FormatInt(now, 10)
	}

	item := &LocalShorthand{
		OID:              id,
		ShorthandTitle:   title,
		ShorthandDesc:    localShorthandDesc(md, url),
		ShorthandContent: md,
		ShorthandMd:      md,
		ShorthandURL:     url,
		ShorthandFrom:    0,
		Created:          now,
	}
	items = append(items, item)
	sortLocalShorthands(items)
	if err = saveLocalShorthands(items); err != nil {
		return
	}
	ret = renderLocalShorthand(item)
	return
}

func RemoveLocalShorthands(ids []string) (err error) {
	localInboxLock.Lock()
	defer localInboxLock.Unlock()

	items, err := loadLocalShorthands()
	if err != nil {
		return
	}

	removeIDs := map[string]bool{}
	for _, id := range ids {
		removeIDs[id] = true
	}
	kept := []*LocalShorthand{}
	for _, item := range items {
		if !removeIDs[item.OID] {
			kept = append(kept, item)
		}
	}
	return saveLocalShorthands(kept)
}

func GetLocalShorthand(id string) (ret map[string]any, err error) {
	localInboxLock.Lock()
	defer localInboxLock.Unlock()

	items, err := loadLocalShorthands()
	if err != nil {
		return
	}
	for _, item := range items {
		if item.OID == id {
			ret = renderLocalShorthand(item)
			return
		}
	}
	ret = map[string]any{}
	return
}

func GetLocalShorthands(page int) (result map[string]any, err error) {
	localInboxLock.Lock()
	defer localInboxLock.Unlock()

	items, err := loadLocalShorthands()
	if err != nil {
		return
	}
	sortLocalShorthands(items)

	if 1 > page {
		page = 1
	}
	total := len(items)
	pageCount := total / localInboxPageSize
	if 0 != total%localInboxPageSize {
		pageCount++
	}
	if 1 > pageCount {
		pageCount = 1
	}
	start := (page - 1) * localInboxPageSize
	if start > total {
		start = total
	}
	end := start + localInboxPageSize
	if end > total {
		end = total
	}

	var shorthands []map[string]any
	for _, item := range items[start:end] {
		shorthands = append(shorthands, renderLocalShorthand(item))
	}
	if nil == shorthands {
		shorthands = []map[string]any{}
	}

	result = map[string]any{
		"data": map[string]any{
			"shorthands": shorthands,
			"pagination": map[string]any{
				"paginationPage":        page,
				"paginationPageCount":   pageCount,
				"paginationRecordCount": total,
				"paginationPageSize":    localInboxPageSize,
			},
		},
	}
	return
}

func localInboxPath() string {
	return filepath.Join(util.DataDir, "storage", "local-inbox.json")
}

func loadLocalShorthands() (ret []*LocalShorthand, err error) {
	p := localInboxPath()
	if !gulu.File.IsExist(p) {
		return []*LocalShorthand{}, nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	if err = gulu.JSON.UnmarshalJSON(data, &ret); err != nil {
		return
	}
	if nil == ret {
		ret = []*LocalShorthand{}
	}
	return
}

func saveLocalShorthands(items []*LocalShorthand) (err error) {
	p := localInboxPath()
	if err = os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return
	}
	data, err := gulu.JSON.MarshalIndentJSON(items, "", "  ")
	if err != nil {
		return
	}
	return os.WriteFile(p, data, 0644)
}

func sortLocalShorthands(items []*LocalShorthand) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].OID > items[j].OID
	})
}

func localShorthandExists(items []*LocalShorthand, id string) bool {
	for _, item := range items {
		if item.OID == id {
			return true
		}
	}
	return false
}

func localShorthandTitle(md, url string) string {
	if "" != md {
		for _, line := range strings.Split(md, "\n") {
			line = strings.TrimSpace(strings.Trim(line, "#>-*` "))
			if "" != line {
				return gulu.Str.SubStr(line, 80)
			}
		}
	}
	if "" != url {
		return url
	}
	return "Untitled"
}

func localShorthandDesc(md, url string) string {
	desc := strings.TrimSpace(md)
	if "" == desc {
		desc = url
	}
	desc = strings.ReplaceAll(desc, "\r\n", "\n")
	desc = strings.ReplaceAll(desc, "\n", " ")
	desc = strings.Join(strings.Fields(desc), " ")
	return gulu.Str.SubStr(desc, 160)
}

func renderLocalShorthand(item *LocalShorthand) map[string]any {
	md := item.ShorthandMd
	luteEngine := NewLute()
	luteEngine.SetFootnotes(true)
	tree := parse.Parse("", []byte(md), luteEngine.ParseOptions)
	luteEngine.RenderOptions.ProtyleMarkNetImg = false
	content := luteEngine.ProtylePreview(tree, luteEngine.RenderOptions, luteEngine.ParseOptions)

	created := item.Created
	if 0 == created {
		if parsed, err := strconv.ParseInt(item.OID, 10, 64); nil == err {
			created = parsed
		} else {
			created = time.Now().UnixMilli()
		}
	}
	hCreated := util.Millisecond2Time(created)

	return map[string]any{
		"oId":              item.OID,
		"shorthandTitle":   item.ShorthandTitle,
		"shorthandDesc":    item.ShorthandDesc,
		"shorthandContent": content,
		"shorthandMd":      md,
		"shorthandURL":     item.ShorthandURL,
		"shorthandFrom":    item.ShorthandFrom,
		"hCreated":         hCreated.Format("2006-01-02 15:04"),
	}
}
