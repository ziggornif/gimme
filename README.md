# gimme

A CDN prototype written in Go

## Run a local object storage

```shell
docker run \
  -p 9000:9000 \
  -p 9001:9001 \
  minio/minio server /data --console-address ":9001"
```

## Run application

### From sources
```shell
# /!\ Use your own values
GIMME_ADMIN_USER=gimmeadmin \
GIMME_ADMIN_PASSWORD=gimmeadmin \
GIMME_SECRET=secret \
go run main.go
```

### With docker

WIP...

## How does it work ?

![schema](./schema.png)

The CDN core is based on a S3 object storage.

Each package will be stored in a bucket as a folder named <package>@<version> to manage packages versioning.

The project use the Minio SDK to be compatible with all S3 providers (Amazon, OVH, Clevercloud ...)

> Important : cloud compatibility has not been tested for the moment

## Usage

> There is no frontend to use the CDN at the moment.
> 
> I will work on it once the backend part will finish

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
