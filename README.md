# gimme

A CDN prototype written in Go

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
secret: secret
s3:
  url: localhost:9000
  key: s3key
  secret: s3secret
  bucketName: gimme
  location: eu-west-1
  ssl: false
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

## How does it work ?

![schema](./schema.png)

The CDN core is based on a S3 object storage.

Each package will be stored in a bucket as a folder named `<package>@<version>` to manage packages versioning.

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
