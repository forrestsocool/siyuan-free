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
	"github.com/88250/gulu"
	"github.com/siyuan-note/siyuan/kernel/conf"
	"github.com/siyuan-note/siyuan/kernel/util"
)

const localUnlockUserToken = "siyuan-local-unlock"

// EnsureLocalUnlockUser provides a local paid user for self-hosted builds so
// existing S3/WebDAV/Local sync feature gates can use the normal code paths.
func EnsureLocalUnlockUser() *conf.User {
	user := Conf.GetUser()
	if nil != user && 1 == user.UserSiYuanOneTimePayStatus && 0 == user.UserSiYuanSubscriptionStatus {
		return user
	}

	user = &conf.User{
		UserId:                          "0",
		UserName:                        "local-unlock",
		UserAvatarURL:                   "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7",
		UserHomeBImgURL:                 "",
		UserTitles:                      []*conf.UserTitle{},
		UserIntro:                       "",
		UserNickname:                    "Local Unlock",
		UserCreateTime:                  "29991231 00:00:00",
		UserSiYuanProExpireTime:         -1,
		UserToken:                       localUnlockUserToken,
		UserTokenExpireTime:             "32503593600",
		UserSiYuanRepoSize:              1099511627776,
		UserSiYuanPointExchangeRepoSize: 0,
		UserSiYuanAssetSize:             0,
		UserTrafficUpload:               0,
		UserTrafficDownload:             0,
		UserTrafficAPIGet:               0,
		UserTrafficAPIPut:               0,
		UserTrafficTime:                 0,
		UserSiYuanSubscriptionPlan:      0,
		UserSiYuanSubscriptionStatus:    0,
		UserSiYuanSubscriptionType:      1,
		UserSiYuanOneTimePayStatus:      1,
	}

	Conf.SetUser(user)
	data, _ := gulu.JSON.MarshalJSON(user)
	Conf.UserData = util.AESEncrypt(string(data))
	Conf.Save()
	return user
}
