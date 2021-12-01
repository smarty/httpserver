package httpserver

import (
	"errors"
	"net/http"
	"runtime/debug"
)

type recoveryHandler struct {
	http.Handler
	ignoredErrors []error
	monitor       monitor
	logger        logger
}

func newRecoveryHandler(handler http.Handler, ignoredErrors []error, monitor monitor, logger logger) http.Handler {
	return &recoveryHandler{Handler: handler, ignoredErrors: ignoredErrors, monitor: monitor, logger: logger}
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

func (this *recoveryHandler) logRecovery(recovered interface{}, request *http.Request) {
	if this.isIgnoredError(recovered) {
		return
	}

	this.monitor.PanicRecovered(request, recovered)
	this.logger.Printf("[ERROR] Recovered panic: %v\n%s", recovered, debug.Stack())
}
func (this *recoveryHandler) isIgnoredError(recovered interface{}) bool {
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
