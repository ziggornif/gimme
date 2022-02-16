MAIN=main.go

audit:
	gosec ./..

build:
	go build -ldflags "-w -s" -o gimme $(MAIN)
