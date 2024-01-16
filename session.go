/*
(Aliasing securecookie's methods may constitute redistribution, so we
include the original securecookie license in accordance with the BSD-3
clause license terms.)

Copyright (c) 2023 The Gorilla Authors. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

	 * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
	 * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
	 * Neither the name of Google Inc. nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

// Package sessions implements HTTP sessions.
//
// (Description TODO)
package sessions

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/securecookie"
)

// Version is the released version of the library.
const Version = "1.0.1"

type sessionCtxKeyType struct{}

const (
	defaultSessionName = "_session"
	defaultMaxAge      = 86400 * 365
)

var (
	sessionCtxKey = sessionCtxKeyType{}
)

func init() {
	// Register the encodings used in this package with gob such that we can
	// successfully save session data in the session.
	gob.Register([]interface{}{})
	gob.Register(map[string]interface{}{})
	gob.Register(&session{})
}

// GenerateRandomKey creates a random key with the given length in bytes. On
// failure, returns nil.
//
// Note that keys created using `GenerateRandomKey()` are not automatically
// persisted. New keys will be created when the application is restarted, and
// previously issued cookies will not be able to be decoded.
//
// Callers should explicitly check for the possibility of a nil return, treat
// it as a failure of the system random number generator, and not continue.
//
// This function is an alias of securecookie.GenerateRandomKey, and is
// provided as a convenience method to avoid the additional import of the
// securecookie library.
func GenerateRandomKey(length int) []byte {
	return securecookie.GenerateRandomKey(length)
}

// A Session manages setting and getting data from the cookie that stores the
// session data.
type Session struct {
	sc    *securecookie.SecureCookie
	name  string
	quiet bool
}

// Options to customize the behaviour of the session.
type Options struct {
	// The name of the cookie (default is "_session").
	Name string

	// MaxAge of the cookie before expiry (default is 365 days). Set it to
	// -1 for no expiry.
	MaxAge int

	// Quiet defines whether or not to suppress all error and warning messages
	// from the library. Defaults to false, since when correctly used, these
	// messages should never appear. Setting to true may suppress critical
	// error and warning messages.
	Quiet bool
}

// New creates a new session manager with the given key.
func New(secret []byte, opts ...Options) *Session {
	var o Options
	for _, opt := range opts {
		o = opt
	}

	if o.Name == "" {
		o.Name = defaultSessionName
	}

	switch o.MaxAge {
	case 0:
		// Default to one year, since some browsers don't set their cookies
		// with the same defaults.
		o.MaxAge = defaultMaxAge
	case -1:
		o.MaxAge = 0
	}

	sc := securecookie.New(secret, nil)
	sc.MaxAge(o.MaxAge)

	return &Session{
		sc:    sc,
		name:  o.Name,
		quiet: o.Quiet,
	}
}

// A session holds the session data. It contains two fields:
//
//   - "data" for long-lived session data that persists between requests,
//   - "flashes" for session data that should be deleted as soon as it is shown.
type session struct {
	Data    map[string]interface{}
	Flashes map[string]interface{}
}

// init ensures that both of the underlying maps have been initialized.
func (s *session) init() {
	if s.Data == nil {
		s.Data = make(map[string]interface{})
	}
	if s.Flashes == nil {
		s.Flashes = make(map[string]interface{})
	}
}

// fromReq returns the map of session values from the request. It will
// never return a nil map, instead, the map will be an initialized empty map
// in the case where the session has no data.
func (s *Session) fromReq(r *http.Request) *session {
	// Fastpath: if the context has already been decoded, access the
	// underlying map and return the value associated with the given key.
	v := r.Context().Value(sessionCtxKey)
	if v != nil {
		ss, ok := v.(*session)
		if ok {
			return ss
		}
	}

	cookie, err := r.Cookie(s.name)
	if err != nil {
		// The only error that can be returned by r.Cookie() is ErrNoCookie,
		// so if the error is not nil, that means that the cookie doesn't
		// exist. When that is the case, the value associated with the given
		// key is guaranteed to be nil, so we return nil.
		ss := &session{}
		ss.init()
		return ss
	}

	ss := &session{}
	if err := s.sc.Decode(s.name, cookie.Value, ss); err != nil {
		if !s.quiet {
			fmt.Printf("sessions: [ERROR] failed to decode session from cookie: %+v\n", err)
		}
		ss.init()
		return ss
	}
	return ss
}

// saveCtx saves a map of session data in the current request's context. It
// also updates the Set-Cookie header of the
func (s *Session) saveCtx(w http.ResponseWriter, r *http.Request, session *session) {
	ctx := context.WithValue(r.Context(), sessionCtxKey, session)
	r2 := r.Clone(ctx)
	*r = *r2

	encoded, err := s.sc.Encode(s.name, session)
	if err != nil {
		if !s.quiet {
			fmt.Printf("sessions: [ERROR} failed to encode cookie: %+v\n", err)
		}
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.name,
		MaxAge:   defaultMaxAge,
		Expires:  time.Now().UTC().Add(time.Duration(defaultMaxAge * time.Second)),
		Value:    encoded,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	})
}

// Session creates a new session from the given HTTP request. If the
// request already has a cookie with an associated session, the session data
// is created from the cookie. If not, a new session is created.
func (s *Session) Get(r *http.Request, key string) interface{} {
	data := s.fromReq(r)
	return data.Data[key]
}

// List returns all key value pairs of session data from the given request.
func (s *Session) List(r *http.Request) map[string]interface{} {
	return s.fromReq(r).Data
}

// Set sets or updates the given value on the session.
func (s *Session) Set(w http.ResponseWriter, r *http.Request, key string, value interface{}) {
	data := s.fromReq(r)
	data.Data[key] = value
	s.saveCtx(w, r, data)
}

// Delete deletes and returns the session value with the given key.
func (s *Session) Delete(w http.ResponseWriter, r *http.Request, key string) interface{} {
	data := s.fromReq(r)
	value := data.Data[key]
	delete(data.Data, key)
	s.saveCtx(w, r, data)
	return value
}

// Reset resets the session, deleting all values.
func (s *Session) Reset(w http.ResponseWriter, r *http.Request) {
	s.saveCtx(w, r, &session{
		Data:    make(map[string]interface{}),
		Flashes: make(map[string]interface{}),
	})
}

// Flash sets a flash message on a request.
func (s *Session) Flash(w http.ResponseWriter, r *http.Request, key string, value interface{}) {
	data := s.fromReq(r)
	data.Flashes[key] = value
	s.saveCtx(w, r, data)
}

// Flashes returns all flash messages, clearing all saved flashes.
func (s *Session) Flashes(w http.ResponseWriter, r *http.Request) map[string]interface{} {
	data := s.fromReq(r)

	// Copy the map before clearing it from the session.
	values := make(map[string]interface{})
	for k, v := range data.Flashes {
		values[k] = v
	}

	data.Flashes = make(map[string]interface{})

	s.saveCtx(w, r, data)
	return values
}

type responseWrapper struct {
	b *bytes.Buffer       // Buffer to write to.
	c int                 // Storage for status code.
	w http.ResponseWriter // Underlying response writer.
}

func (rw *responseWrapper) Header() http.Header {
	return rw.w.Header()
}

func (rw *responseWrapper) Write(data []byte) (int, error) {
	return rw.b.Write(data)
}

func (rw *responseWrapper) WriteHeader(statusCode int) {
	rw.c = statusCode
}

func (rw *responseWrapper) Flush() (int64, error) {
	rw.w.WriteHeader(rw.c)
	return rw.b.WriteTo(rw.w)
}

// TemplMiddleware ensures that the session data is always available on the
// request context for any handler wrapped by the middleware.
//
// This is in contrast to the default behaviour of the library, which is to
// lazily extract the session data from the cookie into the request context
// whenever any session methods are called.
//
// This method makes it possible to access session data in templ's global ctx
// instance, for example, you can use the sessions.FlashesCtx method:
//
//	for key, val := range session.FlashesCtx(ctx) {
//		<div>{ key }: { fmt.Sprintf("%v", val) }</div>
//	}
func (s *Session) TemplMiddleware(next http.Handler) http.Handler {
	pool := &sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the session from the cookie, if it's present and valid.
		session := &session{}
		if cookie, err := r.Cookie(s.name); err != nil {
			session.init()
		} else {
			if err := s.sc.Decode(s.name, cookie.Value, session); err != nil {
				if !s.quiet {
					fmt.Printf("sessions: [ERROR] failed to decode session from cookie: %+v\n", err)
				}
				session.init()
			}
		}

		// Create a response wrapper instance to execute the handler with.
		b := pool.Get().(*bytes.Buffer)
		b.Reset()
		wrapper := &responseWrapper{
			b: b,
			c: 200,
			w: w,
		}

		// Set the session on the request's context so that it's accessible on
		// the handler.
		ctx := context.WithValue(r.Context(), sessionCtxKey, session)

		// Execute the handler.
		next.ServeHTTP(wrapper, r.WithContext(ctx))

		// Encode the updated session so that we can set it as a cookie.
		encoded, err := s.sc.Encode(s.name, session)
		if err != nil {
			if !s.quiet {
				fmt.Printf("sessions: [ERROR} failed to encode cookie: %+v\n", err)
			}
			return
		}

		http.SetCookie(wrapper, &http.Cookie{
			Name:     s.name,
			MaxAge:   defaultMaxAge,
			Expires:  time.Now().UTC().Add(time.Duration(defaultMaxAge * time.Second)),
			Value:    encoded,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
		})

		if _, err := wrapper.Flush(); err != nil {
			if !s.quiet {
				fmt.Printf("sessions: [ERROR] failed to write http response in call to sessions.TemplMiddleware: %v\n", err)
			}
		}

		pool.Put(b)
	})
}

// FlashesCtx returns all flash messages as a map[string]interace{} for the
// given context.
//
// This method is intended to be used with the https://github.com/a-h/templ
// library. It requires the use of the sessions.TemplMiddleware, which ensures
// that  every incoming request has the session data decoded into the context.
//
// When called, the flash messages are cleared on subsequent requests.
//
// This method makes it possible to access session data in templ's global ctx
// instance, for example, you can use the sessions.FlashesCtx method:
//
//	for key, val := range session.FlashesCtx(ctx) {
//		<div>{ key }: { fmt.Sprintf("%v", val) }</div>
//	}
func (s *Session) FlashesCtx(ctx context.Context) map[string]interface{} {
	flashes := make(map[string]interface{})
	v := ctx.Value(sessionCtxKey)

	if v != nil {
		ss, ok := v.(*session)
		if ok {
			for k, v := range ss.Flashes {
				flashes[k] = v
			}
			clear(ss.Flashes)
			return flashes
		}
	}

	if !s.quiet {
		fmt.Printf("sessions: [WARNING] FlashesCtx was called but the session is nil - did you remember to wrap your handler in sessions.TemplMiddleware?\n")
	}
	return flashes
}
