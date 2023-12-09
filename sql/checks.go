/*
* Copyright © 2023 private, Darmstadt, Germany and/or its licensors
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
	"log"
	"tux-lobload/store"
)

const query = `with p(checksumpicture,sha256checksum) as (
	select
		checksumpicture,sha256checksum
	from
		pictures
	where
		checksumpicture = $1)
	select
		l.picturedirectory,
		l.picturename
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
		''
	from
		p
	`

func (di *DatabaseInfo) CheckExists(pic *store.Pictures) {
	pic.Available = store.NoAvailable
	rows, err := di.db.Query(query, pic.ChecksumPicture, pic.Directory, pic.PictureName, hostname)
	if err != nil {
		fmt.Println("Query error:", err)
		log.Fatalf("Query error database call...%v", err)
		return
	}
	defer rows.Close()

	dir := ""
	name := ""
	for rows.Next() {
		pic.Available = store.PicAvailable
		err := rows.Scan(&dir, &name)
		if err != nil {
			log.Fatal("Error scanning read location check")
		}

		switch {
		case name == "" && dir != pic.ChecksumPictureSHA:
			fmt.Println("SHA mismatch", pic.PictureName)
			err := fmt.Errorf("SHA mismatch %s/%s", pic.Directory, pic.PictureName)
			IncError("SHA differs "+pic.PictureName, err)
			pic.Available = store.NoAvailable
		case name != "" && dir == pic.Directory && name == pic.PictureName:
			pic.Available = store.BothAvailable
		default:
		}
	}
}
