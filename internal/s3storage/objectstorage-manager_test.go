package s3storage

import (
	"archive/zip"
	"testing"

	"github.com/minio/minio-go/v7"

	"github.com/gimme-cdn/gimme/test/mocks"

	"github.com/stretchr/testify/assert"
)

func TestObjectStorageManager_CreateBucket(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClient{})
	err := osm.CreateBucket("test", "test")
	assert.Nil(t, err)
}

func TestObjectStorageManager_CreateBucketExists(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClientBucketExists{})
	err := osm.CreateBucket("test", "test")
	assert.Nil(t, err)
}

func TestObjectStorageManager_CreateBucketErr(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClientErr{})
	err := osm.CreateBucket("test", "test")
	assert.Equal(t, "fail to create bucket test in location test", err.String())
}

func TestObjectStorageManager_AddObject(t *testing.T) {
	archive, err := zip.OpenReader("../../test/test.zip")
	assert.Nil(t, err)
	defer func(archive *zip.ReadCloser) {
		err := archive.Close()
		assert.Nil(t, err)
	}(archive)

	osm := NewObjectStorageManager(&mocks.MockOSClient{})
	gimmeerr := osm.AddObject("test", archive.File[0])
	assert.Nil(t, gimmeerr)
	gimmeerr = osm.AddObject("test", archive.File[1])
	assert.Nil(t, gimmeerr)
	gimmeerr = osm.AddObject("test", archive.File[2])
	assert.Nil(t, gimmeerr)
	gimmeerr = osm.AddObject("test", archive.File[3])
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
	gimmeerr := osm.AddObject("test", archive.File[1])
	assert.Equal(t, "fail to put object test in the object s3storage", gimmeerr.String())
}

func TestObjectStorageManager_GetObject(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClient{})
	obj, err := osm.GetObject("test")
	assert.Equal(t, &minio.Object{}, obj)
	assert.Nil(t, err)
}

func TestObjectStorageManager_GetObjectErr(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClientErr{})
	obj, err := osm.GetObject("test")
	assert.Equal(t, "fail to get object test from the object s3storage", err.String())
	assert.Nil(t, obj)
}

func TestObjectStorageManager_ObjectExists(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClient{})
	res := osm.ObjectExists("test")
	assert.True(t, res)
}

func TestObjectStorageManager_ObjectExistsFalsy(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClientErr{})
	res := osm.ObjectExists("test")
	assert.False(t, res)
}

func TestObjectStorageManager_ListObjects(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClient{})
	objs := osm.ListObjects("test")
	assert.Equal(t, "test", objs[0].Key)
}

func TestObjectStorageManager_RemoveObjects(t *testing.T) {
	osm := NewObjectStorageManager(&mocks.MockOSClient{})
	err := osm.RemoveObjects("test")
	assert.Nil(t, err)
}
