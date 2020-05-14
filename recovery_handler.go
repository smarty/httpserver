package httpserver

import (
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

	this.monitor.PanicRecovered(request, err)
	this.logger.Printf("[ERROR] Recovered panic: %v\n%s", err, debug.Stack())
	http.Error(response, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
