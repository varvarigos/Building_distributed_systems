package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	fipb "cs426.yale.edu/lab1/failure_injection/proto"
	umc "cs426.yale.edu/lab1/user_service/mock_client"
	usl "cs426.yale.edu/lab1/user_service/server_lib"
	vmc "cs426.yale.edu/lab1/video_service/mock_client"
	vsl "cs426.yale.edu/lab1/video_service/server_lib"

	pb "cs426.yale.edu/lab1/video_rec_service/proto"
	sl "cs426.yale.edu/lab1/video_rec_service/server_lib"
)

func TestServerBasic(t *testing.T) {
	vrOptions := sl.VideoRecServiceOptions{
		MaxBatchSize:    50,
		DisableFallback: true,
		DisableRetry:    true,
	}
	// You can specify failure injection options here or later send
	// SetInjectionConfigRequests using these mock clients
	uClient :=
		umc.MakeMockUserServiceClient(*usl.DefaultUserServiceOptions())
	vClient :=
		vmc.MakeMockVideoServiceClient(*vsl.DefaultVideoServiceOptions())
	vrService := sl.MakeVideoRecServiceServerWithMocks(
		vrOptions,
		uClient,
		vClient,
	)

	var userId uint64 = 204054
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := vrService.GetTopVideos(
		ctx,
		&pb.GetTopVideosRequest{UserId: userId, Limit: 5},
	)
	assert.True(t, err == nil)

	videos := out.Videos
	assert.Equal(t, 5, len(videos))
	assert.EqualValues(t, 1012, videos[0].VideoId)
	assert.Equal(t, "Harry Boehm", videos[1].Author)
	assert.EqualValues(t, 1209, videos[2].VideoId)
	assert.Equal(t, "https://video-data.localhost/blob/1309", videos[3].Url)
	assert.Equal(t, "precious here", videos[4].Title)

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	vClient.SetInjectionConfig(ctx, &fipb.SetInjectionConfigRequest{
		Config: &fipb.InjectionConfig{
			// fail one in 1 request, i.e., always fail
			FailureRate: 1,
		},
	})

	// Since we disabled retry and fallback, we expect the VideoRecService to
	// throw an error since the VideoService is "down".
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = vrService.GetTopVideos(
		ctx,
		&pb.GetTopVideosRequest{UserId: userId, Limit: 5},
	)
	assert.False(t, err == nil)
}

