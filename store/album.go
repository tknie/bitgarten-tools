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

package store

import "time"

// Picture picture description
type Picture struct {
	Index       int
	AlbumID     int
	Description string `adabas:"::PD"`
	Name        string `adabas:"::PN"`
	Md5         string `adabas:"::PM"`
	Interval    uint32 `adabas:"::PI"`
	MIMEType    string `adabas:"::MI"`
	Width       uint32 `adabas:"::WI"`
	Height      uint32 `adabas:"::HE"`
	Fill        string `adabas:"::PT"`
}

// Album album information
type Album struct {
	Index            uint64     `adabas:":isn"`
	Directory        string     `adabas:"::DI"`
	Published        time.Time  `adabas:"::DT"`
	Key              string     `adabas:"::KY"`
	Title            string     `adabas:"::TI"`
	AlbumDescription string     `adabas:"::TD"`
	Thumbnailhash    string     `adabas:"::TH"`
	Pictures         []*Picture `adabas:"::ET"`
}

// AlbumName name of map for album
var AlbumName string
