package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"runtime/debug"
)

type recoveryHandler struct {
	http.Handler
	monitor monitor
	logger  logger
}

func newRecoveryHandler(handler http.Handler, monitor monitor, logger logger) http.Handler {
	return recoveryHandler{
		Handler: handler,
		monitor: monitor,
		logger:  logger,
	}
}

func (this recoveryHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	defer this.finally(response, request)
	this.Handler.ServeHTTP(response, request)
}
func (this recoveryHandler) finally(response http.ResponseWriter, request *http.Request) {
	err := recover()
	if err == nil {
		return
	}

	this.logRecovery(err, request)
	this.internalServerError(response)
}

func (this recoveryHandler) logRecovery(recovered interface{}, request *http.Request) {
	if isIgnoredError(recovered) {
		return
	}

	this.monitor.PanicRecovered(request, recovered)
	this.logger.Printf("[ERROR] Recovered panic: %v\n%s", recovered, debug.Stack())
}
func (this recoveryHandler) internalServerError(response http.ResponseWriter) {
	http.Error(response, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
func isIgnoredError(recovered interface{}) bool {
	err, isErr := recovered.(error)
	if !isErr {
		return false
	}

	if errors.Is(err, context.Canceled) {
		return true
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	if errors.Is(err, sql.ErrTxDone) {
		return true
	}

	return false
}
