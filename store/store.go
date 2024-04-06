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

package store

import "os"

var URL string
var Credentials string
var PictureName string

// Hostname of this host
var Hostname = "Unknown"

func init() {
	host, err := os.Hostname()
	if err == nil {
		Hostname = host
	}
}

// Store store record
type Store struct {
	Store []interface{}
}

// StoreResponse response information
type StoreResponse struct {
	// NrStored number of stored entries
	NrStored int64 `json:"NrStored,omitempty"`
	// Stored stored json
	Stored []int64 `json:"Stored"`
}
