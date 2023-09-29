package httpserver

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"runtime/debug"
	"strings"
	"unicode"
)

type recoveryHandler struct {
	http.Handler
	ignoredErrors  []error
	dumpRawRequest bool
	monitor        monitor
	logger         logger
}

func newRecoveryHandler(handler http.Handler, ignoredErrors []error, dumpRawRequest bool, monitor monitor, logger logger) http.Handler {
	return &recoveryHandler{Handler: handler, ignoredErrors: ignoredErrors, dumpRawRequest: dumpRawRequest, monitor: monitor, logger: logger}
}

func (this *recoveryHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	defer this.finally(response, request)
	this.Handler.ServeHTTP(response, request)
}

func (this *recoveryHandler) finally(response http.ResponseWriter, request *http.Request) {
	err := recover()
	if err == nil {
		return
	}

	this.logRecovery(err, request)
	this.internalServerError(response)
}

func (this *recoveryHandler) logRecovery(recovered any, request *http.Request) {
	if this.isIgnoredError(recovered) {
		return
	}

	this.monitor.PanicRecovered(request, recovered)
	this.logger.Printf("[ERROR] Recovered panic: %v\n%s%s", recovered, debug.Stack(), this.requestToString(request))
}

func (this *recoveryHandler) isIgnoredError(recovered any) bool {
	err, isErr := recovered.(error)
	if !isErr {
		return false
	}

	for _, ignored := range this.ignoredErrors {
		if errors.Is(err, ignored) {
			return true
		}
	}

	return false
}

func (this *recoveryHandler) internalServerError(response http.ResponseWriter) {
	http.Error(response, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (this *recoveryHandler) requestToString(request *http.Request) string {
	if !this.dumpRawRequest {
		return ""
	}

	raw, err := httputil.DumpRequest(request, true)
	formatted := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || unicode.IsPrint(r) {
			return r
		}
		return '?'
	}, strings.ReplaceAll(string(raw), "\r\n", "\n\t"))

	if err != nil {
		formatted += fmt.Sprintf(" [request formatting error: %s]", err)
	}

	return fmt.Sprint("Recovered request: ", formatted)
}
