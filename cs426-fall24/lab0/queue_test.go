package lab0_test

import (
	"context"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"fmt"

	"cs426.cloud/lab0"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

type intQueue interface {
	Push(i int)
	Pop() (int, bool)
}

type stringQueue interface {
	Push(s string)
	Pop() (string, bool)
}

func runIntQueueTests(t *testing.T, q intQueue) {
	t.Run("queue starts empty", func(t *testing.T) {
		_, ok := q.Pop()
		require.False(t, ok)
	})

	t.Run("simple push pop", func(t *testing.T) {
		q.Push(1)
		q.Push(2)
		q.Push(3)

		x, ok := q.Pop()
		require.True(t, ok)
		require.Equal(t, 1, x)

		x, ok = q.Pop()
		require.True(t, ok)
		require.Equal(t, 2, x)

		x, ok = q.Pop()
		require.True(t, ok)
		require.Equal(t, 3, x)
	})

	t.Run("queue empty again", func(t *testing.T) {
		_, ok := q.Pop()
		require.False(t, ok)
	})

	t.Run("push and pop", func(t *testing.T) {
		for i := 1; i <= 10; i++ {
			q.Push(i)
		}
		for i := 1; i <= 5; i++ {
			x, ok := q.Pop()
			require.True(t, ok)
			require.Equal(t, i, x)
		}

		for i := 11; i <= 100; i++ {
			q.Push(i)
		}

		x, ok := q.Pop()
		require.True(t, ok)
		require.Equal(t, 6, x)

		q.Push(0)
		for i := 7; i <= 100; i++ {
			x, ok := q.Pop()
			require.True(t, ok)
			require.Equal(t, i, x)
		}
		x, ok = q.Pop()
		require.True(t, ok)
		require.Equal(t, 0, x)

		_, ok = q.Pop()
		require.False(t, ok)

		_, ok = q.Pop()
		require.False(t, ok)
	})

	// My own tests
	t.Run("alternating push and pop", func(t *testing.T) {
		q.Push(1)
		x, ok := q.Pop()
		require.True(t, ok)
		require.Equal(t, 1, x)

		q.Push(2)
		q.Push(3)
		x, ok = q.Pop()
		require.True(t, ok)
		require.Equal(t, 2, x)

		q.Push(4)
		x, ok = q.Pop()
		require.True(t, ok)
		require.Equal(t, 3, x)

		x, ok = q.Pop()
		require.True(t, ok)
		require.Equal(t, 4, x)

		_, ok = q.Pop()
		require.False(t, ok)
	})

	t.Run("push duplicate values", func(t *testing.T) {
		val := 1

		funcPop := func() {
			x, ok := q.Pop()
			require.True(t, ok)
			require.Equal(t, val, x)
		}

		q.Push(val)
		q.Push(val)
		q.Push(val)

		funcPop()
		funcPop()
		funcPop()

		_, ok := q.Pop()
		require.False(t, ok)
	})

	t.Run("large number of elements", func(t *testing.T) {
		const N = 10000

		for i := 0; i < N; i++ {
			q.Push(i)
		}

		for i := 0; i < N; i++ {
			x, ok := q.Pop()
			require.True(t, ok)
			require.Equal(t, i, x)
		}

		_, ok := q.Pop()
		require.False(t, ok)
	})

}

func runStringQueueTests(t *testing.T, q stringQueue) {
	t.Run("with strings", func(t *testing.T) {
		_, ok := q.Pop()
		require.False(t, ok)

		q.Push("hello!")

		v, ok := q.Pop()
		require.True(t, ok)
		require.Equal(t, "hello!", v)
	})

	// My own tests
	t.Run("multiple push and pops", func(t *testing.T) {
		funcPop := func(req_val string) {
			x, ok := q.Pop()
			require.True(t, ok)
			require.Equal(t, req_val, x)
		}

		q.Push("1")
		q.Push("2")
		q.Push("3")

		funcPop("1")
		funcPop("2")
		funcPop("3")

		_, ok := q.Pop()
		require.False(t, ok)
	})

	t.Run("push and pop empty string", func(t *testing.T) {
		q.Push("")

		v, ok := q.Pop()
		require.True(t, ok)
		require.Equal(t, "", v)

		_, ok = q.Pop()
		require.False(t, ok)
	})

	t.Run("push and pop same strings", func(t *testing.T) {
		q.Push("hello")
		q.Push("hello")
		q.Push("hello")

		for i := 0; i < 3; i++ {
			v, ok := q.Pop()
			require.True(t, ok)
			require.Equal(t, "hello", v)
		}

		_, ok := q.Pop()
		require.False(t, ok)
	})

	t.Run("push and then pop large number of elements", func(t *testing.T) {
		N := 1000
		for i := 0; i < N; i++ {
			q.Push(fmt.Sprintf("%d", i))
		}

		for i := 0; i < N; i++ {
			v, ok := q.Pop()
			require.True(t, ok)
			require.Equal(t, fmt.Sprintf("%d", i), v)
		}

		_, ok := q.Pop()
		require.False(t, ok)
	})

	t.Run("push varying-length strings", func(t *testing.T) {
		short := "a"
		medium := "medium-length"
		long := "this is a long sentence to test the queue for varying string length inputs"

		funcPop := func(val string) {
			x, ok := q.Pop()
			require.True(t, ok)
			require.Equal(t, val, x)
		}

		q.Push(short)
		q.Push(medium)
		q.Push(long)

		funcPop(short)
		funcPop(medium)
		funcPop(long)

		_, ok := q.Pop()
		require.False(t, ok)
	})

	t.Run("push and pop empty and non-empty strings", func(t *testing.T) {
		funcPop := func(val string) {
			x, ok := q.Pop()
			require.True(t, ok)
			require.Equal(t, val, x)
		}

		q.Push("")
		q.Push("1")
		q.Push("")
		q.Push("2")
		q.Push("")
		q.Push("3")

		funcPop("")
		funcPop("1")
		funcPop("")
		funcPop("2")
		funcPop("")
		funcPop("3")

		_, ok := q.Pop()
		require.False(t, ok)
	})

	t.Run("repeatedly pop from empty queue", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			_, ok := q.Pop()
			require.False(t, ok)
		}
	})

	t.Run("push, pop, push again", func(t *testing.T) {
		q.Push("1")

		v, ok := q.Pop()
		require.True(t, ok)
		require.Equal(t, "1", v)

		q.Push("2")

		v, ok = q.Pop()
		require.True(t, ok)
		require.Equal(t, "2", v)

		_, ok = q.Pop()
		require.False(t, ok)
	})

	t.Run("push, pop, pop again (empty queue)", func(t *testing.T) {
		q.Push("1")

		v, ok := q.Pop()
		require.True(t, ok)
		require.Equal(t, "1", v)

		_, ok = q.Pop()
		require.False(t, ok)

		q.Push("2")

		v, ok = q.Pop()
		require.True(t, ok)
		require.Equal(t, "2", v)

		_, ok = q.Pop()
		require.False(t, ok)

		q.Push("3")
		q.Push("4")

		v, ok = q.Pop()
		require.True(t, ok)
		require.Equal(t, "3", v)

		v, ok = q.Pop()
		require.True(t, ok)
		require.Equal(t, "4", v)

		_, ok = q.Pop()
		require.False(t, ok)
	})
}

