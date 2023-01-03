package store

import (
	"fmt"
	"sync"
)

// WorkerType worker function
type WorkerType func(job string)

// Worker worker function
type Worker struct {
	nrWorkers int
	Jobs      chan string
	End       chan bool
	Listener  WorkerType
	wg        sync.WaitGroup
}

// InitWorker init worker functions
func InitWorker(nrWorker int, l WorkerType) *Worker {
	wr := &Worker{nrWorkers: nrWorker,
		Jobs:     make(chan string, nrWorker),
		End:      make(chan bool, 1),
		Listener: l}
	fmt.Println("Init ", nrWorker)

	wr.wg.Add(nrWorker)
	for w := 1; w <= nrWorker; w++ {
		go wr.workerFunc(w)
	}
	return wr
}

func (wr *Worker) workerFunc(w int) {
	for {
		select {
		case p := <-wr.Jobs:
			wr.Listener(p)
		case <-wr.End:
			wr.wg.Done()
			return
		}
	}
}

// WaitEnd wait for end of worker
func (wr *Worker) WaitEnd() {
	for i := 0; i < wr.nrWorkers; i++ {
		wr.End <- true
	}
	wr.wg.Wait()
}
