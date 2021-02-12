package idgenerator

import (
	"errors"
	"math/rand"
	"time"
)

const (
	int64Max uint64 = 9223372036854775807

	BM4 = ((1 << 4) - 1)
	BM6 = ((1 << 6) - 1)
	BM8 = ((1 << 8) - 1)
)

// LocalGen 生成的id有可能冲突
// idgenerator: 32b timestamp + 18b counter + 6b countspace + 4b reserved + 4b serverid
// localGen:    32b timestamp + 18b rand + 000000 + 0000 + 4b rand
func LocalGen() (int64, error) {
	now := uint64(time.Now().Unix())
	r := uint64(rand.Uint32())
	idUint64 := now<<32 | (r >> 14 << 14) | r&BM4
	if idUint64 > int64Max {
		return 0, errors.New("id exceeded error")
	}
	return int64(idUint64), nil
}

func SetCountSpace(id int64, cs uint32) (int64, bool) {
	if cs >= 64 {
		return id, false
	}

	idUint64 := uint64(id)
	if (idUint64 >> 52) == 0 {
		return id, false
	}

	idUint64 = (idUint64 >> 14 << 14) | uint64(cs&BM6)<<8 | idUint64&BM8
	return int64(idUint64), true
}
