package basic

import (
	"strconv"
	"sync"
)

type ClOrdIDGenerator struct {
	// Prefix is prepended to every ClOrdID so IDs stay unique across restarts.
	Prefix      string
	clOrdIDLock sync.Mutex
	clOrdID     int
}

func (f *ClOrdIDGenerator) Next() string {
	f.clOrdIDLock.Lock()
	defer f.clOrdIDLock.Unlock()

	f.clOrdID++
	return f.Prefix + strconv.Itoa(f.clOrdID)
}
