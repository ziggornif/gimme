package main

import (
	"github.com/gimme-cdn/gimme/internal/application"
)

func main() {
	app := application.NewApplication()
	app.Run()
}
