package gimmecdn

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/gimme-cdn/gimme/internal/domain/gimmecdn/spi/stubs"

	"github.com/stretchr/testify/assert"
)

var objStorage = stubs.NewObjectStorageStub()
var cdn = NewCDNEngine(objStorage)

func TestCdnEngine_ShouldUploadAPackage(t *testing.T) {
	fileName := "../../../test/test.zip"
	fi, _ := os.Stat(fileName)
	size := fi.Size()
	reader, _ := os.Open(fileName)
	defer reader.Close()
	err := cdn.CreatePackage("test", "1.0.0", reader, size)
	assert.Nil(t, err)

	objs := cdn.GetPackageFiles("test", "1.0.0")
	assert.Equal(t, 4, len(objs))

	file, _ := cdn.GetFileFromPackage("test", "1.0.0", "/awesome-lib.min.js")
	buf := new(strings.Builder)
	io.Copy(buf, file.File)
	assert.Equal(t, "console.log('Hello world !!!!');\n", buf.String())
}

func TestCdnEngine_ShouldHaveErrorWhileUploadingExistingPackage(t *testing.T) {
	fileName := "../../../test/test.zip"
	fi, _ := os.Stat(fileName)
	size := fi.Size()
	reader, _ := os.Open(fileName)
	defer reader.Close()
	err := cdn.CreatePackage("test", "1.0.0", reader, size)
	assert.Equal(t, "the package test@1.0.0 already exists", err.String())
}

func TestCdnEngine_ShouldHaveErrorWhileGettingUnexistingPackageFile(t *testing.T) {
	file, err := cdn.GetFileFromPackage("test", "1.0.0", "/wrong")
	assert.Nil(t, file)
	assert.Equal(t, "Not found", err.String())
}

func TestCdnEngine_ShouldHaveErrorWhileGettingInvalidSemverFile(t *testing.T) {
	file, err := cdn.GetFileFromPackage("test", "1.a.b", "/wrong")
	assert.Nil(t, file)
	assert.Equal(t, "invalid version (asked version must be semver compatible)", err.String())
}

func TestCdnEngine_ShouldGetMajorVersionFile(t *testing.T) {
	fileName := "../../../test/test.zip"
	fi, _ := os.Stat(fileName)
	size := fi.Size()
	reader, _ := os.Open(fileName)
	defer reader.Close()
	err := cdn.CreatePackage("test", "1.1.0", reader, size)
	assert.Nil(t, err)

	file, _ := cdn.GetFileFromPackage("test", "1", "/awesome.min.css")
	buf := new(strings.Builder)
	io.Copy(buf, file.File)
	assert.Equal(t, "h3 {\n    color: blue;\n}\n\np {\n    color: red;\n}", buf.String())
}
