package model

import "io"

type CDNObject struct {
	File        io.Reader
	Size        int64
	ContentType string
}
