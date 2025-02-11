package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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

// GetPage returns the page number from the query parameters.
// If the page parameter is not present, it returns 0 (first page).
// If the page parameter is invalid or less than 0, it returns an error.
func (h *HTTPContext) GetPage() (int, error) {
	if pageParam := h.URLParam("page"); pageParam != nil {
		page, err := strconv.Atoi(pageParam[0])
		if err != nil {
			return 0, fmt.Errorf("invalid page number")
		}
		if page < 0 {
			return 0, fmt.Errorf("page number cannot be negative")
		}
		return page, nil
	}
	return 0, nil
}

// URLParam gets a URL parameter. For path parameters (specified in the path pattern as {key}),
// it uses chi.URLParam. For query parameters (?key=value and ?key[]=value), it uses URL.Query().
// If the key is not found, it returns nil. Else it returns a slice of values with at least one element.
// If the key is repeated in the query string, it will return all values.
func (h *HTTPContext) URLParam(key string) []string {
	// First try path parameter
	if param := chi.URLParam(h.Request, key); param != "" {
		return []string{param}
	}
	// Then try query parameter
	keys := h.Request.URL.Query()
	if k, ok := keys[key]; ok {
		return k
	}
	// Try query parameter with [] suffix
	if k, ok := keys[key+"[]"]; ok {
		return k
	}
	return nil
}

// Send replies the request with the provided message.
func (h *HTTPContext) Send(msg []byte, httpStatusCode int) error {
	defer func() {
		if r := recover(); r != nil {
			log.Warn().Msgf("recovered http send panic: %v", r)
		}
	}()

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
		var body []byte
		if req.Body != nil {
			var err error
			body, err = io.ReadAll(req.Body)
			if err != nil {
				log.Warn().Err(err).Msg("failed to read request body")
				resp := &Response{
					Header: ResponseHeader{
						Success: false,
						Message: err.Error(),
					},
				}
				msg, _ := json.Marshal(resp)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				if _, err := w.Write(msg); err != nil {
					log.Error().Err(err).Msg("failed to write response")
				}
				return
			}
			if err := req.Body.Close(); err != nil {
				log.Warn().Err(err).Msg("failed to close request body")
				resp := &Response{
					Header: ResponseHeader{
						Success: false,
						Message: err.Error(),
					},
				}
				msg, _ := json.Marshal(resp)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				if _, err := w.Write(msg); err != nil {
					log.Error().Err(err).Msg("failed to write response")
				}
				return
			}
			if len(body) > 0 {
				log.Debug().Msgf("request: %s", func() string {
					if len(body) > 1024 {
						return fmt.Sprintf("%s...", body[:1024])
					}
					return string(body)
				}())
			}
		}
		// Create request object with user ID from JWT
		request := &Request{
			Data:    body,
			Context: hc,
			Path:    strings.Split(req.URL.Path, "/")[1:],
			UserID:  req.Header.Get("X-User-ID"),
		}

		handlerResp, err := handlerFunc(request)
		resp := new(Response)
		if err != nil {
			log.Warn().Err(err).Msg("failed request")
			resp.Header.Success = false

			// Convert error to HTTPError if it isn't one already
			var httpErr *HTTPError
			if e, ok := err.(*HTTPError); ok {
				httpErr = e
			} else {
				httpErr = ErrInternalServerError.WithErr(err)
			}

			resp.Header.Message = httpErr.Error()
			msg, marshalErr := json.Marshal(resp)
			if marshalErr != nil {
				log.Error().Err(marshalErr).Msg("failed to marshal response")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				if _, err := w.Write([]byte(`{"header":{"success":false,"message":"internal server error"}}`)); err != nil {
					log.Error().Err(err).Msg("failed to write response")
				}
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(httpErr.Code)
			if _, err := w.Write(msg); err != nil {
				log.Error().Err(err).Msg("failed to write response")
			}
			return
		}
		// Check if response is binary
		if binaryResp, ok := handlerResp.(*BinaryResponse); ok {
			w.Header().Set("Content-Type", binaryResp.ContentType)
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write(binaryResp.Data); err != nil {
				log.Error().Err(err).Msg("failed to write binary response")
			}
			return
		}

		// Handle normal JSON response
		resp.Header.Success = true
		resp.Data = handlerResp
		data, err := json.Marshal(resp)
		if err != nil {
			log.Error().Err(err).Msg("failed to marshal response")
			resp := &Response{
				Header: ResponseHeader{
					Success: false,
					Message: err.Error(),
				},
			}
			msg, _ := json.Marshal(resp)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write(msg); err != nil {
				log.Error().Err(err).Msg("failed to write response")
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(data); err != nil {
			log.Error().Err(err).Msg("failed to write response")
		}
	}
}
