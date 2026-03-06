package storage

import (
	"archive/zip"
	"context"
	"testing"

	"github.com/minio/minio-go/v7"

	"github.com/gimme-cdn/gimme/test/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectStorageManager_CreateBucket(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClient{})
	err := osm.CreateBucket(context.Background(), "test", "test")
	assert.Nil(t, err)
}

func TestObjectStorageManager_CreateBucketExists(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClientBucketExists{})
	err := osm.CreateBucket(context.Background(), "test", "test")
	assert.Nil(t, err)
}

func TestObjectStorageManager_CreateBucketErr(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClientErr{})
	err := osm.CreateBucket(context.Background(), "test", "test")
	assert.Equal(t, "fail to create bucket test in location test", err.Error())
}

func TestObjectStorageManager_AddObject(t *testing.T) {
	archive, err := zip.OpenReader("../../test/test.zip")
	assert.Nil(t, err)
	defer func(archive *zip.ReadCloser) {
		err := archive.Close()
		assert.Nil(t, err)
	}(archive)

	osm := NewObjectStorageManager(&mocks.MockOSClient{})
	gimmeerr := osm.AddObject(context.Background(), "test", archive.File[0])
	assert.Nil(t, gimmeerr)
	gimmeerr = osm.AddObject(context.Background(), "test", archive.File[1])
	assert.Nil(t, gimmeerr)
	gimmeerr = osm.AddObject(context.Background(), "test", archive.File[2])
	assert.Nil(t, gimmeerr)
	gimmeerr = osm.AddObject(context.Background(), "test", archive.File[3])
	assert.Nil(t, gimmeerr)
}

func TestObjectStorageManager_AddObjectErr(t *testing.T) {
	archive, err := zip.OpenReader("../../test/test.zip")
	assert.Nil(t, err)
	defer func(archive *zip.ReadCloser) {
		err := archive.Close()
		assert.Nil(t, err)
	}(archive)

	osm := NewObjectStorageManager(&mocks.MockOSClientErr{})
	gimmeerr := osm.AddObject(context.Background(), "test", archive.File[1])
	assert.Equal(t, "fail to put object test in the object storage", gimmeerr.Error())
}

func TestObjectStorageManager_GetObject(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClient{})
	obj, err := osm.GetObject(context.Background(), "test")
	assert.Equal(t, &minio.Object{}, obj)
	assert.Nil(t, err)
}

func TestObjectStorageManager_GetObjectErr(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClientErr{})
	obj, err := osm.GetObject(context.Background(), "test")
	assert.Equal(t, "fail to get object test from the object storage", err.Error())
	assert.Nil(t, obj)
}

func TestObjectStorageManager_ObjectExists(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClient{})
	res := osm.ObjectExists(context.Background(), "test")
	assert.True(t, res)
}

func TestObjectStorageManager_ObjectExistsFalsy(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClientErr{})
	res := osm.ObjectExists(context.Background(), "test")
	assert.False(t, res)
}

func TestObjectStorageManager_ListObjects(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClient{})
	objs := osm.ListObjects(context.Background(), "test")
	assert.Equal(t, "test", objs[0].ETag)
}

func TestObjectStorageManager_RemoveObjects(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClient{})
	err := osm.RemoveObjects(context.Background(), "test")
	assert.Nil(t, err)
}

// TestObjectStorageManager_RemoveObjects_ListErr checks that a listing error
// (object.Err != nil) is collected and returned as a GimmeError.
func TestObjectStorageManager_RemoveObjects_ListErr(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClientErr{})
	err := osm.RemoveObjects(context.Background(), "test")
	assert.NotNil(t, err)
}

// TestObjectStorageManager_RemoveObjects_DeleteErr checks that a deletion error
// (rErr.Err != nil) is collected and returned as a GimmeError.
func TestObjectStorageManager_RemoveObjects_DeleteErr(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClientRemoveErr{})
	err := osm.RemoveObjects(context.Background(), "test")
	assert.NotNil(t, err)
}

// TestObjectStorageManager_Ping_Success checks that Ping succeeds when the
// bucket exists.
func TestObjectStorageManager_Ping_Success(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClientBucketExists{})
	// CreateBucket first so osm.bucketName is set.
	require.Nil(t, osm.CreateBucket(context.Background(), "test", "test"))
	err := osm.Ping(context.Background())
	assert.Nil(t, err)
}

// TestObjectStorageManager_Ping_BucketNotFound checks that Ping returns an error
// when the bucket does not exist.
func TestObjectStorageManager_Ping_BucketNotFound(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClient{})
	// MockOSClient.BucketExists returns false, nil → bucket not found.
	err := osm.Ping(context.Background())
	assert.NotNil(t, err)
}

// TestObjectStorageManager_Ping_ClientErr checks that Ping returns an error
// when BucketExists returns an error.
func TestObjectStorageManager_Ping_ClientErr(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClientErr{})
	err := osm.Ping(context.Background())
	assert.NotNil(t, err)
}
