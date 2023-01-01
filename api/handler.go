package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// RouterHandlerFn is the function signature for adding handlers to the HTTProuter.
type RouterHandlerFn = func(r *Request) (interface{}, error)

// Request represents an HTTP request to the API.
// It contains the request Body data, the URL path and the HTTP context.
// The context can be used for obtaining URL parameters and sending responses.
type Request struct {
	Data    []byte
	Path    []string
	Context *HTTPContext
	UserID  string
}

// HTTPContext is the Context for an HTTP request.
type HTTPContext struct {
	Writer  http.ResponseWriter
	Request *http.Request
}

// URLParam is a wrapper around go-chi to get a URL parameter (specified in the path pattern as {key})
func (h *HTTPContext) URLParam(key string) string {
	return chi.URLParam(h.Request, key)
}

// Send replies the request with the provided message.
func (h *HTTPContext) Send(msg []byte, httpStatusCode int) error {
	defer func() {
		if r := recover(); r != nil {
			log.Warn().Msgf("recovered http send panic: %v", r)
		}
	}()
	defer h.Request.Body.Close()

	if httpStatusCode < 100 || httpStatusCode >= 600 {
		return fmt.Errorf("http status code %d not supported", httpStatusCode)
	}
	if h.Request.Context().Err() != nil {
		// The connection was closed, so don't try to write to it.
		return fmt.Errorf("connection is closed")
	}
	h.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(msg)+1))
	h.Writer.Header().Set("Content-Type", "application/json")
	h.Writer.WriteHeader(httpStatusCode)

	if len(msg) > 0 {
		log.Debug().Msgf("response: %s", msg)
		if _, err := h.Writer.Write(msg); err != nil {
			return err
		}
	}
	// Ensure we end the response with a newline, to be nice.
	_, err := h.Writer.Write([]byte("\n"))
	return err
}

// routerHandler is a wrapper around the HTTP handler function to handle the request and response.
// It reads the request body, calls the handler function and sends the response.
// The errors are automatically logged and returned to the client.
func (a *API) routerHandler(handlerFunc RouterHandlerFn) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		hc := &HTTPContext{Request: req, Writer: w}
		body, err := io.ReadAll(req.Body)
		log.Debug().Msgf("request: %s", body)
		if err != nil {
			log.Warn().Err(err).Msg("failed to read request body")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := req.Body.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close request body")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		response, err := handlerFunc(
			&Request{
				Data:    body,
				Context: hc,
				Path:    strings.Split(req.URL.Path, "/")[1:],
				UserID:  req.Header.Get("X-User-ID"),
			})
		if err != nil {
			log.Warn().Err(err).Msg("failed to handle request")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var data []byte
		if response != nil {
			data, err = json.Marshal(response)
			if err != nil {
				log.Error().Err(err).Msg("failed to marshal response")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if err := hc.Send(data, http.StatusOK); err != nil {
			log.Error().Err(err).Msg("failed to send response")
		}
	}
}
