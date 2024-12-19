package kvtest

import (
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Like previous labs, you must write some tests on your own.
// Add your test cases in this file and submit them as extra_test.go.
// You must add at least 5 test cases, though you can add as many as you like.
//
// You can use any type of test already used in this lab: server
// tests, client tests, or integration tests.
//
// You can also write unit tests of any utility functions you have in utils.go
//
// Tests are run from an external package, so you are testing the public API
// only. You can make methods public (e.g. utils) by making them Capitalized.

func TestLargeKeyValue(t *testing.T) {
	setup := MakeTestSetup(MakeBasicOneShard())

	largeValue := ""
	func(n int) {
		const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		b := make([]byte, n)
		for i := range b {
			b[i] = letterBytes[rand.Intn(len(letterBytes))]
		}
		largeValue = string(b)
	}(1024 * 1024 * 10) // 10 MB string

	err := setup.Set("large-key", largeValue, 5*time.Second)
	assert.Nil(t, err)

	val, wasFound, err := setup.Get("large-key")
	assert.Nil(t, err)
	assert.True(t, wasFound)
	assert.Equal(t, largeValue, val)

	setup.Shutdown()
}

func TestConcurrentSetDeleteSameKey(t *testing.T) {
	setup := MakeTestSetup(MakeBasicOneShard())
	key := "thisisakey"
	var wg sync.WaitGroup
	goros := runtime.NumCPU() * 2
	iters := 100

	for i := 0; i < goros; i++ {
		wg.Add(2)

		go func() {
			for j := 0; j < iters; j++ {
				err := setup.Set(key, "value", 5*time.Second)
				assert.Nil(t, err)
			}
			wg.Done()
		}()

		go func() {
			for j := 0; j < iters; j++ {
				err := setup.Delete(key)
				assert.Nil(t, err)
			}
			wg.Done()
		}()
	}

	wg.Wait()
	setup.Shutdown()
}

func TestPartialShardUnavailability(t *testing.T) {
	setup := MakeTestSetup(MakeTwoNodeBothAssignedSingleShard())
	err := setup.Set("thisisakey", "thisisavalue", 5*time.Second)
	assert.Nil(t, err)

	setup.nodes["n1"].Shutdown() // n1 is now unavailable but n2 is still available

	val, wasFound, err := setup.Get("thisisakey")
	assert.Nil(t, err)
	assert.True(t, wasFound)
	assert.Equal(t, "thisisavalue", val)
}

func TestStressTTLExpiration(t *testing.T) {
	setup := MakeTestSetup(MakeBasicOneShard())
	keys := RandomKeys(1000, 10)

	for _, key := range keys {
		err := setup.Set(key, "thiskeywillexpire", 100*time.Millisecond)
		assert.Nil(t, err)
	}

	time.Sleep(500 * time.Millisecond)

	for _, key := range keys {
		_, wasFound, err := setup.Get(key)
		assert.Nil(t, err)
		assert.False(t, wasFound)
	}
}

func TestRapidKeyExpirationWithGet(t *testing.T) {
	setup := MakeTestSetup(MakeBasicOneShard())
	key := "thiskeywillexpire"

	err := setup.Set(key, "value", 100*time.Millisecond)
	assert.Nil(t, err)

	for i := 0; i < 75; i++ {
		time.Sleep(20 * time.Millisecond)
		_, wasFound, _ := setup.Get(key)
		if i > 5 {
			assert.False(t, wasFound)
		} else if i < 4 {
			assert.True(t, wasFound)
		}
	}

	setup.Shutdown()
}

func TestLoadSpikeHandling(t *testing.T) {
	setup := MakeTestSetup(MakeBasicOneShard())
	key := "thisisakey"
	var wg sync.WaitGroup
	goros := runtime.NumCPU() * 4
	iters := 2000

	for i := 0; i < goros; i++ {
		wg.Add(2)

		go func() {
			for j := 0; j < iters; j++ {
				err := setup.Set(key, "value", 1*time.Second)
				assert.Nil(t, err)
			}
			wg.Done()
		}()

		go func() {
			for j := 0; j < iters; j++ {
				_, _, err := setup.Get(key)
				assert.Nil(t, err)
			}
			wg.Done()
		}()
	}

	wg.Wait()
	setup.Shutdown()
}

func TestHighVolumeShardMigration(t *testing.T) {
	setup := MakeTestSetup(MakeFourNodesWithFiveShards())
	keys := RandomKeys(500, 20)
	vals := RandomKeys(500, 40)

	for i := 0; i < 500; i++ {
		err := setup.Set(keys[i], vals[i], 10*time.Second)
		assert.Nil(t, err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		for i := 0; i < 75; i++ {
			err := setup.MoveRandomShard()
			assert.Nil(t, err)
			time.Sleep(100 * time.Millisecond)
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 500; i++ {
			for _, key := range keys {
				_, _, err := setup.Get(key)
				assert.Nil(t, err)
			}
		}
		wg.Done()
	}()

	wg.Wait()
	setup.Shutdown()
}

func TestConcurrentLargeGetSetDelete(t *testing.T) {
	setup := MakeTestSetup(MakeMultiShardSingleNode())
	keys := RandomKeys(1000, 10)

	var wg sync.WaitGroup
	goros := runtime.NumCPU() * 4

	for i := 0; i < goros; i++ {
		wg.Add(3)

		go func() {
			for _, key := range keys {
				err := setup.Set(key, "value", 10*time.Second)
				assert.Nil(t, err)
			}
			wg.Done()
		}()

		go func() {
			for _, key := range keys {
				_, _, err := setup.Get(key)
				assert.Nil(t, err)
			}
			wg.Done()
		}()

		go func() {
			for _, key := range keys {
				err := setup.Delete(key)
				assert.Nil(t, err)
			}
			wg.Done()
		}()
	}

	wg.Wait()
	setup.Shutdown()
}
