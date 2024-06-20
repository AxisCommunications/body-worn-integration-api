package server

import (
	"net/http"
	"os"
	"strconv"
	"strings"
)

// returnStatusFromEnv is a http middleware that overrides a request handler with a
// custom handler that returns the http status defined in `DEBUG_MSS_HTTP_STATUS` env var.
// Used for testing error handling.
func returnStatusFromEnv(handler http.Handler) http.Handler {

	statusOverrideForPrefix := statusOverrider()
	overrideHandler := statusOverrideForPrefix != nil

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !overrideHandler {
			handler.ServeHTTP(w, r)
			return
		}

		// Override status
		httpStatus, ok := statusOverrideForPrefix(r.URL.String())
		if !ok {
			handler.ServeHTTP(w, r)
			return
		}

		w.WriteHeader(httpStatus)
		w.Write([]byte(http.StatusText(httpStatus) + "\n"))
	})
}

func statusOverrider() func(url string) (int, bool) {
	val, ok := os.LookupEnv("DEBUG_MSS_HTTP_STATUS")
	if !ok {
		return nil
	}
	// Env var may contain either of "400", "/auth/:400", "/auth/:400,/status:503"
	// Eg. a "global" status override or a comma separated list of url to status override

	parts := strings.Split(val, ",")
	if len(parts) == 0 {
		return nil
	}

	httpStatus, parseErr := strconv.Atoi(val)
	if len(parts) == 1 && parseErr == nil {
		return func(s string) (int, bool) { return httpStatus, true }
	}

	overrides := map[string]int{}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		subparts := strings.Split(part, ":") // URL : STATUS
		if len(subparts) == 2 {
			httpStatus, parseErr := strconv.Atoi(subparts[1])
			if parseErr != nil {
				continue
			}
			overrides[subparts[0]] = httpStatus
		}
	}

	if len(overrides) == 0 {
		return nil
	}

	return func(url string) (int, bool) {
		matches := map[string]int{}
		for prefix, status := range overrides {
			if strings.HasPrefix(url, prefix) {
				matches[prefix] = status
			}
		}
		if len(matches) == 0 {
			return 0, false
		}

		n := 0
		status := 0
		for prefix, st := range matches {
			if len(prefix) > n {
				n = len(prefix)
				status = st
			}
		}

		return status, true
	}
}

// logRequestResponse is a http middleware that logs requests and the status returned to the client.
func logRequestResponse(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logWriter := &responseLogger{
			ResponseWriter: w,
			status:         200, // Default is 200
		}
		handler.ServeHTTP(logWriter, r)

		logger.Infof("%s %s %s%s : Status %d %s\n",
			r.RemoteAddr, r.Method, r.Host, r.URL.String(),
			logWriter.status, http.StatusText(logWriter.status),
		)
	})
}

// responseLogger is an implementation of http.ResponseWriter that logs returned Status code and content length
type responseLogger struct {
	http.ResponseWriter
	status int
	length int
}

func (w *responseLogger) WriteHeader(status int) {
	w.ResponseWriter.WriteHeader(status)
	w.status = status
}

func (w *responseLogger) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.length += n
	return n, err
}
