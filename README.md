# go-reverse-proxy
a simple go reverse proxy that does request body rewrites

## Running the proxy

```bash
export FORWARDED_URL: https://postman-echo.com/post
export PATH_PREEFIX: /
export SKIP_SSL_VALIDATION: false
export X_FORWARDED_HOST_HEADER: X-Original-Host
export X_FORWARDED_PATH_HEADER: X-Original-Path
```

from source

```
go run main.go
```

from binary

```
./go-reverse-proxy
```

## using curl

```bash
curl --location --request POST localhost:8080   --data "www.google.com This is expected to be sent back as part of response body."
```

## Help

```
NAME: go-reverse-proxy

USAGE:
    [global options] command [command options] [arguments...]

COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --port value, -p value           (default: "8080") [$PORT]
   --forwarded-url value, -f value   [$FORWARDED_URL]
   --path-prefix value, -x value     [$PATH_PREFIX]
   --skip-ssl-validation, -k         [$SKIP_SSL_VALIDATION]
   --x-forwarded-host-header value   [$X_FORWARDED_HOST_HEADER]
   --x-forwarded-path-header value   [$X_FORWARDED_PATH_HEADER]
   --help, -h                       show help
   --version, -v                    print the version
```