# gimme

A CDN prototype written in Go

## Run Minio object storage

```shell
docker run \
  -p 9000:9000 \
  -p 9001:9001 \
  minio/minio server /data --console-address ":9001"
```

## Run application

```shell
go run main.go
```

## Upload content to the CDN

WIP...

## Load library from the CDN

WIP...
