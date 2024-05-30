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
	"sync"
	"time"

	"github.com/tknie/log"
)

type indexType int

const (
	loadIndex indexType = iota
	loadedIndex
	insertedIndex
	endStoreIndex
	duplicateIndex
	duplicateLocationIndex
	commitedIndex
	doneIndex
)

const lastIndex = doneIndex

var indexInfo = []string{"load", "loaded", "inserted", "end store",
	"duplicate", "duplicateLocation", "commited", "done"}

type statInfo struct {
	counter  uint64
	duration time.Duration
}

type PictureConnection struct {
	start           time.Time
	ShortenName     bool
	ChecksumRun     bool
	Started         uint64
	StatInfo        [lastIndex + 1]statInfo
	checked         uint64
	skipped         uint64
	ToBig           uint64
	RequestBlobSize int64
	MaxBlobSize     int64
	Errors          sync.Map
	Filter          []string
	NrErrors        uint64
}

type timeInfo struct {
	startTime time.Time
}

type workerState byte

const (
	InitStoreWorker workerState = iota
	LoadingStoreWorker
	WaitingStoreWorker
	DoneStoreWorker
	StopStoreWorker
	Done2StoreWorker
)

var workerStates = []string{"init", "loading", "waiting", "done", "stop", "done2"}

func (ws workerState) String() string {
	return workerStates[ws]
}

type storeWorkerStatistic struct {
	state       workerState
	currentFile string
}

var storeWorkerStatistics []storeWorkerStatistic
var readerWorkerStatistics []storeWorkerStatistic

func InitStoreWorkerStatistics(nrThreadReader int) {
	storeWorkerStatistics = make([]storeWorkerStatistic, nrThreadReader)
}

func InitReaderWorkerStatistics(nrThreadReader int) {
	readerWorkerStatistics = make([]storeWorkerStatistic, nrThreadReader)
}

func SetState(index int, state workerState) {
	storeWorkerStatistics[index].state = state
	storeWorkerStatistics[index].currentFile = ""
}

func SetStateWithFile(index int, state workerState, filename string) {
	SetState(index, state)
	storeWorkerStatistics[index].currentFile = filename
}

func SetReaderState(index int, state workerState) {
	readerWorkerStatistics[index].state = state
	readerWorkerStatistics[index].currentFile = ""
}

func SetReaderStateWithFile(index int, state workerState, filename string) {
	SetReaderState(index, state)
	readerWorkerStatistics[index].currentFile = filename
}

var ps = &PictureConnection{}

const timeFormat = "2006-01-02 15:04:05"

var stopSchedule chan bool
var statLock sync.Mutex
var wgStat sync.WaitGroup

const maxNrCount = 4

var similarCount = maxNrCount
var lastChecked uint64

var output = func() {
	if ps.checked != 0 && ps.checked == lastChecked {
		fmt.Printf("Waiting counter = %04d\n", similarCount)
		for i, sws := range storeWorkerStatistics {
			fmt.Printf("%d. store worker thread works in state '%s': %s\n", i, sws.state, sws.currentFile)
		}
		for i, sws := range readerWorkerStatistics {
			fmt.Printf("%d. reader worker thread works in state '%s': %s\n", i, sws.state, sws.currentFile)
		}
		similarCount--
		if similarCount < 0 {
			log.Log.Fatal("similarity count triggered, not processing images!!!")
		}
	} else {
		similarCount = maxNrCount
	}
	lastChecked = ps.checked
	tn := time.Now().Format(timeFormat)
	fmt.Printf("%s statistics started=%05d checked=%05d skipped=%02d too big=%02d errors=%02d\n",
		tn, ps.Started, ps.checked, ps.skipped, ps.ToBig, ps.NrErrors)
	log.Log.Infof("%s statistics started=%05d checked=%05d skipped=%02d too big=%02d errors=%02d\n",
		tn, ps.Started, ps.checked, ps.skipped, ps.ToBig, ps.NrErrors)
	for i := 0; i < int(doneIndex)+1; i++ {
		avg := time.Duration(0)
		if ps.StatInfo[i].counter > 0 {
			avg = ps.StatInfo[i].duration / time.Duration(ps.StatInfo[i].counter)
		}
		avg = avg.Round(time.Second)
		fmt.Printf("%s statistics %18s -> counter=%04d duration=%v average=%v\n", tn, indexInfo[i],
			ps.StatInfo[i].counter, ps.StatInfo[i].duration.Round(time.Second), avg)
		log.Log.Infof("%s statistics %18s -> counter=%04d duration=%v average=%v", tn, indexInfo[i],
			ps.StatInfo[i].counter, ps.StatInfo[i].duration.Round(time.Second), avg)
	}
	fmt.Printf("%s statistics max Blocksize=%s deferred Blocksize=%v\n",
		tn, ByteCountBinary(ps.MaxBlobSize), ByteCountBinary(ps.RequestBlobSize))
	log.Log.Infof("%s statistics max Blocksize=%s deferred Blocksize=%v\n",
		tn, ByteCountBinary(ps.MaxBlobSize), ByteCountBinary(ps.RequestBlobSize))
	log.Log.Infof("--------------------------------------------------------------\n")
}

