package storage

import (
	"archive/zip"
	"testing"

	"github.com/minio/minio-go/v7"

	"github.com/gimme-cli/gimme/resources/tests/mocks"

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
	assert.Equal(t, "Fail to create bucket test in location test", err.Error())
}

func TestObjectStorageManager_AddObject(t *testing.T) {
	archive, err := zip.OpenReader("../../resources/tests/test.zip")
	assert.Nil(t, err)
	defer archive.Close()

	osm := NewObjectStorageManager(&mocks.MockOSClient{})
	err = osm.AddObject("test", archive.File[0])
	err = osm.AddObject("test", archive.File[1])
	err = osm.AddObject("test", archive.File[2])
	err = osm.AddObject("test", archive.File[3])
	assert.Nil(t, err)
}

func TestObjectStorageManager_AddObjectErr(t *testing.T) {
	archive, err := zip.OpenReader("../../resources/tests/test.zip")
	assert.Nil(t, err)
	defer archive.Close()

	osm := NewObjectStorageManager(&mocks.MockOSClientErr{})
	err = osm.AddObject("test", archive.File[1])
	assert.Equal(t, "Fail to put object test in the object storage", err.Error())
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
	assert.Equal(t, "Fail to get object test from the object storage", err.Error())
	assert.Nil(t, obj)
}
