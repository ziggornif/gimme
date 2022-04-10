package utils

import (
	"fmt"
	"io"
	"os"
)

// CopyFile - copy source file to dest
func CopyFile(src string, dst string) {
	source, err := os.Open(src)
	if err != nil {
		fmt.Println("Error while opening src file")
	}
	defer func(source *os.File) {
		err := source.Close()
		if err != nil {
			fmt.Println("Error while closing file")
		}
	}(source)

	destination, err := os.Create(dst)
	if err != nil {
		fmt.Println("Error while creating file")
	}
	defer func(destination *os.File) {
		err := destination.Close()
		if err != nil {
			fmt.Println("Error while creating file")
		}
	}(destination)
	_, err = io.Copy(destination, source)
	if err != nil {
		fmt.Println("Error during file copy")
	}
}
