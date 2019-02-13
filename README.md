# go-reverse-proxy
a simple go reverse proxy that does request body rewrites

## Testing

setup

```bash
export PORT=8080
export FORWARDED_URL=https://postman-echo.com/post
export REQUEST_BODY_FIND=www[.]google[.]com
export REQUEST_BODY_REPLACE=www.example.com
```

from source

```
go run main.go
```

from binary

```
./go-reverse-proxy
```

on the client

```bash
curl --location --request POST localhost:8080   --data "www.google.com This is expected to be sent back as part of response body."
```