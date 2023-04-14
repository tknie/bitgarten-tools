package sql

import (
	"fmt"
	"sync"
	"time"

	"github.com/tknie/adabas-go-api/adatypes"
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
	ShortenName     bool
	ChecksumRun     bool
	Started         uint64
	StatInfo        [lastIndex + 1]statInfo
	checked         uint64
	skipped         uint64
	ToBig           uint64
	RequestBlobSize int64
	MaxBlobSize     int64
	Errors          map[string]uint64
	Filter          []string
	NrErrors        uint64
}

type timeInfo struct {
	startTime time.Time
}

var ps = &PictureConnection{Errors: make(map[string]uint64)}

const timeFormat = "2006-01-02 15:04:05"

var stopSchedule chan bool
var statLock sync.Mutex
var wgStat sync.WaitGroup

var output = func() {
	tn := time.Now().Format(timeFormat)
	fmt.Printf("%s statistics started=%02d checked=%02d skipped=%02d too big=%02d errors=%02d\n",
		tn, ps.Started, ps.checked, ps.skipped, ps.ToBig, ps.NrErrors)
	for i := 0; i < int(doneIndex)+1; i++ {
		avg := time.Duration(0)
		if ps.StatInfo[i].counter > 0 {
			avg = ps.StatInfo[i].duration / time.Duration(ps.StatInfo[i].counter)
		}
		fmt.Printf("%s statistics %18s -> counter=%04d duration=%v average=%v\n", tn, indexInfo[i],
			ps.StatInfo[i].counter, ps.StatInfo[i].duration, avg)
	}
	fmt.Printf("%s statistics max Blocksize=%s deferred Blocksize=%v\n",
		tn, ByteCountBinary(ps.MaxBlobSize), ByteCountBinary(ps.RequestBlobSize))
	fmt.Printf("--------------------------------------------------------------\n")
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

func IncStarted() *timeInfo {
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

func (di *timeInfo) used(index int) {
	ps.StatInfo[index].counter++
	ps.StatInfo[index].duration += time.Since(di.startTime)
}
