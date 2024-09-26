package server_lib

import (
	"context"
	"log"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	ranker "cs426.yale.edu/lab1/ranker"
	umc "cs426.yale.edu/lab1/user_service/mock_client"
	upb "cs426.yale.edu/lab1/user_service/proto"
	pb "cs426.yale.edu/lab1/video_rec_service/proto"
	vmc "cs426.yale.edu/lab1/video_service/mock_client"
	vpb "cs426.yale.edu/lab1/video_service/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type VideoRecServiceOptions struct {
	// Server address for the UserService"
	UserServiceAddr string
	// Server address for the VideoService
	VideoServiceAddr string
	// Maximum size of batches sent to UserService and VideoService
	MaxBatchSize int
	// If set, disable fallback to cache
	DisableFallback bool
	// If set, disable all retries
	DisableRetry bool
}

type VideoRecServiceServer struct {
	pb.UnimplementedVideoRecServiceServer
	options             VideoRecServiceOptions
	usr_client          upb.UserServiceClient
	vid_client          vpb.VideoServiceClient
	total_reqs          uint64
	total_errs          uint64
	total_usr_serv_errs uint64
	total_active_reqs   int64
	total_latency       int64
	total_stale_resp    uint64
	total_vid_serv_errs uint64
	trending_vids       *cachedVideos
}

type cachedVideos struct {
	videos  []*vpb.VideoInfo
	timeout int64
	m       sync.RWMutex
}

func MakeVideoRecServiceServer(options VideoRecServiceOptions) (*VideoRecServiceServer, error) {
	serv := &VideoRecServiceServer{
		options:       options,
		trending_vids: &cachedVideos{},
	}

	usr_conn, err := grpc.NewClient(options.UserServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		if !options.DisableRetry {
			log.Printf("Retrying UserService connection establishment: %v", err)
			usr_conn, err = grpc.NewClient(options.UserServiceAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()))
		}
		if err != nil {
			log.Printf("Failed to connect to UserService address %s: %v", options.UserServiceAddr, err)

			return nil, err
		}
	}

	vid_conn, err := grpc.NewClient(options.VideoServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		if !options.DisableRetry {
			log.Printf("Retrying VideoService connection establishment: %v", err)
			vid_conn, err = grpc.NewClient(options.VideoServiceAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()))
		}
		if err != nil {
			log.Printf("Failed to connect to VideoService address %s: %v", options.UserServiceAddr, err)

			return nil, err
		}
	}

	serv.usr_client = upb.NewUserServiceClient(usr_conn)
	serv.vid_client = vpb.NewVideoServiceClient(vid_conn)

	go serv.refreshTrendingVideos()

	return serv, nil
}

func MakeVideoRecServiceServerWithMocks(
	options VideoRecServiceOptions,
	mockUserClient *umc.MockUserServiceClient,
	mockVideoClient *vmc.MockVideoServiceClient,
) *VideoRecServiceServer {
	serv := &VideoRecServiceServer{
		options:       options,
		usr_client:    mockUserClient,
		vid_client:    mockVideoClient,
		trending_vids: &cachedVideos{},
	}

	go serv.refreshTrendingVideos()

	return serv
}

func (server *VideoRecServiceServer) refreshTrendingVideos() {
	for {
		if time.Now().Unix() > server.trending_vids.timeout {
			trending_resp, err := server.vid_client.GetTrendingVideos(context.Background(), &vpb.GetTrendingVideosRequest{})
			if err != nil {
				log.Printf("Failed to retrieve trends: %v", err)
				time.Sleep(10 * time.Second)
				continue
			}

			max_batch_size := server.options.MaxBatchSize
			video_ids := trending_resp.GetVideos()
			len_vids := len(video_ids)
			var trending_vids []*vpb.VideoInfo
			for i := 0; i < len_vids; i += max_batch_size {
				j := int(math.Min(float64(i+max_batch_size), float64(len(video_ids))))

				vid_resp, err := server.vid_client.GetVideo(context.Background(), &vpb.GetVideoRequest{VideoIds: video_ids[i:j]})
				if err != nil {
					log.Printf("Failed to fetch videos: %v", err)
					break
				}
				batch_vids := vid_resp.GetVideos()
				trending_vids = append(trending_vids, batch_vids...)
			}

			server.trending_vids.m.Lock()
			server.trending_vids.videos = trending_vids
			server.trending_vids.timeout = time.Now().Unix() + int64(trending_resp.GetExpirationTimeS())
			server.trending_vids.m.Unlock()
		}

		time.Sleep(10 * time.Second)
	}
}

