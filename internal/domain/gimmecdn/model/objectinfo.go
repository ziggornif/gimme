package model

import "time"

type ObjectInfos struct {
	Key          string
	LastModified time.Time
	Size         int64
	ContentType  string
}
