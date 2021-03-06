package net_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/cloudfoundry/cli/cf/configuration/coreconfig"
	"github.com/cloudfoundry/cli/cf/errors"
	. "github.com/cloudfoundry/cli/cf/net"
	"github.com/cloudfoundry/cli/cf/trace/tracefakes"
	testconfig "github.com/cloudfoundry/cli/testhelpers/configuration"
	testterm "github.com/cloudfoundry/cli/testhelpers/terminal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var failingRoutingAPIRequest = func(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(http.StatusBadRequest)
	jsonResponse := `{ "name": "some-error", "message": "The host is taken: test1" }`
	fmt.Fprintln(writer, jsonResponse)
}

var invalidTokenRoutingAPIRequest = func(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(http.StatusUnauthorized)
	jsonResponse := `{ "name": "UnauthorizedError", "message": "bad token!" }`
	fmt.Fprintln(writer, jsonResponse)
}

var _ = Describe("Routing Api Gateway", func() {
	var gateway Gateway
	var config coreconfig.Reader
	var fakeLogger *tracefakes.FakePrinter

	BeforeEach(func() {
		fakeLogger = new(tracefakes.FakePrinter)
		config = testconfig.NewRepository()
		gateway = NewRoutingAPIGateway(config, time.Now, &testterm.FakeUI{}, fakeLogger)
	})

	It("parses error responses", func() {
		ts := httptest.NewTLSServer(http.HandlerFunc(failingRoutingAPIRequest))
		defer ts.Close()
		gateway.SetTrustedCerts(ts.TLS.Certificates)

		request, apiErr := gateway.NewRequest("GET", ts.URL, "TOKEN", nil)
		_, apiErr = gateway.PerformRequest(request)

		Expect(apiErr).NotTo(BeNil())
		Expect(apiErr.Error()).To(ContainSubstring("The host is taken: test1"))
		Expect(apiErr.(errors.HTTPError).ErrorCode()).To(ContainSubstring("some-error"))
	})

	It("parses invalid token responses", func() {
		ts := httptest.NewTLSServer(http.HandlerFunc(invalidTokenRoutingAPIRequest))
		defer ts.Close()
		gateway.SetTrustedCerts(ts.TLS.Certificates)

		request, apiErr := gateway.NewRequest("GET", ts.URL, "TOKEN", nil)
		_, apiErr = gateway.PerformRequest(request)

		Expect(apiErr).NotTo(BeNil())
		Expect(apiErr.Error()).To(ContainSubstring("bad token"))
		Expect(apiErr.(errors.HTTPError)).To(HaveOccurred())
	})

	Context("when the Routing API returns a invalid JSON", func() {
		var invalidJSONResponse = func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusUnauthorized)
			jsonResponse := `¯\_(ツ)_/¯`
			fmt.Fprintln(writer, jsonResponse)
		}

		It("returns a 500 http error", func() {
			ts := httptest.NewTLSServer(http.HandlerFunc(invalidJSONResponse))
			defer ts.Close()
			gateway.SetTrustedCerts(ts.TLS.Certificates)

			request, apiErr := gateway.NewRequest("GET", ts.URL, "TOKEN", nil)
			_, apiErr = gateway.PerformRequest(request)

			Expect(apiErr).NotTo(BeNil())
			Expect(apiErr.(errors.HTTPError)).To(HaveOccurred())
			Expect(apiErr.(errors.HTTPError).StatusCode()).To(Equal(http.StatusInternalServerError))
		})
	})
})
