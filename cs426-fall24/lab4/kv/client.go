package kv

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"cs426.yale.edu/lab4/kv/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Kv struct {
	shardMap   *ShardMap
	clientPool ClientPool

	// Add any client-side state you want here
}

func MakeKv(shardMap *ShardMap, clientPool ClientPool) *Kv {
	kv := &Kv{
		shardMap:   shardMap,
		clientPool: clientPool,
	}
	// Add any initialization logic
	return kv
}

// Get semantics from the client:
// If a request to a node is successful, return (Value, WasFound, nil)
// Requests where WasFound == false are still considered successful.
// For now (though see B3), any errors returned from GetClient or the RPC can be propagated back to the caller as ("", false, err).
// If no nodes are available (none host the shard), return an error.
func (kv *Kv) Get(ctx context.Context, key string) (string, bool, error) {
	// Trace-level logging -- you can remove or use to help debug in your tests
	// with `-log-level=trace`. See the logging section of the spec.
	logrus.WithFields(
		logrus.Fields{"key": key},
	).Trace("client sending Get() request")

	shardID := GetShardForKey(key, kv.shardMap.NumShards())
	available_nodes := kv.shardMap.NodesForShard(shardID)

	if len(available_nodes) == 0 {
		return "", false, status.Error(codes.NotFound, "no nodes available")
	}

	startIndex := rand.IntN(len(available_nodes))
	lastErr := error(nil)
	for i := 0; i < len(available_nodes); i++ {
		node := available_nodes[(i+startIndex)%len(available_nodes)]
		client, err := kv.clientPool.GetClient(node)
		if err != nil {
			lastErr = err
			continue
		}
		response, err := client.Get(ctx, &proto.GetRequest{Key: key})
		if err == nil {
			return response.Value, response.WasFound, nil
		}
		lastErr = err
	}

	return "", false, lastErr
}

func (kv *Kv) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	logrus.WithFields(
		logrus.Fields{"key": key},
	).Trace("client sending Set() request")

	shardID := GetShardForKey(key, kv.shardMap.NumShards())
	available_nodes := kv.shardMap.NodesForShard(shardID)
	if len(available_nodes) == 0 {
		return status.Error(codes.NotFound, "no nodes available")
	}

	var wg sync.WaitGroup
	errorHandle := make(chan error, len(available_nodes))

	request := &proto.SetRequest{Key: key, Value: value, TtlMs: int64(ttl / time.Millisecond)}

	for _, node := range available_nodes {
		wg.Add(1)
		go func(node string) {
			defer wg.Done()
			client, err := kv.clientPool.GetClient(node)
			if err != nil {
				errorHandle <- err
				return
			}
			_, err = client.Set(ctx, request)
			if err != nil {
				errorHandle <- err
			}
		}(node)
	}

	wg.Wait()
	close(errorHandle)

	if len(errorHandle) > 0 {
		return <-errorHandle
	}

	return nil
}

func (kv *Kv) Delete(ctx context.Context, key string) error {
	logrus.WithFields(
		logrus.Fields{"key": key},
	).Trace("client sending Delete() request")

	shardID := GetShardForKey(key, kv.shardMap.NumShards())
	available_nodes := kv.shardMap.NodesForShard(shardID)
	if len(available_nodes) == 0 {
		return status.Error(codes.NotFound, "no nodes available")
	}

	var wg sync.WaitGroup
	errorHandle := make(chan error, len(available_nodes))

	request := &proto.DeleteRequest{Key: key}

	for _, node := range available_nodes {
		wg.Add(1)
		go func(node string) {
			defer wg.Done()
			client, err := kv.clientPool.GetClient(node)
			if err != nil {
				errorHandle <- err
				return
			}
			_, err = client.Delete(ctx, request)
			if err != nil {
				errorHandle <- err
			}
		}(node)
	}

	wg.Wait()
	close(errorHandle)

	if len(errorHandle) > 0 {
		return <-errorHandle
	}

	return nil
}
