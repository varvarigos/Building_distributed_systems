package kv

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"cs426.yale.edu/lab4/kv/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type KvServerImpl struct {
	proto.UnimplementedKvServer
	nodeName string

	shardMap        *ShardMap
	listener        *ShardMapListener
	clientPool      ClientPool
	shutdown        chan struct{}
	mu              sync.RWMutex
	ttl             chan bool
	availableShards map[int32]*shardData
}

type cacheItem struct {
	value        string
	ttl          int64
	time_elapsed time.Time
}

type shardData struct {
	data        map[string]*cacheItem
	mu          sync.RWMutex
	closeSignal chan bool
}

func (server *KvServerImpl) retrieveShardData(shardID int32) *proto.GetShardContentsResponse {
	numNodes := len(server.shardMap.NodesForShard(int(shardID)))
	startIndex := rand.Intn(numNodes)
	for i := 0; i < numNodes; i++ {
		node := server.shardMap.NodesForShard(int(shardID))[(startIndex+i)%numNodes]
		if node != server.nodeName {
			client, err := server.clientPool.GetClient(node)
			if err == nil {
				response, err := client.GetShardContents(context.Background(), &proto.GetShardContentsRequest{Shard: shardID})
				if err == nil {
					return response
				}
			}
		}
	}
	return &proto.GetShardContentsResponse{}
}

func (server *KvServerImpl) handleShardMapUpdate() {
	server.mu.Lock()
	defer server.mu.Unlock()

	shardsToBeAdded := make(map[int32]bool)
	shardsToBeRemoved := make(map[int32]bool)

	for _, shard := range server.shardMap.ShardsForNode(server.nodeName) {
		_, found := server.availableShards[int32(shard)]
		if !found {
			shardsToBeAdded[int32(shard)] = true
		}
	}

	for shard := range server.availableShards {
		found := false
		for _, shardInMap := range server.shardMap.ShardsForNode(server.nodeName) {
			if int32(shardInMap) == shard {
				found = true
				break
			}
		}
		if !found {
			shardsToBeRemoved[shard] = true
		}
	}

	if len(shardsToBeAdded) > 0 {
		var shardContentsResponses []*proto.GetShardContentsResponse
		var resultsMu sync.Mutex
		var wg sync.WaitGroup

		for shard := range shardsToBeAdded {
			newShard := &shardData{data: make(map[string]*cacheItem), mu: sync.RWMutex{}, closeSignal: make(chan bool)}

			go func() {
				ticker := time.NewTicker(500 * time.Millisecond)
				defer ticker.Stop()

				for {
					select {
					case <-ticker.C:
						newShard.mu.Lock()
						currTime := time.Now()
						for cacheKey, cacheItem := range newShard.data {
							if currTime.Sub(cacheItem.time_elapsed).Milliseconds() >= cacheItem.ttl {
								delete(newShard.data, cacheKey)
							}
						}
						newShard.mu.Unlock()
					case <-newShard.closeSignal:
						return
					}
				}
			}()

			server.availableShards[shard] = newShard

			wg.Add(1)
			go func(shard int32) {
				defer wg.Done()
				res := server.retrieveShardData(shard)
				resultsMu.Lock()
				shardContentsResponses = append(shardContentsResponses, res)
				resultsMu.Unlock()
			}(shard)
		}

		server.mu.Unlock()
		wg.Wait()
		server.mu.Lock()

		for _, resp := range shardContentsResponses {
			for _, shardVal := range resp.Values {
				shardID := GetShardForKey(shardVal.Key, server.shardMap.NumShards())
				shard := server.availableShards[int32(shardID)]
				shard.mu.Lock()
				shard.data[shardVal.Key] = &cacheItem{}
				shard.data[shardVal.Key].value = shardVal.Value
				shard.data[shardVal.Key].ttl = shardVal.TtlMsRemaining
				shard.data[shardVal.Key].time_elapsed = time.Now()
				shard.mu.Unlock()
			}
		}
	}
	if len(shardsToBeRemoved) > 0 {
		for shard := range shardsToBeRemoved {
			server.availableShards[shard].closeSignal <- true
			delete(server.availableShards, shard)
		}
	}
}

func (server *KvServerImpl) shardMapListenLoop() {
	listener := server.listener.UpdateChannel()
	for {
		select {
		case <-server.shutdown:
			return
		case <-listener:
			server.handleShardMapUpdate()
		}
	}
}

func MakeKvServer(nodeName string, shardMap *ShardMap, clientPool ClientPool) *KvServerImpl {
	listener := shardMap.MakeListener()
	server := KvServerImpl{
		nodeName:        nodeName,
		shardMap:        shardMap,
		listener:        &listener,
		clientPool:      clientPool,
		shutdown:        make(chan struct{}),
		ttl:             make(chan bool),
		availableShards: make(map[int32]*shardData),
	}
	for _, shard := range server.availableShards {
		shard.closeSignal <- true
	}

	go server.shardMapListenLoop()
	server.handleShardMapUpdate()
	return &server
}

