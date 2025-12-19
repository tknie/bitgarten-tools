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
	"errors"
	"fmt"
	"os"

	"github.com/tknie/flynn"
	"github.com/tknie/flynn/common"
	"github.com/tknie/log"
)

func DatabaseConnect() (*DatabaseInfo, error) {
	sourceUrl := os.Getenv("POSTGRES_URL")
	pwd := os.Getenv("POSTGRES_PASSWORD")
	// fmt.Println("Connect : " + sourceUrl)
	connSource, err := Connect(sourceUrl, pwd)
	if err != nil {
		fmt.Println("Error opening connection:", err)
		fmt.Println("Set POSTGRES_URL and/or POSTGRES_PASSWORD to define remote database")
		return nil, err
	}
	return connSource, nil
}

func DatabaseLocation() (*common.Reference, string, error) {
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		fmt.Println("POSTGRES_URL not defined")
		return nil, "", errors.New("POSTGRES_URL not defined")
	}

	ref, passwd, err := common.NewReference(url)
	if err != nil {
		fmt.Println("POSTGRES_URL parser error:", err)
		return nil, "", err
	}
	if passwd == "" {
		passwd = os.Getenv("POSTGRES_PASSWORD")
	}
	return ref, passwd, nil
}

func DatabaseHandler() (common.RegDbID, error) {
	ref, passwd, err := DatabaseLocation()
	if err != nil {
		return 0, err
	}
	log.Log.Debugf("Connect to %s:%d", ref.Host, ref.Port)
	id, err := flynn.Handler(ref, passwd)
	if err != nil {
		fmt.Println("Error opening connection:", err)
		return 0, err
	}
	return id, nil
}
