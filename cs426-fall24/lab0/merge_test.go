package lab0_test

import (
	"context"
	"testing"
	"time"

	"fmt"

	"cs426.cloud/lab0"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func chanToSlice[T any](ch chan T) []T {
	vals := make([]T, 0)
	for item := range ch {
		vals = append(vals, item)
	}
	return vals
}

type mergeFunc = func(chan string, chan string, chan string)

func runMergeTest(t *testing.T, merge mergeFunc) {
	t.Run("empty channels", func(t *testing.T) {
		a := make(chan string)
		b := make(chan string)
		out := make(chan string)
		close(a)
		close(b)

		merge(a, b, out)
		// If your lab0 hangs here, make sure you are closing your channels!
		require.Empty(t, chanToSlice(out))
	})

	// My own tests
	t.Run("only one empty channel", func(t *testing.T) {
		a := make(chan string)
		b := make(chan string, 2)
		out := make(chan string, 10)

		b <- "a"
		b <- "b"
		close(a)
		close(b)

		merge(a, b, out)
		expected := []string{"a", "b"}
		require.ElementsMatch(t, expected, chanToSlice(out))
	})

	t.Run("full single-element channels", func(t *testing.T) {
		a := make(chan string, 1)
		b := make(chan string, 1)
		out := make(chan string, 2)

		a <- "1"
		b <- "a"
		close(a)
		close(b)

		merge(a, b, out)
		expected := []string{"1", "a"}
		require.ElementsMatch(t, expected, chanToSlice(out))
	})

	t.Run("full large channels", func(t *testing.T) {
		a := make(chan string, 30)
		b := make(chan string, 30)
		out := make(chan string, 60)

		expected := []string{}
		for i := 1; i <= 30; i++ {
			a_i := fmt.Sprintf("%d", i)
			b_i := string(rune('a' + i - 1))
			a <- a_i
			b <- b_i

			expected = append(expected, a_i)
			expected = append(expected, b_i)
		}

		close(a)
		close(b)

		merge(a, b, out)
		require.ElementsMatch(t, expected, chanToSlice(out))
	})

	t.Run("different size channels", func(t *testing.T) {
		a := make(chan string, 3)
		b := make(chan string, 7)
		out := make(chan string, 15)

		expected := []string{}
		for i := 0; i < 7; i++ {
			if i < 3 {
				a_i := fmt.Sprintf("%d", i+1)
				a <- a_i

				expected = append(expected, a_i)
			}
			b_i := string(rune('a' + i))
			b <- b_i

			expected = append(expected, b_i)
		}

		close(a)
		close(b)

		merge(a, b, out)
		// expected := []string{"1", "2", "3", "a", "b", "c", "d", "e", "f", "g"}
		require.ElementsMatch(t, expected, chanToSlice(out))
	})

	t.Run("channels with empty strings", func(t *testing.T) {
		a := make(chan string, 2)
		b := make(chan string, 2)
		out := make(chan string, 15)

		a <- ""
		a <- "1"
		b <- "2"
		b <- ""

		close(a)
		close(b)

		merge(a, b, out)
		expected := []string{"", "1", "2", ""}
		require.ElementsMatch(t, expected, chanToSlice(out))
	})

	t.Run("non-full channels", func(t *testing.T) {
		a := make(chan string, 2)
		b := make(chan string, 3)
		out := make(chan string, 20)

		a <- "1"
		b <- "2"
		b <- "3"

		close(a)
		close(b)

		merge(a, b, out)
		expected := []string{"1", "2", "3"}
		require.ElementsMatch(t, expected, chanToSlice(out))
	})

}

func TestMergeChannels(t *testing.T) {
	runMergeTest(t, func(a, b, out chan string) {
		lab0.MergeChannels(a, b, out)
	})
}

func TestMergeOrCancel(t *testing.T) {
	runMergeTest(t, func(a, b, out chan string) {
		_ = lab0.MergeChannelsOrCancel(context.Background(), a, b, out)
	})

	t.Run("already canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		a := make(chan string, 1)
		b := make(chan string, 1)
		out := make(chan string, 10)

		eg, _ := errgroup.WithContext(context.Background())
		eg.Go(func() error {
			return lab0.MergeChannelsOrCancel(ctx, a, b, out)
		})
		err := eg.Wait()
		a <- "a"
		b <- "b"

		require.Error(t, err)
		require.Equal(t, []string{}, chanToSlice(out))
	})

	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		a := make(chan string)
		b := make(chan string)
		out := make(chan string, 10)

		eg, _ := errgroup.WithContext(context.Background())
		eg.Go(func() error {
			return lab0.MergeChannelsOrCancel(ctx, a, b, out)
		})
		a <- "a"
		b <- "b"
		cancel()

		err := eg.Wait()
		require.Error(t, err)
		require.Equal(t, []string{"a", "b"}, chanToSlice(out))
	})

	// My own tests
	t.Run("cancel midway", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		a := make(chan string, 5)
		b := make(chan string, 5)
		out := make(chan string, 10)

		for i := 1; i <= 5; i++ {
			val := fmt.Sprintf("%d", i)

			a <- val
			b <- val
		}

		eg, ctx := errgroup.WithContext(ctx)
		eg.Go(func() error {
			return lab0.MergeChannelsOrCancel(ctx, a, b, out)
		})

		cancel()

		err := eg.Wait()
		require.Error(t, err)

		require.LessOrEqual(t, len(chanToSlice(out)), 10,
			"Expected fewer elements due to early cancellation")
	})

	t.Run("cancel before merge", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		a := make(chan string, 5)
		b := make(chan string, 5)
		out := make(chan string, 10)

		eg, _ := errgroup.WithContext(context.Background())
		eg.Go(func() error {
			return lab0.MergeChannelsOrCancel(ctx, a, b, out)
		})

		err := eg.Wait()
		require.Error(t, err)

		require.Empty(t, chanToSlice(out), "Expected no values due to immediate cancellation")
	})

	t.Run("early cancellation of big channels", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		a := make(chan string, 10000)
		b := make(chan string, 10000)
		out := make(chan string, 20000)

		for i := 0; i < 5000; i++ {
			a <- fmt.Sprintf("a%d", i)
			b <- fmt.Sprintf("b%d", i)
		}

		eg, _ := errgroup.WithContext(ctx)

		eg.Go(func() error {
			return lab0.MergeChannelsOrCancel(ctx, a, b, out)
		})

		time.Sleep(20 * time.Microsecond)
		cancel()

		err := eg.Wait()
		require.Error(t, err)

		require.GreaterOrEqual(t, len(chanToSlice(out)), 1)
		require.LessOrEqual(t, len(chanToSlice(out)), 10000)
	})

	t.Run("merge identical channels", func(t *testing.T) {
		ctx := context.Background()

		a := make(chan string, 5)
		b := make(chan string, 5)
		out := make(chan string, 15)

		expected := []string{}
		for i := 1; i <= 5; i++ {
			val := fmt.Sprintf("%d", i)
			a <- val
			b <- val

			expected = append(expected, val)
			expected = append(expected, val)
		}

		close(a)
		close(b)

		eg, _ := errgroup.WithContext(context.Background())
		eg.Go(func() error {
			return lab0.MergeChannelsOrCancel(ctx, a, b, out)
		})

		err := eg.Wait()
		require.NoError(t, err)
		require.ElementsMatch(t, expected, chanToSlice(out))
	})
}

