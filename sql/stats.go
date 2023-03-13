package sql

import (
	"fmt"
	"sync"
	"time"

	"github.com/tknie/adabas-go-api/adatypes"
)

type PictureConnection struct {
	ShortenName       bool
	ChecksumRun       bool
	Started           uint64
	Empty             uint64
	Loaded            uint64
	Duplicate         uint64
	DuplicateLocation uint64
	Commited          uint64
	Checked           uint64
	ToBig             uint64
	RequestBlobSize   int64
	MaxBlobSize       int64
	Errors            map[string]uint64
	Filter            []string
	NrErrors          uint64
	NrDeleted         uint64
	Ignored           uint64
}

var ps = &PictureConnection{Errors: make(map[string]uint64)}

const timeFormat = "2006-01-02 15:04:05"

var stopSchedule chan bool
var statLock sync.Mutex
var wgStat sync.WaitGroup

var output = func() {
	fmt.Printf("%s Picture directory checked=%02d loaded=%02d duplicate=%02d too big=%02d errors=%02d deleted=%02d\n",
		time.Now().Format(timeFormat), ps.Checked, ps.Loaded, ps.Duplicate, ps.ToBig, ps.NrErrors, ps.NrDeleted)
	fmt.Printf("%s Picture directory started=%02d empty=%02d ignored=%02d  duplicate Location=%02d commited=%02d\n",
		time.Now().Format(timeFormat), ps.Started, ps.Empty, ps.Ignored, ps.DuplicateLocation, ps.Commited)
	fmt.Printf("%s Picture directory max Blocksize=%02d deferred Blocksize=%02d\n",
		time.Now().Format(timeFormat), ps.MaxBlobSize, ps.RequestBlobSize)
}

func StartStats() {

	schedule(output, 5*time.Second)

}

func EndStats() {
	fmt.Println("Trigger ending...")
	wgStat.Add(1)
	stopSchedule <- true
	fmt.Println("Waiting ending...")
	wgStat.Wait()

	fmt.Printf("%s Done\n", time.Now().Format(timeFormat))
	output()
	for e, n := range ps.Errors {
		fmt.Println(e, ":", n)
		adatypes.Central.Log.Errorf("%03d -> %s", n, e)
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

func IncDuplicate() {
	ps.Duplicate++
}

func IncDuplicateLocation() {
	ps.DuplicateLocation++
}

func IncStarted() {
	ps.Started++
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
	adatypes.Central.Log.Errorf("Increase error for %s: %v", fileName, err)
	if e, ok := ps.Errors[fileName+"->"+err.Error()]; ok {
		ps.Errors[fileName+"->"+err.Error()] = e + 1
		return
	}
	ps.Errors[fileName+"->"+err.Error()] = 1
}
