# gimme

A self-hosted CDN solution written in Go.

## What is a CDN ?

> A content delivery network (CDN) refers to a geographically distributed group of servers which work together to provide fast delivery of Internet content.
>
> A CDN allows for the quick transfer of assets needed for loading Internet content including HTML pages, javascript files, stylesheets, images, and videos. […]
> 
> https://www.cloudflare.com/learning/cdn/what-is-a-cdn/

## Configuration

Gimme configuration is stored in a yaml file.

You need to created it before running the application.

| Parameter      | Description                                |
|----------------|--------------------------------------------|
| secret         | Application secret                         |
| admin.user     | Administration user                        |
| admin.password | Administration password                    |
| port           | Exposition port (default 8080)             |
| s3.url         | Object storage url                         |
| s3.key         | Object storage key                         |
| s3.secret      | Object storage secret                      |
| s3.bucketName  | Bucket name                                |
| s3.location    | Object storage location                    |
| ssl            | Enable SSL (default true)                  |
| metrics        | Enable OpenMetrics endpoint (default true) |

### Example

```yaml
admin:
  user: gimmeadmin
  password: gimmeadmin
port: 8080 # (default 8080)
secret: secret
s3:
  url: your.s3.url.cloud
  key: s3key
  secret: s3secret
  bucketName: gimme (default gimme)
  location: eu-west-1
  ssl: true
```


## Run application

### From sources
```shell
make build
```

```shell
./gimme
```

### With docker

Execute the following command to run a gimme instance.

```shell
docker run -p 8080:8080 -v `pwd`/gimme.yml:/config/gimme.yml \
  ziggornif/gimme:latest
```

Or with docker compose :

```yaml
version: "3.9"
services:
  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    ports:
      - "9000:9000"
      - "9001:9001"
  gimme:
    image: ziggornif/gimme:latest
    ports:
      - "8080:8080"
    volumes:
      - ./gimme.yml:/config/gimme.yml
```

## Run a local object storage

```shell
docker run \
  -p 9000:9000 \
  -p 9001:9001 \
  minio/minio server /data --console-address ":9001"
```

> **/!\ You must create the access key / secret from Minio admin console if you are using a local minio object storage.**

## Architecture

![schema](./schema.png)

The CDN core is based on a S3 object storage.

Each package will be stored in a bucket as a folder named `<package>@<version>` to manage packages versioning.

The project use the Minio SDK to be compatible with all S3 providers.

**💡NOTE : A caching system in front of the CDN is strongly recommended to improve performance.**

## Usage

### Create access token

Use your `GIMME_ADMIN_USER` and `GIMME_ADMIN_PASSWORD` as a basic authentication to create a new access token.
```shell
curl --location --request POST 'http://localhost:8080/create-token' \
--header 'Authorization: Basic Z2ltbWVhZG1pbjpnaW1tZWFkbWlu' \
--header 'Content-Type: application/json' \
--data-raw '{
    "name": "awesome-token",
    "expirationDate": "2022-02-17"
}'
```

> NOTE : If the `expirationDate` is not set, the token expiration will be set to 15 minutes

### Upload content to the CDN

The `POST /packages` route allows you to upload content to the CDN.

This route currently only accept a zip archive file which contains the files to import.

You also must provide a package name and a version.

**This route needs a valid access-token to process the upload.**

**Example :**
```shell
curl --location --request POST 'http://localhost:8080/packages' \
--header 'Authorization: Bearer xxxxxxx' \
--form 'file=@"tests/awesome-lib.zip"' \
--form 'name="awesome-lib"' \
--form 'version="1.0.0"'
```

### Load library from the CDN

Once your package uploaded in the CDN, you can use it from the following URL.

```text
<base_url>/gimme/<package>@<version>/<your_file>.<js|css|...>
```

**Example :**

Open the `tests/index.html` file. 

This file load js and css dependencies from the CDN (uploaded with the previous curl command).

```html
<link rel="stylesheet" href="http://localhost:8080/gimme/awesome-lib@1.0.0/awesome.min.css">
...
<script src="http://localhost:8080/gimme/awesome-lib@1.0.0/awesome-lib.min.js" type="module"></script>
```

You can try it with your favourite http server tool.

```shell
cd tests
npx http-server --cors .
```

### Show package content

The root package URL package allows to view the list of available files.

Example :
```text
<base_url>/gimme/awesome-lib@3.0.0
```

![interface](./examples/interface.png)


## Setup a basic cache system with Nginx

Here is a docker-compose deployment file with a simple Nginx cache system.

The following configuration will cache all called CDN files during 10 minutes.

```yaml
version: "3.9"
services:
  front:
    image: nginx
    volumes:
      - ./default.conf:/etc/nginx/nginx.conf
    ports:
      - "80:80"
  gimme:
    image: ziggornif/gimme:latest
    ports:
      - "8080:8080"
    volumes:
      - ./gimme-conf/gimme.yml:/config/gimme.yml
```

And the Nginx `default.conf` config file :
```text
events {

}
http {
  proxy_cache_path  /cache  levels=1:2 keys_zone=STATIC:10m inactive=24h  max_size=1g;
  server {
    listen 80;

    location ~* \.(?:jpg|jpeg|gif|png|ico|woff2|css|js)$ {
		proxy_pass http://gimme:8080;
		proxy_set_header       Host $host;
        proxy_buffering        on;
        proxy_cache            STATIC;
        proxy_cache_valid      200  10m;
	}

    location / {
      proxy_pass http://gimme:8080;
    }
  }
}
```

## Deployment

The [example deployment folder](./examples/deployment) contains docker and kubernetes deployment configuration examples.

## Monitoring

Each CDN running instance expose an OpenMetrics endpoint.

This endpoint is useful to monitor instances with Prometheus and Grafana.

A monitoring configuration with a default prometheus configuration and a grafana dashboard is avaiable in [the monitoring example dir](./examples/monitoring).

There is also a docker-compose deployment configuration with prometheus and grafana here : [link](./examples/deployment/docker-compose/with-local-s3)