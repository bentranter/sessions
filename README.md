# Sessions

![testing](https://github.com/bentranter/sessions/actions/workflows/test.yml/badge.svg)
[![godoc](https://godoc.org/github.com/bentranter/sessions?status.svg)](https://godoc.org/github.com/bentranter/sessions)

HTTP session cookie management for Go. It allows you to set both data that persists between requests (session data), and data that persists until the next request (flash data).

Unlike typical session libraries for Go, sessions uses the request's context for storage within the same request liftime, allowing you to access the session between multiple handlers or HTTP middleware, as well as within test cases that do not use an HTTP server.

## Usage

Install the package with `go get`:

```sh
$ go get github.com/bentranter/sessions
```

Then you can set session and flash data:

```go
// Create a new session manager.
session := New(GenerateRandomKey(32), Options{
    // Set a custom session name (default is "_session").
    Name: "_example_session_name",
})

// Get and set session data.
http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodGet {
        name := session.Get(r, "name")
        fmt.Fprintf(w, "Your name is: %s\n", name)
        return
    }
    session.Set(w, r, "name", "Ben")
    http.Redirect(w, r, "/", http.StatusSeeOther)
})

// List and display all session data as JSON.
http.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
    values := session.List(r)

    data, err := json.MarshalIndent(values, "", "  ")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Write(data)
})

// Delete a flash message.
http.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
    session.Delete(w, r, "name")
    http.Redirect(w, r, "/", http.StatusFound)
})

// List and set flash messages.
http.HandleFunc("/flash", func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodGet {
        flashes := session.Flashes(w, r)

        data, err := json.MarshalIndent(flashes, "", "  ")
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        w.Write(data)
        return
    }

    if r.URL.Query().Has("flash") {
        session.Flash(w, r, "notice", "This is a flash message")
    }
    http.Redirect(w, r, "/flash", http.StatusSeeOther)
})

// Clear all session data.
http.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
    session.Reset(w, r)
    http.Redirect(w, r, "/", http.StatusFound)
})
```
