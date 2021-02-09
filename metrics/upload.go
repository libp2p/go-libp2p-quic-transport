package metrics

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"cloud.google.com/go/storage"
)

const (
	uploadTimeout = time.Minute
	// Google Storage uses a default value of 16 MB here, which leads to significant
	// memory usage when a few uploads are running concurrently.
	// If the size of the uploaded file is smaller than the chunkSize, it can be uploaded
	// in a single request. Most (compressed) qlogs are around 1 kB, so setting a buffer of
	// 16 kB will probably allow > 95% of qlogs to be uploaded in a single request.
	chunkSize = 16 * 1 << 10
)

var (
	uploadInitOnce sync.Once
	storageClient  *storage.Client
)

// Upload writes all data from r to a file on Google Cloud Storage.
func Upload(name string, r io.Reader) (string, error) {
	log.Println("uploading", name)
	var initErr error
	uploadInitOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		storageClient, initErr = storage.NewClient(ctx)
	})
	if initErr != nil || storageClient == nil {
		return "", initErr
	}
	bkt := storageClient.Bucket("transport-performance-qlog")
	obj := bkt.Object(name)
	ctx, cancel := context.WithTimeout(context.Background(), uploadTimeout)
	defer cancel()
	w := obj.NewWriter(ctx)
	w.ChunkSize = chunkSize
	if _, err := io.Copy(w, r); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return fmt.Sprintf("gs://%s/%s", w.Attrs().Bucket, w.Attrs().Name), nil
}
