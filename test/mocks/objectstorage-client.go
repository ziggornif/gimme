package mocks

import (
	"context"
	"io"
	"sort"
	"strings"

	"github.com/minio/minio-go/v7"
)

type MockOSClient struct {
	PutObjectOptions []minio.PutObjectOptions
	Objects          []minio.ObjectInfo
}

func (osc *MockOSClient) MakeBucket(_ context.Context, _ string, _ minio.MakeBucketOptions) error {
	return nil
}

func (osc *MockOSClient) BucketExists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (osc *MockOSClient) PutObject(_ context.Context, _ string, _ string, _ io.Reader, _ int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	osc.PutObjectOptions = append(osc.PutObjectOptions, opts)
	return minio.UploadInfo{Size: 10}, nil
}
func (osc *MockOSClient) GetObject(_ context.Context, _ string, _ string, _ minio.GetObjectOptions) (*minio.Object, error) {
	return &minio.Object{}, nil
}

func (osc *MockOSClient) ListObjects(ctx context.Context, _ string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	ch := make(chan minio.ObjectInfo)
	go func() {
		defer close(ch)
		objects := make([]minio.ObjectInfo, len(osc.Objects))
		copy(objects, osc.Objects)
		if len(objects) == 0 {
			objects = []minio.ObjectInfo{{Key: "test/file.js", ETag: "test"}}
		}
		sort.Slice(objects, func(i, j int) bool { return objects[i].Key < objects[j].Key })
		seen := map[string]struct{}{}
		for _, object := range objects {
			if !strings.HasPrefix(object.Key, opts.Prefix) || object.Key <= opts.StartAfter {
				continue
			}
			if !opts.Recursive {
				rest := strings.TrimPrefix(object.Key, opts.Prefix)
				child, _, found := strings.Cut(rest, "/")
				if !found {
					continue
				}
				object = minio.ObjectInfo{Key: opts.Prefix + child + "/"}
				if _, exists := seen[object.Key]; exists {
					continue
				}
				seen[object.Key] = struct{}{}
			}
			select {
			case <-ctx.Done():
				return
			case ch <- object:
			}
		}
	}()
	return ch
}

func (osc *MockOSClient) RemoveObjects(_ context.Context, _ string, _ <-chan minio.ObjectInfo, _ minio.RemoveObjectsOptions) <-chan minio.RemoveObjectError {
	ch := make(chan minio.RemoveObjectError, 1)
	defer close(ch)
	return ch
}