type channelFetcher struct {
	ch chan string
}

func newChannelFetcher(ch chan string) *channelFetcher {
	return &channelFetcher{ch: ch}
}

func (f *channelFetcher) Fetch() (string, bool) {
	v, ok := <-f.ch
	return v, ok
}

func TestMergeFetches(t *testing.T) {
	runMergeTest(t, func(a, b, out chan string) {
		lab0.MergeFetches(newChannelFetcher(a), newChannelFetcher(b), out)
	})
}

func TestMergeFetchesAdditional(t *testing.T) {
	// TODO: add your extra tests here
	t.Run("single-element fetchers", func(t *testing.T) {
		a := make(chan string, 1)
		b := make(chan string, 1)
		out := make(chan string, 2)

		a <- "a"
		b <- "b"
		close(a)
		close(b)

		lab0.MergeFetches(newChannelFetcher(a), newChannelFetcher(b), out)

		expected := []string{"a", "b"}
		require.ElementsMatch(t, expected, chanToSlice(out))
	})

	t.Run("multi-element fetchers", func(t *testing.T) {
		a := make(chan string, 10)
		b := make(chan string, 10)
		out := make(chan string, 20)

		for i := 1; i <= 10; i++ {
			a <- fmt.Sprintf("%d", i)
		}
		for i := 0; i < 10; i++ {
			b <- string(rune('a' + i))
		}

		close(a)
		close(b)

		lab0.MergeFetches(newChannelFetcher(a), newChannelFetcher(b), out)

		expected := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10",
			"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
		require.ElementsMatch(t, expected, chanToSlice(out))
	})
}
