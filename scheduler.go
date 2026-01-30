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
package bitgartentools

import (
	"fmt"
	"sync"
	"time"
)

var stopSchedule chan bool
var syncSchedule chan bool

var wgStat sync.WaitGroup

func ScheduleParameter(what func(start time.Time, parameter interface{}), parameter interface{}, delay time.Duration) {
	stopSchedule = make(chan bool)
	syncSchedule = make(chan bool)
	startTime := time.Now()
	go func() {
		for {
			select {
			case <-time.After(delay):
			case <-stopSchedule:
				what(startTime, parameter)
				syncSchedule <- true
				return
			}
			what(startTime, parameter)
		}
	}()

}

func Schedule(what func(), delay time.Duration) {
	stopSchedule = make(chan bool)
	go func() {
		for {
			select {
			case <-time.After(delay):
			case <-stopSchedule:
				wgStat.Done()
				return
			}
			what()
		}
	}()

}

func EndStats(fct func()) {
	fmt.Println("Trigger ending...")
	wgStat.Add(1)
	stopSchedule <- true
	fmt.Println("Waiting ending...")
	wgStat.Wait()

	fmt.Printf("%s Done\n", time.Now().Format(TimeFormat))
	fct()
}

func StopScheduler() {
	stopSchedule <- true

	<-syncSchedule

}