func (server *KvServerImpl) Shutdown() {
	server.shutdown <- struct{}{}
	server.listener.Close()
}

func (server *KvServerImpl) Get(
	ctx context.Context,
	request *proto.GetRequest,
) (*proto.GetResponse, error) {
	// Trace-level logging for node receiving this request (enable by running with -log-level=trace),
	// feel free to use Trace() or Debug() logging in your code to help debug tests later without
	// cluttering logs by default. See the logging section of the spec.
	logrus.WithFields(
		logrus.Fields{"node": server.nodeName, "key": request.Key},
	).Trace("node received Get() request")

	server.mu.RLock()
	defer server.mu.RUnlock()

	if request.Key == "" {
		return &proto.GetResponse{Value: "", WasFound: false}, status.Error(codes.InvalidArgument, "empty key is not allowed")
	}

	shardID := GetShardForKey(request.Key, server.shardMap.NumShards())

	_, found := server.availableShards[int32(shardID)]
	if !found {
		return &proto.GetResponse{Value: "", WasFound: false}, status.Error(codes.NotFound, "NotFound")
	}

	server.availableShards[int32(shardID)].mu.RLock()
	shardData, ok := server.availableShards[int32(shardID)].data[request.Key]
	if !ok {
		server.availableShards[int32(shardID)].mu.RUnlock()
		return &proto.GetResponse{Value: "", WasFound: false}, nil
	}

	ttl := shardData.ttl
	if time.Since(shardData.time_elapsed).Milliseconds() >= ttl {
		server.availableShards[int32(shardID)].mu.RUnlock()
		return &proto.GetResponse{Value: "", WasFound: false}, nil
	}

	server.availableShards[int32(shardID)].mu.RUnlock()
	return &proto.GetResponse{Value: shardData.value, WasFound: true}, nil
}

func (server *KvServerImpl) Set(
	ctx context.Context,
	request *proto.SetRequest,
) (*proto.SetResponse, error) {
	logrus.WithFields(
		logrus.Fields{"node": server.nodeName, "key": request.Key},
	).Trace("node received Set() request")

	server.mu.Lock()
	defer server.mu.Unlock()

	if request.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "empty key is not allowed")
	}

	shardID := GetShardForKey(request.Key, server.shardMap.NumShards())

	_, found := server.availableShards[int32(shardID)]
	if !found {
		return &proto.SetResponse{}, status.Error(codes.NotFound, "NotFound")
	}

	shard := server.availableShards[int32(shardID)]
	shard.mu.Lock()
	shard.data[request.Key] = &cacheItem{value: request.Value, ttl: request.TtlMs, time_elapsed: time.Now()}
	shard.mu.Unlock()

	return &proto.SetResponse{}, nil
}

func (server *KvServerImpl) Delete(
	ctx context.Context,
	request *proto.DeleteRequest,
) (*proto.DeleteResponse, error) {
	logrus.WithFields(
		logrus.Fields{"node": server.nodeName, "key": request.Key},
	).Trace("node received Delete() request")

	server.mu.Lock()
	defer server.mu.Unlock()

	if request.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "empty key is not allowed")
	}

	shardID := GetShardForKey(request.Key, server.shardMap.NumShards())

	_, found := server.availableShards[int32(shardID)]
	if !found {
		return &proto.DeleteResponse{}, status.Error(codes.NotFound, "NotFound")
	}

	shard := server.availableShards[int32(shardID)]
	shard.mu.Lock()
	delete(shard.data, request.Key)
	shard.mu.Unlock()

	return &proto.DeleteResponse{}, nil
}

func (server *KvServerImpl) GetShardContents(
	ctx context.Context,
	request *proto.GetShardContentsRequest,
) (*proto.GetShardContentsResponse, error) {
	server.mu.RLock()
	defer server.mu.RUnlock()

	shardData, found := server.availableShards[int32(request.Shard)]
	response := &proto.GetShardContentsResponse{Values: make([]*proto.GetShardValue, 0)}

	shardData.mu.RLock()
	defer shardData.mu.RUnlock()

	if !found {
		return nil, status.Error(codes.NotFound, "NotFound")
	}

	for key, cacheItem := range shardData.data {
		if time.Since(cacheItem.time_elapsed).Milliseconds() < cacheItem.ttl {
			elapsed_time := cacheItem.ttl - time.Since(cacheItem.time_elapsed).Milliseconds()
			response.Values = append(
				response.Values,
				&proto.GetShardValue{
					Key:            key,
					Value:          cacheItem.value,
					TtlMsRemaining: elapsed_time,
				})
		}
	}

	return response, nil
}
