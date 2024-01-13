# Sessions

HTTP session cookie management for Go. It allows you to set both data that persists between requests (session data), and data that persists until the next request (flash data).

Unlike typical session libraries for Go, sessions uses the request's context for storage within the same request liftime, allowing you to access the session between multiple handlers or HTTP middleware, as well as within test cases that do not use an HTTP server.

## Usage

Install the package with `go get`:

```sh
$ go get github.com/bentranter/sessions
```

Then you can set session and flash data:

```go
// TODO
```