func ByteCountBinary(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func StartStats() {

	schedule(output, 15*time.Second)

}

func EndStats() {
	fmt.Println("Trigger ending...")
	wgStat.Add(1)
	stopSchedule <- true
	fmt.Println("Waiting ending...")
	wgStat.Wait()

	fmt.Printf("%s Done\n", time.Now().Format(timeFormat))
	output()
	ps.Errors.Range(func(e, n any) bool {
		fmt.Println("Error:", e, ":", n)
		log.Log.Errorf("End stats %03d -> %s", n, e)
		return true
	})
}

func schedule(what func(), delay time.Duration) {
	stopSchedule = make(chan bool)
	ps.start = time.Now()
	go func() {
		for {
			what()
			select {
			case <-time.After(delay):
			case <-stopSchedule:
				wgStat.Done()
				return
			}
		}
	}()

}

func DeferredBlobSize(blobSize int64) {
	if blobSize > ps.RequestBlobSize {
		ps.RequestBlobSize = blobSize
	}
}

func RegisterBlobSize(blobSize int64) {
	statLock.Lock()
	defer statLock.Unlock()
	if blobSize > ps.MaxBlobSize {
		ps.MaxBlobSize = blobSize
	}
}

func IncStored() *timeInfo {
	return &timeInfo{startTime: time.Now()}
}

func (di *timeInfo) IncLoaded() {
	di.used(int(loadedIndex))
}

func (di *timeInfo) IncEndStored() {
	di.used(int(endStoreIndex))
}

func (di *timeInfo) IncDuplicate() {
	di.used(int(duplicateIndex))
}

func (di *timeInfo) IncDuplicateLocation() {
	di.used(int(duplicateLocationIndex))
}

func (ps *PictureConnection) IncStarted() *timeInfo {
	ps.Started++
	return &timeInfo{startTime: time.Now()}
}

func IncChecked() *timeInfo {
	ps.checked++
	return &timeInfo{startTime: time.Now()}
}

func (di *timeInfo) IncDone() {
	di.used(int(doneIndex))
}

func (di *timeInfo) IncInserted() {
	di.used(int(insertedIndex))
}

func (di *timeInfo) IncInsert() {
	di.used(int(loadIndex))
}

func (di *timeInfo) IncCommit() {
	di.used(int(commitedIndex))
}

func IncToBig() {
	ps.ToBig++

}

func IncSkipped() {
	ps.skipped++
}

func IncError(prefix string, err error) {
	ps.NrErrors++
	if err == nil {
		return
	}
	if e, ok := ps.Errors.Load(prefix + ":" + err.Error()); ok {
		ps.Errors.Store(err.Error(), e.(uint64)+1)
		return
	}
	ps.Errors.Store(prefix+":"+err.Error(), uint64(1))
}

func IncErrorFile(err error, fileName string) {
	ps.NrErrors++
	if err == nil {
		return
	}
	log.Log.Errorf("Increase error for %s: %v", fileName, err)
	if e, ok := ps.Errors.Load(fileName + "->" + err.Error()); ok {
		ps.Errors.Store(fileName+"->"+err.Error(), e.(uint64)+1)
		return
	}
	ps.Errors.Store(fileName+"->"+err.Error(), uint64(1))
}

func (di *timeInfo) used(index int) {
	ps.StatInfo[index].counter++
	ps.StatInfo[index].duration += time.Since(di.startTime)
}
