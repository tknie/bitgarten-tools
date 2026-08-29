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
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/tknie/bitgartentools/sql"
	"github.com/tknie/bitgartentools/store"
	"github.com/tknie/flynn/common"
	"github.com/tknie/services"
)

type typePoint int

const (
	sha1Measure typePoint = iota
	recordMeasure
)

var measurePoints = []string{
	"SHA1 checksum generation",
	"Record processing",
}

var wgHash sync.WaitGroup

const nrThreads = 5

var measure sync.Map
var counter = uint64(0)
var channel = make(chan *store.Pictures, 10)
var quit = make(chan bool, nrThreads)

const searchSha1Hash = `{{if not .Deleted -}} markdelete = false AND {{end}}
 {{.Filter}} sha1checksum IS NULL`

// GenerateImageHash generate image hash
func GenerateImageHash(parameter *ImageHashParameter) error {
	defer dumpMeasure()
	for i := 0; i < nrThreads; i++ {
		go workOnChannel(i)
	}
	if parameter.PreFilter != "" {
		parameter.PreFilter = fmt.Sprintf(" AND LOWER(title) LIKE '%s%%' AND ", parameter.PreFilter)
	}

	// Prepare template
	t1 := template.New("t1")
	t1 = template.Must(t1.Parse(searchSha1Hash))
	var sqlCmd bytes.Buffer
	t1.Execute(&sqlCmd, struct {
		Deleted bool
		All     bool
		Filter  string
	}{parameter.Deleted, parameter.All, parameter.PreFilter})

	id, err := sql.DatabaseHandler()
	if err != nil {
		return fmt.Errorf("POSTGRES error: %v", err)
	}
	query := &common.Query{
		TableName:  "pictures",
		Fields:     []string{"ChecksumPicture", "media"},
		DataStruct: &store.Pictures{},
		Limit:      strconv.Itoa(parameter.Limit),
		Search:     sqlCmd.String(),
	}
	if parameter.JSON {
		fmt.Printf("\"Hash\":[")
	}

	_, err = id.Query(query, workOnRecord)
	if err != nil {
		services.ServerMessage("Error query ...: %v %d", err, counter)
		return fmt.Errorf("query error: %v", err)
	}
	time.Sleep(10 * time.Second)
	services.ServerMessage("Finished generating SHA1 hash for %d pictures", counter)
	for i := 0; i < nrThreads; i++ {
		quit <- true
	}
	wgHash.Wait()
	return nil
}

func workOnRecord(search *common.Query, result *common.Result) error {
	defer measureTime(recordMeasure, 0, time.Now())
	counter++
	p := result.Data.(*store.Pictures)
	channel <- p
	if counter%1000 == 0 {
		services.ServerMessage("Generated SHA1 hash for %06d pictures", counter)
	}
	// fmt.Printf("{\"Checksum\":\"%s\",\"Title\":\"%s\",\"MIMEType\":\"%s\",\"Checksumpicture\":\"%s\"}\n",
	// 	p.ChecksumPicture, p.Title, p.MIMEType, hd)
	return nil
}

func workOnChannel(id int) {
	wgHash.Add(1)
	chanCounter := uint64(0)
	for {
		select {
		case p := <-channel:
			buffer := bytes.NewBuffer(p.Media)
			hd := generateSHA1(buffer)
			err := insertSha1Hash(p.ChecksumPicture, id, hd)
			if err != nil {
				os.Exit(111)
			}
			chanCounter++
		case <-quit:
			services.ServerMessage("Work channel %d counter = %d", id, chanCounter)
			wgHash.Done()
			return
		}
	}
}

func insertSha1Hash(checksumpicture string, id int, sha1checksum string) error {
	defer measureTime(sha1Measure, id, time.Now())
	wid, err := sql.DatabaseHandler()
	if err != nil {
		fmt.Println("POSTGRES error", err)
		return nil
	}
	insert := &common.Entries{
		Fields: []string{"sha1checksum"},
		Values: [][]any{{sha1checksum}},
		Update: []string{"checksumpicture='" + checksumpicture + "'"},
	}
	_, n, err := wid.Update("pictures", insert)
	if err != nil {
		fmt.Println("Error inserting", n, ":", err)
		fmt.Println("Pic:", checksumpicture)
		return err
	}

	return nil
}

func generateSHA1(buffer *bytes.Buffer) string {
	s := sha1.Sum(buffer.Bytes())
	//fmt.Printf("Generate SHA1 hash for picture %v %s\n", s, base64.StdEncoding.EncodeToString(s[:]))
	return base64.StdEncoding.EncodeToString(s[:])
}

func measureTime(tp typePoint, id int, t time.Time) {
	f := fmt.Sprintf("%d-%d", tp, id)
	if v, ok := measure.Load(f); ok {
		d := v.(time.Duration)
		measure.Store(f, d+time.Since(t))
	} else {
		measure.Store(f, time.Since(t))
	}
}

func dumpMeasure() {

	measure.Range(func(key, value any) bool {
		k := key.(string)
		v := value.(time.Duration)
		keys := strings.Split(k, "-")
		measureKey, _ := strconv.Atoi(keys[0])
		services.ServerMessage("Measure %v: %v", measurePoints[measureKey], v)
		return true
	})
}
