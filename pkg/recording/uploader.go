package recording

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"voxray-go/pkg/metrics"
)

// RecordingJob describes a finalized recording to upload.
type RecordingJob struct {
	LocalPath string
	Bucket    string
	Key       string
}

// Uploader uploads recordings to S3 using a fixed-size worker pool.
// THREAD SAFETY: Enqueue may be called from any goroutine; workers are the only consumers of jobs; Shutdown must not be called concurrently with Enqueue.
type Uploader struct {
	client     *s3.Client
	jobs       chan RecordingJob
	wg         sync.WaitGroup
	maxRetries int
}

// NewUploader creates a new uploader with the given worker count, queue size, and max retries (0 = default 3).
// SCALING: one worker per ~N concurrent sessions uploading at once; tune for S3 bandwidth.
func NewUploader(ctx context.Context, workerCount, queueSize, maxRetries int) (*Uploader, error) {
	if workerCount <= 0 {
		workerCount = 2
	}
	if queueSize <= 0 {
		queueSize = 32
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	u := &Uploader{
		client:     s3.NewFromConfig(awsCfg),
		jobs:       make(chan RecordingJob, queueSize),
		maxRetries: maxRetries,
	}
	for i := 0; i < workerCount; i++ {
		u.wg.Add(1)
		go u.worker(ctx)
	}
	return u, nil
}

// Enqueue adds a job to the upload queue.
func (u *Uploader) Enqueue(job RecordingJob) error {
	select {
	case u.jobs <- job:
		metrics.RecordingJobsEnqueuedTotal.Inc()
		metrics.RecordingQueueDepth.Set(float64(len(u.jobs)))
		return nil
	default:
		metrics.RecordingJobsFailedTotal.Inc()
		return fmt.Errorf("recording queue full")
	}
}

// Shutdown waits for workers to finish processing pending jobs until ctx is done.
func (u *Uploader) Shutdown(ctx context.Context) {
	close(u.jobs)
	select {
	case <-ctx.Done():
	case <-func() <-chan struct{} {
		ch := make(chan struct{})
		go func() {
			u.wg.Wait()
			close(ch)
		}()
		return ch
	}():
	}
}

func (u *Uploader) worker(ctx context.Context) {
	for job := range u.jobs {
		if err := u.uploadOnce(ctx, job); err != nil {
			metrics.RecordingJobsFailedTotal.Inc()
		} else {
			metrics.RecordingJobsSuccessTotal.Inc()
		}
		metrics.RecordingQueueDepth.Set(float64(len(u.jobs)))
	}
	u.wg.Done()
}

func (u *Uploader) uploadOnce(ctx context.Context, job RecordingJob) error {
	if job.LocalPath == "" || job.Bucket == "" || job.Key == "" {
		return fmt.Errorf("invalid recording job: %+v", job)
	}
	f, err := os.Open(job.LocalPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// MEMORY: stream file to S3 rather than buffering full WAV in heap.
	input := &s3.PutObjectInput{
		Bucket: aws.String(job.Bucket),
		Key:    aws.String(job.Key),
		Body:   f,
	}
	var lastErr error
	backoff := 100 * time.Millisecond
	for attempt := 0; attempt <= u.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if _, err := f.Seek(0, 0); err != nil {
				return err
			}
		}
		_, lastErr = u.client.PutObject(ctx, input)
		if lastErr == nil {
			_ = os.Remove(job.LocalPath)
			return nil
		}
	}
	return lastErr
}

// BuildS3Key builds a key like "<basePath>/yyyy/mm/dd/<callID>.wav".
func BuildS3Key(basePath, callID, format string, t time.Time) string {
	if format == "" {
		format = "wav"
	}
	if basePath == "" {
		basePath = "recordings"
	}
	basePath = filepath.ToSlash(basePath)
	if basePath[len(basePath)-1] == '/' {
		basePath = basePath[:len(basePath)-1]
	}
	datePath := fmt.Sprintf("%04d/%02d/%02d", t.Year(), t.Month(), t.Day())
	filename := fmt.Sprintf("%s.%s", callID, format)
	return fmt.Sprintf("%s/%s/%s", basePath, datePath, filename)
}

