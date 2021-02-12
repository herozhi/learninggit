package pkg

import (
	"math/rand"
	"time"
)

// Retry backoff with jitter sleep to prevent overloaded conditions during intervals
func RetryBackoff(retry int, minBackoff, maxBackoff time.Duration) time.Duration {
	if retry < 0 {
		retry = 0
	}

	backoff := minBackoff << uint(retry)
	if backoff > maxBackoff || backoff < minBackoff {
		backoff = maxBackoff
	}

	if backoff == 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(backoff)))
}
