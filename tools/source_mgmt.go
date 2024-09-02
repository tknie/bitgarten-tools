/*
* Copyright © 2024 private, Darmstadt, Germany and/or its licensors
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
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tknie/log"
	"gopkg.in/yaml.v2"
)

type scan struct {
	Directories []string `yaml:"directories"`
}

// ReadConfig read config file
func ReadScanFile(file string) ([]byte, error) {
	scanFile, err := os.Open(file)
	if err != nil {
		log.Log.Debugf("Open file error: %#v", err)
		return nil, fmt.Errorf("open file err of %s: %v", file, err)
	}
	defer scanFile.Close()

	fi, _ := scanFile.Stat()
	log.Log.Debugf("File size=%d", fi.Size())
	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, scanFile)
	if err != nil {
		log.Log.Debugf("Read file error: %#v", err)
		return nil, fmt.Errorf("read file err of %s: %v", file, err)
	}
	return buffer.Bytes(), nil
}

func EvaluatePictureDirectories() (directories []string, err error) {
	// Check for environment variable
	e := os.Getenv("BITGARTEN_DIRECTORIES")
	if e != "" {
		directories = strings.Split(e, ",")
		return directories, nil
	}

	scan := &scan{}
	byteValue, err := ReadScanFile("scan.yaml")
	if err != nil {
		return nil, err
	}
	// Check if yaml is present
	err = yaml.Unmarshal(byteValue, scan)
	if err != nil {
		log.Log.Debugf("Unmarshal error: %#v", err)
		return nil, err
	}
	for i := 0; i < len(scan.Directories); i++ {
		scan.Directories[i] = os.ExpandEnv(scan.Directories[i])
	}
	return scan.Directories, nil
}
