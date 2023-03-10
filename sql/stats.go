package sql

import (
	"fmt"
	"sync"
	"time"
)

type PictureConnection struct {
	ShortenName bool
	ChecksumRun bool
	Found       uint64
	Empty       uint64
	Loaded      uint64
	Duplicate   uint64
	Commited    uint64
	Checked     uint64
	ToBig       uint64
	Errors      map[string]uint64
	Filter      []string
	NrErrors    uint64
	NrDeleted   uint64
	Ignored     uint64
	MaxBlobSize int64
}

var ps = &PictureConnection{Errors: make(map[string]uint64)}

const timeFormat = "2006-01-02 15:04:05"

var stopSchedule chan bool
var statLock sync.Mutex
var wgStat sync.WaitGroup

func StartStats() {

	output := func() {
		fmt.Printf("%s Picture directory checked=%02d loaded=%02d found=%02d too big=%02d errors=%02d deleted=%02d\n",
			time.Now().Format(timeFormat), ps.Checked, ps.Loaded, ps.Found, ps.ToBig, ps.NrErrors, ps.NrDeleted)
		fmt.Printf("%s Picture directory empty=%02d ignored=%02d  max Blocksize=%02d\n",
			time.Now().Format(timeFormat), ps.Empty, ps.Ignored, ps.MaxBlobSize)
	}

	schedule(output, 5*time.Second)

}

func EndStats() {
	fmt.Println("Trigger ending...")
	wgStat.Add(1)
	stopSchedule <- true
	fmt.Println("Waiting ending...")
	wgStat.Wait()
	fmt.Printf("%s Done Picture directory checked=%d loaded=%d found=%d too big=%d empty=%d ignored=%d errors=%d\n",
		time.Now().Format(timeFormat), ps.Checked, ps.Loaded, ps.Found, ps.ToBig, ps.Empty, ps.Ignored, ps.NrErrors)
	for e, n := range ps.Errors {
		fmt.Println(e, ":", n)
	}

}

func schedule(what func(), delay time.Duration) {
	stopSchedule = make(chan bool)

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

func RegisterBlobSize(blobSize int64) {
	statLock.Lock()
	defer statLock.Unlock()
	if blobSize > ps.MaxBlobSize {
		ps.MaxBlobSize = blobSize
	}
}

func IncDuplicate() {
	ps.Duplicate++
	ps.Found++
}

func IncChecked() {
	ps.Checked++
}

func IncInsert() {
	ps.Loaded++
}

func IncCommit() {
	ps.Commited++
}

func IncIgnore() {
	ps.Ignored++
}

func IncToBig() {
	ps.ToBig++
}

func IncError(err error) {
	ps.NrErrors++
	if err == nil {
		return
	}
	if e, ok := ps.Errors[err.Error()]; ok {
		ps.Errors[err.Error()] = e + 1
		return
	}
	ps.Errors[err.Error()] = 1
}

func IncErrorFile(err error, fileName string) {
	ps.NrErrors++
	if err == nil {
		return
	}
	if e, ok := ps.Errors[fileName+"->"+err.Error()]; ok {
		ps.Errors[fileName+"->"+err.Error()] = e + 1
		return
	}
	ps.Errors[fileName+"->"+err.Error()] = 1
}
