package app

import (
	"io"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
)

func RunLocalHTTP(h *Handler) error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	h.logger.Infof("starting local HTTP server on http://localhost%s", addr)

	return http.ListenAndServe(
		addr,
		http.HandlerFunc(h.serveHTTP),
	)
}

func (h *Handler) serveHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Errorf("failed to read local request body: %v", err)
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}

	req := events.APIGatewayV2HTTPRequest{
		Body:    string(body),
		Headers: requestHeaders(r.Header),
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: r.Method,
			},
		},
	}

	resp, err := h.Handle(r.Context(), req)
	if err != nil {
		h.logger.Errorf("handler returned error in local HTTP mode: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	for key, value := range resp.Headers {
		w.Header().Set(key, value)
	}
	if resp.StatusCode != 0 {
		w.WriteHeader(resp.StatusCode)
	}
	_, _ = w.Write([]byte(resp.Body))
}

func requestHeaders(headers http.Header) map[string]string {
	values := make(map[string]string, len(headers))
	for key, items := range headers {
		if len(items) == 0 {
			continue
		}
		values[key] = items[0]
	}
	return values
}
