# gimme

A self-hosted CDN solution written in Go

## Run a local object storage

```shell
docker run \
  -p 9000:9000 \
  -p 9001:9001 \
  minio/minio server /data --console-address ":9001"
```

## Configuration

Gimme configuration is stored in a yaml file.

You need to created it before running the application.

| Parameter      | Description             |
|----------------|-------------------------|
| secret         | Application secret      |
| admin.user     | Administration user     |
| admin.password | Administration password |
| s3.url         | Object storage url      |
| s3.key         | Object storage key      |
| s3.secret      | Object storage secret   |
| s3.bucketName  | Bucket name             |
| s3.location    | Object storage location |
| ssl            | Enable SSL              |

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
  ssl: true (default true)
```


## Run application

> **/!\ You must create the access key / secret from Minio admin console if you are using a local minio object storage.**

### From sources
```shell
go run main.go
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
