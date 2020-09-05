package lcache

import (
	"fmt"
	"log"
	"math/rand"
	"testing"
	"time"
	"unsafe"
)

var src = rand.NewSource(time.Now().UnixNano())

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	letterIdxBits = 8                    // 8 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func TestConsistentHash(t *testing.T) {
	var shards uint32 = 64
	iterations := 1000000
	expectedCount := float64(iterations) / float64(shards)
	fuzzPercent := 0.10
	delta := float64(expectedCount) * fuzzPercent
	tally := make([]int, shards)
	m := NewConcurrentMap(shards)

	for i := 0; i < iterations; i++ {
		// use static key length to rule out non-uniform distribution of key lengths which might affect hash distribution
		key := RandStringBytesMaskImprSrcUnsafe(32)
		hash := (m.(*cmap)).hash(key)
		tally[hash]++
	}

	outliers := 0
	for i, count := range tally {
		if float64(count) < expectedCount-delta || float64(count) > expectedCount+delta {
			println(fmt.Sprintf("[%d] expected count was %.0f but got %d", i, expectedCount, count))
			outliers++
		}
	}
	if outliers > 0 {
		log.Fatal(fmt.Sprintf("hash distribution was non-uniform: outliers=%d shards=%d iter=%d fuzz=%.2f",
			outliers, shards, iterations, fuzzPercent))
	}
}

// stolen from https://stackoverflow.com/a/31832326
func RandStringBytesMaskImprSrcUnsafe(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return *(*string)(unsafe.Pointer(&b))
}
