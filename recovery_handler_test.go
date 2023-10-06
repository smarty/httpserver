package httpserver

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestRecoveryHandlerFixture(t *testing.T) {
	gunit.Run(new(RecoveryHandlerFixture), t)
}

type RecoveryHandlerFixture struct {
	*gunit.Fixture

	handler http.Handler

	response *httptest.ResponseRecorder
	request  *http.Request

	serveHTTPCount    int
	serveHTTPResponse http.ResponseWriter
	serveHTTPRequest  *http.Request
	serveHTTPError    any

	panicRecoveredCount   int
	panicRecoveredRequest *http.Request
	panicRecoveredError   any

	logged []string
}

func (this *RecoveryHandlerFixture) Setup() {
	this.response = httptest.NewRecorder()
	this.request = httptest.NewRequest("GET", "/", nil)
	ignoredErrors := []error{context.Canceled, context.DeadlineExceeded, sql.ErrTxDone}
	this.handler = newRecoveryHandler(this, ignoredErrors, true, this, this)
}

func (this *RecoveryHandlerFixture) TestInnerHandlerCalled() {
	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.serveHTTPCount, should.Equal, 1)
	this.So(this.serveHTTPResponse, should.Equal, this.response)
	this.So(this.serveHTTPRequest, should.Equal, this.request)
}
func (this *RecoveryHandlerFixture) TestInnerHandlerDoesNotPanic_NotRecoveryNecessary() {
	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.response.Code, should.Equal, 200)
	this.So(this.panicRecoveredCount, should.Equal, 0)
}
func (this *RecoveryHandlerFixture) TestInnerHandlerPanic_ContextCancellation_ReturnHTTP500() {
	this.serveHTTPError = fmt.Errorf("inner: %w", context.Canceled)

	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.response.Code, should.Equal, 500)
	this.So(this.panicRecoveredCount, should.Equal, 0)
	this.So(this.panicRecoveredRequest, should.BeNil)
	this.So(this.logged, should.BeEmpty)
}
func (this *RecoveryHandlerFixture) TestInnerHandlerPanic_ContextDeadlineExceeded_ReturnHTTP500() {
	this.serveHTTPError = fmt.Errorf("inner: %w", context.DeadlineExceeded)

	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.response.Code, should.Equal, 500)
	this.So(this.panicRecoveredCount, should.Equal, 0)
	this.So(this.panicRecoveredRequest, should.BeNil)
	this.So(this.logged, should.BeEmpty)
}
func (this *RecoveryHandlerFixture) TestInnerHandlerPanic_SQL_TransactionDone_ReturnHTTP500() {
	this.serveHTTPError = fmt.Errorf("inner: %w", sql.ErrTxDone)

	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.response.Code, should.Equal, 500)
	this.So(this.panicRecoveredCount, should.Equal, 0)
	this.So(this.panicRecoveredRequest, should.BeNil)
	this.So(this.logged, should.BeEmpty)
}
func (this *RecoveryHandlerFixture) TestInnerHandlerPanicsWithError_ItShouldNotPanicButShouldNotifyMonitorAndReturnHTTP500() {
	this.serveHTTPError = errors.New("panic value")

	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.response.Code, should.Equal, 500)
	this.So(this.panicRecoveredCount, should.Equal, 1)
	this.So(this.panicRecoveredRequest, should.Equal, this.request)
	if this.So(this.logged, should.HaveLength, 1) {
		this.So(this.logged[0], should.StartWith, "[ERROR] Recovered panic: panic value")
	}
}
func (this *RecoveryHandlerFixture) TestInnerHandlerPanicWithNonError_ItShouldNotPanicButShouldNotifyMonitorAndReturnHTTP500() {
	this.serveHTTPError = "panic value"

	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.response.Code, should.Equal, 500)
	this.So(this.panicRecoveredCount, should.Equal, 1)
	this.So(this.panicRecoveredRequest, should.Equal, this.request)
	if this.So(this.logged, should.HaveLength, 1) {
		this.So(this.logged[0], should.StartWith, "[ERROR] Recovered panic: panic value")
	}
}
func (this *RecoveryHandlerFixture) TestInnerHandlerPanic_PostData_ReturnHTTP500() {
	body := bytes.NewReader([]byte("field1=value1&field2=value2\000"))
	this.request = httptest.NewRequest("POST", "/", body)

	this.serveHTTPError = "panic value"
	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.response.Code, should.Equal, 500)
	this.So(this.panicRecoveredCount, should.Equal, 1)
	this.So(this.panicRecoveredRequest, should.Equal, this.request)
	if this.So(this.logged, should.HaveLength, 1) {
		this.So(this.logged[0], should.StartWith, "[ERROR] Recovered panic: panic value")
		this.So(this.logged[0], should.EndWith, "=value2?")
	}
}

func (this *RecoveryHandlerFixture) TestInnerHandlerPanic_PostDataClosed_ReturnHTTP500() {
	this.request = httptest.NewRequest("HEAD", "/", dummyReader{})

	this.serveHTTPError = "panic value"
	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.response.Code, should.Equal, 500)
	this.So(this.panicRecoveredCount, should.Equal, 1)
	this.So(this.panicRecoveredRequest, should.Equal, this.request)
	if this.So(this.logged, should.HaveLength, 1) {
		this.So(this.logged[0], should.StartWith, "[ERROR] Recovered panic: panic value")
		this.So(this.logged[0], should.ContainSubstring, "closed pipe")
		this.So(this.logged[0], should.ContainSubstring, "HEAD /")
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (this *RecoveryHandlerFixture) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	this.serveHTTPCount++
	this.serveHTTPResponse = response
	this.serveHTTPRequest = request
	if this.serveHTTPError != nil {
		if request.Method == "POST" {
			_, _ = request.Body.Read(make([]byte, 10)) //simulate partial read
		} else if request.Method == "HEAD" {
			_, _ = io.ReadAll(request.Body)
			_ = request.Body.Close()
		}
		panic(this.serveHTTPError)
	}
}

func (this *RecoveryHandlerFixture) PanicRecovered(request *http.Request, err any) {
	this.panicRecoveredCount++
	this.panicRecoveredRequest = request
	this.panicRecoveredError = err
}

func (this *RecoveryHandlerFixture) Printf(format string, args ...any) {
	this.logged = append(this.logged, fmt.Sprintf(format, args...))
}

type dummyReader struct{}

func (d dummyReader) Close() error               { return io.ErrClosedPipe }
func (d dummyReader) Read(_ []byte) (int, error) { return 0, io.EOF }
