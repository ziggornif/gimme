package main

import (
	"archive/zip"
	"fmt"
	"log"
	"net/http"
	"regexp"

	"github.com/drouian-m/gimme/files"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	filesManager, err := files.NewFilesManager(files.MinioConfig{
		Endpoint:        "localhost:9000",
		AccessKeyID:     "test",
		SecretAccessKey: "golangtest",
		UseSSL:          false,
	})
	if err != nil {
		log.Fatalln(err)
	}

	err = filesManager.CreateBucket("gimme", "eu-west-1")
	if err != nil {
		log.Fatalln(err)
	}

	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()
	router.Use(cors.Default())

	router.GET("/gimme/:package/*file", func(c *gin.Context) {
		var filePath string
		file := c.Param("file")
		if len(file) > 0 {
			filePath += fmt.Sprintf("%v%v", c.Param("package"), file)
		} else {
			filePath = c.Param("package")
		}
		object, err := filesManager.GetObject(filePath)
		if err != nil {
			fmt.Println(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		infos, _ := object.Stat()

		if infos.Size == 0 {
			c.Status(http.StatusNotFound)
			return
		}
		c.DataFromReader(http.StatusOK, infos.Size, infos.ContentType, object, nil)
	})

	router.POST("/packages", func(c *gin.Context) {
		file, _ := c.FormFile("file")
		name := c.PostForm("name")
		version := c.PostForm("version")

		src, _ := file.Open()
		defer src.Close()

		//IDEA: did I need to also support single file import ?
		// we can detect file type here
		// - js / css case => upload directly in the object storage in a new folder <package>@<version>/<file>
		// - archive => unzip and upload each file in the object storage (same folder convention)

		archive, err := zip.NewReader(src, file.Size)
		if err != nil {
			panic(err)
		}

		folderName := fmt.Sprintf("%s@%s", name, version)

		var re = regexp.MustCompile(fmt.Sprintf(`^%s`, name))

		for _, f := range archive.File {

			//TODO: improve this. I don't know if it's the safest solution
			// because it's presume that project name and package name are equals
			fileName := re.ReplaceAllString(f.FileHeader.Name, folderName)

			filesManager.AddObject(fileName, f)
			fmt.Println("unzipping file ", f.Name)
		}

		c.Status(http.StatusNoContent)
		return
	})

	router.Run(":8080")
}
