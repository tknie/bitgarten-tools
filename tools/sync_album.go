/*
* Copyright © 2018-2026 private, Darmstadt, Germany and/or its licensors
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

package tools

import (
	"crypto/md5"
	"fmt"
	"os"

	"github.com/tknie/bitgartentools/sql"
)

type SyncAlbumParameter struct {
	Title       string
	ListSource  bool
	ListDest    bool
	InsertAlbum bool
	SyncAll     bool
	SkipCheck   bool
}

func SyncAlbum(parameter *SyncAlbumParameter) error {
	var a *sql.Albums
	connSource, err := sql.DatabaseConnect()
	if err != nil {
		fmt.Println("Error creating connection:", err)
		fmt.Println("Set POSTGRES_URL and/or POSTGRES_PASSWORD to define remote database")
		return err
	}
	destUrl := os.Getenv("POSTGRES_DESTINATION_URL")
	pwd := os.Getenv("POSTGRES_DESTINATION_PASSWORD")
	destSource, err := sql.Connect(destUrl, pwd)
	if err != nil {
		fmt.Println("Error creating connection:", err)
		fmt.Println("Set POSTGRES_DESTINATION_URL and/or POSTGRES_DESTINATION_PASSWORD to define remote database")
		return err
	}
	copyList := make([]string, 0)
	if parameter.Title != "" {
		copyList = append(copyList, parameter.Title)
	}
	switch {
	case parameter.ListSource || parameter.SyncAll:
		l, err := connSource.ListAlbums()
		if err != nil {
			fmt.Println("List albums error:", err)
			return err
		}
		if len(copyList) == 0 {
			if !parameter.SyncAll {
				return nil
			}
			copyList = l
		}
	case parameter.ListDest:
		_, err = destSource.ListAlbums()
		if err != nil {
			fmt.Println("List albums error:", err)
			return err
		}
		return nil
	default:
	}
	for _, t := range copyList {
		a, err = connSource.ReadAlbum(t)
		if err != nil {
			fmt.Println("Error reading album:", err)
			return err
		}
		if a != nil {
			a.Display()
			for _, p := range a.Pictures {
				f, err := destSource.CheckPicture(p.ChecksumPicture)
				if err != nil {
					fmt.Println("Error checking picature:", err)
					return err
				}
				if !f {
					fmt.Println("Not in destination database, picture", p.ChecksumPicture, f)
					err := copyPicture(connSource, destSource, p.ChecksumPicture,
						parameter.SkipCheck)
					if err != nil {
						fmt.Println("Error copying picture:", err)
						return err
					}
				}
			}
			err = destSource.WriteAlbum(a)
			if err != nil {
				fmt.Println("Error writing album:", err)
				return err
			}
			for _, ap := range a.Pictures {
				err = destSource.WriteAlbumPictures(ap)
				if err != nil {
					fmt.Println("Error writing album pictures:", err)
					return err
				}
			}
		}
	}
	return nil
}

func copyPicture(connSource, destSource *sql.DatabaseInfo, checksum string, skipCheck bool) error {
	p, err := connSource.ReadPicture(checksum)
	if err != nil {
		fmt.Println("Error reading picture:", err)
		return err
	}
	c := fmt.Sprintf("%X", md5.Sum(p.Media))
	if !skipCheck && p.ChecksumPicture != c {
		return fmt.Errorf("checksum mismatch: %s != %s(%d) %v", p.ChecksumPicture, c, len(c), skipCheck)
	}
	fmt.Println("Successful read picture", p.ChecksumPicture, p.Created)
	err = destSource.WritePicture(p)
	if err != nil {
		fmt.Println("Error writing picture:", err)
		return err
	}
	return nil
}
