/*
* Copyright © 2023-2024 private, Darmstadt, Germany and/or its licensors
*
* SPDX-License-Identifier: Apache-2.0
*
*   Licensed under the Apache License, Version 2.0 (the "License");
*   you may not use this file except in compliance with the License.
*   You may obtain a copy of the License at
*
*       http://www.apache.org/licenses/LICENSE-2.0
*
*   Unless required by applicable law or agreed to in writing, software
*   distributed under the License is distributed on an "AS IS" BASIS,
*   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*   See the License for the specific language governing permissions and
*   limitations under the License.
*
 */

package sql

import (
	"fmt"

	"github.com/docker/go-units"
	"github.com/tknie/bitgartentools/store"

	"github.com/tknie/log"

	"github.com/tknie/flynn/common"
)

const query = `with p(checksumpicture,sha256checksum,picopt,medialen) as (
	select
		checksumpicture,sha256checksum,picopt,length(media)
	from
		pictures
	where
		checksumpicture = $1)
	select
		l.picturedirectory,
		l.picturename, picopt,medialen
	from
		picturelocations l,
		p
	where
		l.checksumpicture = p.checksumpicture
		and l.picturename = $3
		and l.picturedirectory = $2
		and l.picturehost = $4
	union
	select
		p.sha256checksum,
		'','',medialen
	from
		p
	`

var MaxBlobSize int64

func init() {
	MaxBlobSize, _ = units.FromHumanSize("1GB")
}

func (di *DatabaseInfo) CheckExists(pic *store.Pictures) {
	pic.Available = store.NoAvailable

	batch := &common.Query{TableName: "pictures", Search: query,
		Parameters: []any{pic.ChecksumPicture, pic.Directory, pic.PictureName, store.Hostname}}
	err := di.id.BatchSelectFct(batch, func(search *common.Query, result *common.Result) error {
		pic.Available = store.PicAvailable
		dir := result.Rows[0].(string)
		name := result.Rows[1].(string)
		picopt := ""
		if result.Rows[2] != nil {
			picopt = result.Rows[2].(string)
		}
		mediaLength := result.Rows[3].(int32)
		log.Log.Debugf("%s dir=%s,name=%s,picopt=%s mediaLen=%d",
			pic.ChecksumPicture, dir, name, picopt, mediaLength)

		switch {
		case name == "" && dir != pic.ChecksumPictureSHA:
			fmt.Printf("SHA mismatch <%s> <name=%s> <dir=%s> <chksum=%s>\n", pic.PictureName, name, dir, pic.ChecksumPicture)
			err := fmt.Errorf("SHA mismatch %s/%s", pic.Directory, pic.PictureName)
			IncError("SHA differs "+pic.PictureName, err)
			pic.Available = store.NoAvailable
		case name != "" && dir == pic.Directory && name == pic.PictureName:
			log.Log.Debugf("Media %s -> %d", picopt, mediaLength)
			if picopt != "webstore" {
				pic.Available = store.BothAvailable
			}
		default:
			log.Log.Debugf("%s small size default check name=%s dir=%s sha=%s", pic.ChecksumPicture, name, dir, pic.ChecksumPictureSHA)
		}
		return nil
	})
	if err != nil {
		fmt.Println("Check exists query error:", err)
		log.Log.Fatalf("Query error database call...%v", err)
		return
	}
	log.Log.Debugf("%s: Current available %s", pic.ChecksumPicture, pic.Available)
	if int64(len(pic.Media)) > MaxBlobSize && pic.Available != store.BothAvailable {
		log.Log.Debugf("Check REST client ... size bigger than %d", MaxBlobSize)
		found, err := CheckRestClient(pic.ChecksumPicture)
		if err != nil {
			log.Log.Fatalf("REST client check failed")
		}
		if !found {
			log.Log.Debugf("Maximal blob size...no available for %s", pic.ChecksumPicture)
			pic.Available = store.ToBigNoAvailable
		} else {
			log.Log.Debugf("Maximal blob size...both available for %s", pic.ChecksumPicture)
			pic.Available = store.BothAvailable
		}
	}
	log.Log.Debugf("%s: Final available %s", pic.ChecksumPicture, pic.Available)
}
