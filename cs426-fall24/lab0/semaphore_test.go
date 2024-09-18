package lab0_test

import (
	"context"
	"testing"
	"time"

	"cs426.cloud/lab0"
	"github.com/stretchr/testify/require"
)

func TestSemaphore(t *testing.T) {
	t.Run("semaphore basic", func(t *testing.T) {
		s := lab0.NewSemaphore()
		go func() {
			s.Post()
		}()
		err := s.Wait(context.Background())
		require.NoError(t, err)
	})
	t.Run("semaphore starts with zero available resources", func(t *testing.T) {
		s := lab0.NewSemaphore()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		err := s.Wait(ctx)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
	t.Run("semaphore post before wait does not block", func(t *testing.T) {
		s := lab0.NewSemaphore()
		s.Post()
		s.Post()
		s.Post()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		err := s.Wait(ctx)
		require.NoError(t, err)
	})
	t.Run("post after wait releases the wait", func(t *testing.T) {
		s := lab0.NewSemaphore()
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		go func() {
			time.Sleep(20 * time.Millisecond)
			s.Post()
		}()
		err := s.Wait(ctx)
		require.NoError(t, err)
	})

	// My own tests
	t.Run("multiple waiters released by multiple posts", func(t *testing.T) {
		s := lab0.NewSemaphore()
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		a := make(chan error)
		b := make(chan error)

		go func() {
			a <- s.Wait(ctx)
		}()
		go func() {
			b <- s.Wait(ctx)
		}()

		time.Sleep(10 * time.Millisecond)

		s.Post()
		s.Post()

		require.NoError(t, <-a)
		require.NoError(t, <-b)
	})

	t.Run("waits w/o many posts", func(t *testing.T) {
		s := lab0.NewSemaphore()
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		go func() {
			time.Sleep(10 * time.Millisecond)
			s.Post()
		}()

		err1 := s.Wait(ctx)
		require.NoError(t, err1)

		err2 := s.Wait(ctx)
		require.ErrorIs(t, err2, context.DeadlineExceeded)
	})

	t.Run("multiple posts and successful waits", func(t *testing.T) {
		s := lab0.NewSemaphore()

		s.Post()
		s.Post()
		require.NoError(t, s.Wait(context.Background()))
		require.NoError(t, s.Wait(context.Background()))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		err := s.Wait(ctx)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("multiple concurrent waiters released by multiple posts", func(t *testing.T) {
		s := lab0.NewSemaphore()

		a := make(chan error)
		b := make(chan error)
		c := make(chan error)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		go func() {
			a <- s.Wait(ctx)
		}()
		go func() {
			b <- s.Wait(ctx)
		}()
		go func() {
			c <- s.Wait(ctx)
		}()

		time.Sleep(10 * time.Millisecond)

		s.Post()
		s.Post()
		s.Post()

		require.NoError(t, <-a)
		require.NoError(t, <-b)
		require.NoError(t, <-c)
	})
}