func TestBatching(t *testing.T) {
	uOptions := usl.UserServiceOptions{
		Seed:                 123,
		SleepNs:              0,
		FailureRate:          0,
		ResponseOmissionRate: 0,
		MaxBatchSize:         5,
	}
	vOptions := vsl.VideoServiceOptions{
		Seed:                 123,
		TtlSeconds:           25,
		SleepNs:              0,
		FailureRate:          0,
		ResponseOmissionRate: 0,
		MaxBatchSize:         5,
	}
	vrOptions := sl.VideoRecServiceOptions{
		MaxBatchSize:    5,
		DisableFallback: true,
		DisableRetry:    true,
	}
	mock_uClient := umc.MakeMockUserServiceClient(uOptions)
	mock_vClient := vmc.MakeMockVideoServiceClient(vOptions)
	vrService := sl.MakeVideoRecServiceServerWithMocks(
		vrOptions,
		mock_uClient,
		mock_vClient,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := vrService.GetTopVideos(
		ctx,
		&pb.GetTopVideosRequest{UserId: uint64(204054), Limit: 5},
	)
	videos := resp.Videos
	batchSize := vrOptions.MaxBatchSize

	assert.Len(t, videos, 5, "The number of returned videos should match the limit")
	assert.GreaterOrEqual(t, len(videos), batchSize)
	assert.True(t, len(videos)%batchSize == 0 || len(videos) == 5)

	assert.EqualValues(t, "Loyal Windler", videos[0].Author)
	assert.EqualValues(t, "The pleasant badger's determination", videos[0].GetTitle())
	assert.EqualValues(t, "https://video-data.localhost/blob/1131", videos[0].Url)
	assert.EqualValues(t, "Alyce Stanton", videos[1].Author)
	assert.EqualValues(t, "helpless about", videos[1].GetTitle())
	assert.EqualValues(t, "https://video-data.localhost/blob/1033", videos[1].Url)
	assert.EqualValues(t, "Kirstin Miller", videos[2].Author)
	assert.EqualValues(t, "important over", videos[2].GetTitle())
	assert.EqualValues(t, "https://video-data.localhost/blob/1091", videos[2].Url)
	assert.NoError(t, err)
}

func TestUserServiceErrorHandling(t *testing.T) {
	uOptions := usl.UserServiceOptions{
		Seed:                 123,
		SleepNs:              0,
		FailureRate:          1.0,
		ResponseOmissionRate: 0,
		MaxBatchSize:         10,
	}
	vOptions := vsl.VideoServiceOptions{
		Seed:                 123,
		TtlSeconds:           60,
		SleepNs:              0,
		FailureRate:          0,
		ResponseOmissionRate: 0,
		MaxBatchSize:         10,
	}
	vrOptions := sl.VideoRecServiceOptions{
		MaxBatchSize:    10,
		DisableFallback: true,
		DisableRetry:    true,
	}

	mockUserClient := umc.MakeMockUserServiceClient(uOptions)
	mockVideoClient := vmc.MakeMockVideoServiceClient(vOptions)
	vrService := sl.MakeVideoRecServiceServerWithMocks(
		vrOptions,
		mockUserClient,
		mockVideoClient,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := vrService.GetTopVideos(
		ctx,
		&pb.GetTopVideosRequest{UserId: 987654, Limit: 5},
	)

	assert.Error(t, err, "Error due to the UserService failure")
	assert.Contains(t, err.Error(), "Fallback is disabled")
}

func TestVideoServiceErrorHandling(t *testing.T) {
	uOptions := usl.UserServiceOptions{
		Seed:                 123,
		SleepNs:              0,
		FailureRate:          0,
		ResponseOmissionRate: 0,
		MaxBatchSize:         10,
	}
	vOptions := vsl.VideoServiceOptions{
		Seed:                 123,
		TtlSeconds:           60,
		SleepNs:              0,
		FailureRate:          1.0,
		ResponseOmissionRate: 0,
		MaxBatchSize:         10,
	}
	vrOptions := sl.VideoRecServiceOptions{
		MaxBatchSize:    10,
		DisableFallback: true,
		DisableRetry:    true,
	}

	mockUserClient := umc.MakeMockUserServiceClient(uOptions)
	mockVideoClient := vmc.MakeMockVideoServiceClient(vOptions)
	vrService := sl.MakeVideoRecServiceServerWithMocks(
		vrOptions,
		mockUserClient,
		mockVideoClient,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := vrService.GetTopVideos(
		ctx,
		&pb.GetTopVideosRequest{UserId: 123456, Limit: 5},
	)

	assert.Error(t, err, "Error due to the VideoService failure")
	assert.Contains(t, err.Error(), "Fallback is disabled")
}

func TestGetStats(t *testing.T) {
	vrOptions := sl.VideoRecServiceOptions{
		MaxBatchSize:    50,
		DisableFallback: false,
		DisableRetry:    true,
	}

	uClient := umc.MakeMockUserServiceClient(*usl.DefaultUserServiceOptions())
	vClient := vmc.MakeMockVideoServiceClient(*vsl.DefaultVideoServiceOptions())
	vrService := sl.MakeVideoRecServiceServerWithMocks(
		vrOptions,
		uClient,
		vClient,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = vrService.GetTopVideos(ctx, &pb.GetTopVideosRequest{UserId: 204054, Limit: 5})
	_, _ = vrService.GetTopVideos(ctx, &pb.GetTopVideosRequest{UserId: 204055, Limit: 3})

	resp, err := vrService.GetStats(ctx, &pb.GetStatsRequest{})
	assert.NoError(t, err)

	assert.Equal(t, uint64(2), resp.TotalRequests)
	assert.Equal(t, uint64(0), resp.TotalErrors)
	assert.Equal(t, uint64(0), resp.StaleResponses)
	assert.True(t, resp.AverageLatencyMs > 0)
}

func TestStatsTrackingWithFallback(t *testing.T) {
	vrOptions := sl.VideoRecServiceOptions{
		MaxBatchSize:    10,
		DisableFallback: false,
		DisableRetry:    true,
	}

	uOptions := usl.UserServiceOptions{
		Seed:                 123,
		SleepNs:              0,
		FailureRate:          1.0,
		ResponseOmissionRate: 0,
		MaxBatchSize:         10,
	}

	vOptions := vsl.VideoServiceOptions{
		Seed:                 123,
		TtlSeconds:           60,
		SleepNs:              0,
		FailureRate:          0,
		ResponseOmissionRate: 0,
		MaxBatchSize:         10,
	}

	mockUserClient := umc.MakeMockUserServiceClient(uOptions)
	mockVideoClient := vmc.MakeMockVideoServiceClient(vOptions)

	vrService := sl.MakeVideoRecServiceServerWithMocks(vrOptions, mockUserClient, mockVideoClient)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = vrService.GetTopVideos(ctx, &pb.GetTopVideosRequest{UserId: 987654, Limit: 5})

	uOptions.FailureRate = 0
	vOptions.FailureRate = 1.0
	mockUserClient = umc.MakeMockUserServiceClient(uOptions)
	mockVideoClient = vmc.MakeMockVideoServiceClient(vOptions)

	vrService = sl.MakeVideoRecServiceServerWithMocks(vrOptions, mockUserClient, mockVideoClient)
	_, _ = vrService.GetTopVideos(ctx, &pb.GetTopVideosRequest{UserId: 123456, Limit: 3})

	stats, err := vrService.GetStats(ctx, &pb.GetStatsRequest{})
	assert.NoError(t, err)

	assert.Equal(t, uint64(1), stats.TotalRequests)
	assert.Equal(t, uint64(0), stats.TotalErrors, "Since Fallback is enabled")
	assert.Equal(t, uint64(0), stats.UserServiceErrors, "Since Fallback is enabled")
	assert.Equal(t, uint64(0), stats.VideoServiceErrors, "Since Fallback is enabled")
	assert.Equal(t, uint64(1), stats.StaleResponses)
}

func TestFallbackTrendingVideos(t *testing.T) {
	uClient := umc.MakeMockUserServiceClient(*usl.DefaultUserServiceOptions())
	vClient := vmc.MakeMockVideoServiceClient(*vsl.DefaultVideoServiceOptions())
	vrOptions := sl.VideoRecServiceOptions{
		MaxBatchSize:    10,
		DisableFallback: false,
		DisableRetry:    true,
	}
	vrService := sl.MakeVideoRecServiceServerWithMocks(
		vrOptions,
		uClient,
		vClient,
	)

	uClient.SetInjectionConfig(context.Background(), &fipb.SetInjectionConfigRequest{
		Config: &fipb.InjectionConfig{
			FailureRate: 1,
		},
	})
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.GetTopVideosRequest{
		UserId: uint64(111111),
		Limit:  int32(3),
	}

	resp, err := vrService.GetTopVideos(ctx, req)

	assert.NoError(t, err, "Fallback is enabled")
	assert.True(t, resp.StaleResponse, "Stale response due to Fallback")
	assert.Equal(t, int(3), len(resp.Videos))
}
