package rand

import (
	impl "math/rand"
	"runtime"
	"sync"
	"time"

	pid "github.com/choleraehyq/pid"
)

const (
	cacheLineSize = 64
)

var (
	shardsLen int
)

type lockedSource struct {
	_ [cacheLineSize]byte
	sync.Mutex
	*impl.Rand
}

func (ls *lockedSource) Intn(n int) (r int) {
	ls.Lock()
	r = ls.Rand.Intn(n)
	ls.Unlock()
	return
}

type Rand []*lockedSource

func init() {
	shardsLen = runtime.GOMAXPROCS(0)
	defaultRand = NewRand()
}

func NewRand() Rand {
	s := make([]*lockedSource, shardsLen)
	for i := 0; i < shardsLen; i++ {
		s[i] = &lockedSource{
			Rand: impl.New(impl.NewSource(time.Now().UnixNano())),
		}
	}
	return s
}

func (r Rand) Intn(n int) int {
	return r[pid.GetPid()%shardsLen].Intn(n)
}

var defaultRand Rand

func Intn(n int) int {
	return defaultRand.Intn(n)
}
