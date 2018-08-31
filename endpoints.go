package endpoints

import (
	"bytes"
	"encoding/json"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"

	applog "google.golang.org/appengine/log"
)

// contextKey is used to store values on a context.
type contextKey int

// Context value keys.
const (
	invalidKey contextKey = iota
	requestKey
	authenticatorKey
)

func logAlways(c context.Context, m string, v ...interface{}) {
	if c != nil {
		applog.Debugf(c, m, v...)
	}
}

func logAlwaysNoContext(r *http.Request, m string, v ...interface{}) {
	if r != nil {
		c := appengine.NewContext(r)
		if c != nil {
			applog.Debugf(c, m, v...)
		}
	}
}

type VoidMessage struct {
}

var typeOfVoidMessage = reflect.TypeOf(new(VoidMessage))

// HTTPRequest returns the request associated with a context.
func HTTPRequest(c context.Context) *http.Request {
	r, _ := c.Value(requestKey).(*http.Request)
	return r
}

// NewContext returns a new context for an in-flight API (HTTP) request.
func NewContext(r *http.Request) context.Context {
	c := appengine.NewContext(r)
	c = context.WithValue(c, requestKey, r)
	return c
}

type Service interface {
	EndpointPrefix() string
}

// EndpointHandlerWrapper takes an endpoint method and turns it into an http.HandlerFunc
// The reason for this is because endpoints doesn't support custom domains.
// I am wrapping the endpoints so we can leave them intact for when this is supported.
// The Panic in here is only at runtime.
func EndpointHandlerWrapper(service interface{}, name string) http.HandlerFunc {
	// using this helper because it gets the Request type and Return type for us
	reqType := typeOfVoidMessage
	method := reflect.ValueOf(service).MethodByName(name)
	if !method.IsValid() {
		log.Printf("method: %s\n", method)
		log.Panic("bad method")
	}
	numIn, numOut := method.Type().NumIn(), method.Type().NumOut()
	// Endpoint methods one to three arguments and
	// return either one or two values.
	if !(1 <= numIn && numIn <= 3 && 1 <= numOut && numOut <= 2) {
		return nil
	}
	// The response message is either an input or and output, not both.
	if numIn == 3 && numOut == 2 {
		return nil
	}
	// If there's a request type it's the second argument.
	if numIn >= 2 {
		reqType = method.Type().In(1).Elem()
	}
	// The last returned value is an error.
	// errType := method.Type().Out(method.Type().NumOut() - 1)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// read in the request
		body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			logAlwaysNoContext(r, "readall")
			return
		}
		if err := r.Body.Close(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			logAlwaysNoContext(r, "close")
			return
		}

		logAlwaysNoContext(r, "request body: %s", body)

		// Restore the body in the original request.
		r.Body = ioutil.NopCloser(bytes.NewReader(body))

		// get a new request Struct and unmarshal it
		val := reflect.New(reqType)
		if err := json.Unmarshal(body, val.Interface()); err != nil {
			w.WriteHeader(422) // unprocessable entity
			logAlwaysNoContext(r, "unmarshal")
			if err := json.NewEncoder(w).Encode(err); err != nil {
				logAlwaysNoContext(r, "encode err")
			}
			return
		}

		// call the Endpoint
		c := NewContext(r)
		args := []reflect.Value{reflect.ValueOf(c)}
		if numIn >= 2 {
			args = append(args, val)
		}
		ret := method.Call(args)

		var errValue, respValue reflect.Value

		// check for errors
		if numOut == 2 {
			respValue = ret[0]
			errValue = ret[1]
		} else {
			errValue = ret[0]
		}

		// Check if method returned an error
		if errr := errValue.Interface(); errr != nil {
			//logAlwaysNoContext(r, "errr")
			w.WriteHeader(http.StatusInternalServerError)
			if err := json.NewEncoder(w).Encode(errr); err != nil {
				logAlwaysNoContext(r, "encode errr")
			}
			return
		}

		// encode the response
		if err := json.NewEncoder(w).Encode(respValue.Interface()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			logAlwaysNoContext(r, "encode ret")
			return
		}
	}
}