func runConcurrentQueueTests(t *testing.T, q *lab0.ConcurrentQueue[int]) {
	const concurrency = 16
	const pushes = 1000

	ctx := context.Background()
	t.Run("concurrent pushes", func(t *testing.T) {
		eg, _ := errgroup.WithContext(ctx)
		for i := 0; i < concurrency; i++ {
			eg.Go(func() error {
				for i := 0; i < pushes; i++ {
					q.Push(1234)
				}
				return nil
			})
		}
		eg.Wait()

		for i := 0; i < concurrency*pushes; i++ {
			v, ok := q.Pop()
			require.True(t, ok)
			require.Equal(t, 1234, v)
		}
		_, ok := q.Pop()
		require.False(t, ok)
	})

	t.Run("concurrent pops", func(t *testing.T) {
		eg, _ := errgroup.WithContext(ctx)
		for i := 0; i < concurrency*pushes/2; i++ {
			q.Push(i)
		}

		var sum int64
		var found int64
		for i := 0; i < concurrency; i++ {
			eg.Go(func() error {
				for i := 0; i < pushes; i++ {
					v, ok := q.Pop()
					if ok {
						atomic.AddInt64(&sum, int64(v))
						atomic.AddInt64(&found, 1)
					}
				}
				return nil
			})
		}
		eg.Wait()
		// In the end, we should have exactly concurrency * pushes/2 elements in popped
		require.Equal(t, int64(concurrency*pushes/2), found)
		// Mathematical SUM(i=0...7999)
		require.Equal(t, int64(31996000), sum)
	})

	// My own tests:
	t.Run("concurrent push-pop pairs", func(t *testing.T) {
		eg, _ := errgroup.WithContext(ctx)

		var sum int64
		var found int64

		for i := 0; i < concurrency; i++ {
			eg.Go(func() error {
				for j := 0; j < pushes; j++ {
					q.Push(j)
					v, ok := q.Pop()
					if ok {
						atomic.AddInt64(&sum, int64(v))
						atomic.AddInt64(&found, 1)
					}
				}
				return nil
			})
		}

		eg.Wait()
		require.Equal(t, int64(concurrency*pushes), found)

		expected_sum := int64(concurrency * pushes * (pushes - 1) / 2)
		require.Equal(t, expected_sum, sum, "Sum of popped values does not match")
	})

	t.Run("concurrent pushes with random delays", func(t *testing.T) {
		eg, _ := errgroup.WithContext(ctx)
		for i := 0; i < concurrency; i++ {
			eg.Go(func() error {
				for i := 0; i < pushes; i++ {
					time.Sleep(time.Duration(rand.Intn(3)) * time.Millisecond)
					q.Push(1234)
				}
				return nil
			})
		}
		eg.Wait()

		for i := 0; i < concurrency*pushes; i++ {
			v, ok := q.Pop()
			require.True(t, ok)
			require.Equal(t, 1234, v)
		}
		_, ok := q.Pop()
		require.False(t, ok)
	})

	t.Run("concurrent push and pop mixed", func(t *testing.T) {
		eg, _ := errgroup.WithContext(ctx)

		for i := 0; i < concurrency; i++ {
			eg.Go(func() error {
				for j := 0; j < pushes; j++ {
					q.Push(j)
				}
				return nil
			})
		}

		var sum int64
		var found int64
		for i := 0; i < concurrency; i++ {
			eg.Go(func() error {
				for j := 0; j < pushes; j++ {
					v, ok := q.Pop()
					if ok {
						atomic.AddInt64(&sum, int64(v))
						atomic.AddInt64(&found, 1)
					}
				}
				return nil
			})
		}

		eg.Wait()

		for i := 0; i < int(concurrency*pushes)-int(found); i++ {
			_, ok := q.Pop()
			require.True(t, ok)
		}

		_, ok := q.Pop()
		require.False(t, ok)

		require.GreaterOrEqual(t, int64(concurrency*pushes), found)

		expected_sum := int64(concurrency * pushes * (pushes - 1) / 2)
		require.GreaterOrEqual(t, expected_sum, sum, "Sum of popped values exceeds upper bound")
	})

	t.Run("concurrent push and pop mixed (STRESS TEST)", func(t *testing.T) {
		const new_concurrency = 100
		const new_pushes = 10000
		eg, _ := errgroup.WithContext(ctx)

		for i := 0; i < new_concurrency; i++ {
			eg.Go(func() error {
				for j := 0; j < new_pushes; j++ {
					q.Push(j)
				}
				return nil
			})
		}

		var sum int64
		var found int64
		for i := 0; i < new_concurrency; i++ {
			eg.Go(func() error {
				for j := 0; j < new_pushes; j++ {
					v, ok := q.Pop()
					if ok {
						atomic.AddInt64(&sum, int64(v))
						atomic.AddInt64(&found, 1)
					}
				}
				return nil
			})
		}

		eg.Wait()

		for i := 0; i < int(new_concurrency*new_pushes)-int(found); i++ {
			_, ok := q.Pop()
			require.True(t, ok)
		}

		_, ok := q.Pop()
		require.False(t, ok)

		require.GreaterOrEqual(t, int64(new_concurrency*new_pushes), found)

		expected_sum := int64(new_concurrency * new_pushes * (new_pushes - 1) / 2)
		require.GreaterOrEqual(t, expected_sum, sum, "Sum of popped values exceeds upper bound")
	})

}

func TestGenericQueue(t *testing.T) {
	q := lab0.NewQueue[int]()

	require.NotNil(t, q)
	runIntQueueTests(t, q)

	qs := lab0.NewQueue[string]()
	require.NotNil(t, qs)
	runStringQueueTests(t, qs)
}

func TestConcurrentQueue(t *testing.T) {
	q := lab0.NewConcurrentQueue[int]()
	require.NotNil(t, q)
	runIntQueueTests(t, q)

	qs := lab0.NewConcurrentQueue[string]()
	require.NotNil(t, qs)
	runStringQueueTests(t, qs)

	qc := lab0.NewConcurrentQueue[int]()
	require.NotNil(t, qc)
	runConcurrentQueueTests(t, qc)
}