func (server *VideoRecServiceServer) GetTopVideos(
	ctx context.Context,
	req *pb.GetTopVideosRequest,
) (*pb.GetTopVideosResponse, error) {
	t0 := time.Now().UnixMilli()
	atomic.AddInt64(&server.total_active_reqs, 1)
	defer atomic.AddInt64(&server.total_active_reqs, -1)
	atomic.AddUint64(&server.total_reqs, 1)

	usr_resp, err := server.usr_client.GetUser(ctx, &upb.GetUserRequest{UserIds: []uint64{req.UserId}})
	if err != nil || len(usr_resp.GetUsers()) == 0 {
		if server.options.DisableFallback {
			atomic.AddUint64(&server.total_usr_serv_errs, 1)
			atomic.AddUint64(&server.total_errs, 1)
		}
		return server.handleFallback(req)
	}

	users := usr_resp.GetUsers()[0]
	video_from_subs := server.getSubscribedVideos(ctx, users.GetSubscribedTo())
	video_from_subs = deduplicate(video_from_subs)

	vids, err := server.getVideoInfo(ctx, video_from_subs)
	if err != nil {
		if server.options.DisableFallback {
			atomic.AddUint64(&server.total_usr_serv_errs, 1)
			atomic.AddUint64(&server.total_errs, 1)
		}
		return server.handleFallback(req)
	}

	ranked_vids := server.rankerVids(users.GetUserCoefficients(), vids)
	lim_num_vids := int(req.Limit)
	if lim_num_vids == 0 || lim_num_vids > len(ranked_vids) {
		lim_num_vids = len(ranked_vids)
	}

	atomic.AddInt64(&server.total_latency, time.Now().UnixMilli()-t0)
	return &pb.GetTopVideosResponse{Videos: ranked_vids[:lim_num_vids]}, nil
}

func (server *VideoRecServiceServer) getSubscribedVideos(ctx context.Context, subscribedUserIDs []uint64) []uint64 {
	var liked_sub_videos []uint64

	for i := 0; i < len(subscribedUserIDs); i += server.options.MaxBatchSize {
		j := int(math.Min(float64(i+server.options.MaxBatchSize), float64(len(subscribedUserIDs))))

		usr_resp, err := server.usr_client.GetUser(ctx, &upb.GetUserRequest{UserIds: subscribedUserIDs[i:j]})
		if err != nil {
			log.Printf("Failed to fetch subscribed user: %v", err)
			continue
		}

		for _, user := range usr_resp.GetUsers() {
			liked_sub_videos = append(liked_sub_videos, user.GetLikedVideos()...)
		}
	}

	return liked_sub_videos
}

func (server *VideoRecServiceServer) getVideoInfo(ctx context.Context, videoIds []uint64) ([]*vpb.VideoInfo, error) {
	videos := make([]*vpb.VideoInfo, 0)

	for i := 0; i < len(videoIds); i += server.options.MaxBatchSize {
		j := int(math.Min(float64(i+server.options.MaxBatchSize), float64(len(videoIds))))

		vid_resp, err := server.vid_client.GetVideo(ctx, &vpb.GetVideoRequest{VideoIds: videoIds[i:j]})
		if err != nil {
			return nil, err
		}
		videos = append(videos, vid_resp.GetVideos()...)
	}

	return videos, nil
}

func (server *VideoRecServiceServer) rankerVids(userCoeffs *upb.UserCoefficients, videos []*vpb.VideoInfo) []*vpb.VideoInfo {
	ranker := &ranker.BcryptRanker{}
	scores := make([]struct {
		Video *vpb.VideoInfo
		Score uint64
	}, len(videos))

	for i, video := range videos {
		score := ranker.Rank(userCoeffs, video.GetVideoCoefficients())
		scores[i] = struct {
			Video *vpb.VideoInfo
			Score uint64
		}{Video: video, Score: score}
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].Score > scores[j].Score })

	vid_ranks := make([]*vpb.VideoInfo, len(videos))
	for i, score := range scores {
		vid_ranks[i] = score.Video
	}

	return vid_ranks
}

func (server *VideoRecServiceServer) handleFallback(req *pb.GetTopVideosRequest) (*pb.GetTopVideosResponse, error) {
	if server.options.DisableFallback {
		log.Printf("Fallback disabled")
		return nil, status.Error(codes.Unavailable, "Fallback is disabled")
	}

	server.trending_vids.m.RLock()
	defer server.trending_vids.m.RUnlock()

	if len(server.trending_vids.videos) == 0 {
		log.Printf("Total trending videos: %d", len(server.trending_vids.videos))
	}
	atomic.AddUint64(&server.total_stale_resp, 1)

	lim_num_vids := int(req.Limit)
	if lim_num_vids == 0 || lim_num_vids > len(server.trending_vids.videos) {
		lim_num_vids = len(server.trending_vids.videos)
	}

	return &pb.GetTopVideosResponse{
		Videos:        server.trending_vids.videos[:lim_num_vids],
		StaleResponse: true,
	}, nil
}

func deduplicate(items []uint64) []uint64 {
	seen := map[uint64]bool{}
	non_dup := []uint64{}

	for _, i := range items {
		if !seen[i] {
			seen[i] = true
			non_dup = append(non_dup, i)
		}
	}

	return non_dup
}

func (server *VideoRecServiceServer) GetStats(ctx context.Context, req *pb.GetStatsRequest) (*pb.GetStatsResponse, error) {
	avg_latency := float32(0)
	if server.total_reqs > 0 {
		total_latency := float32(atomic.LoadInt64(&server.total_latency))
		avg_latency = total_latency / float32(server.total_reqs)
	}

	response := &pb.GetStatsResponse{
		TotalRequests:      atomic.LoadUint64(&server.total_reqs),
		TotalErrors:        atomic.LoadUint64(&server.total_errs),
		ActiveRequests:     uint64(atomic.LoadInt64(&server.total_active_reqs)),
		UserServiceErrors:  atomic.LoadUint64(&server.total_usr_serv_errs),
		VideoServiceErrors: atomic.LoadUint64(&server.total_vid_serv_errs),
		AverageLatencyMs:   avg_latency,
		StaleResponses:     atomic.LoadUint64(&server.total_stale_resp),
	}

	return response, nil
}
