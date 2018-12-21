package provider

import "net/http"

type boundHeadersTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *boundHeadersTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.headers {
		req.Header.Add(k, v)
	}

	return t.base.RoundTrip(req)
}

// clientWithHeader returns an http.Client which sets the given headers on
// each request sent to the server
func clientWithHeaders(headers map[string]string) *http.Client {
	return &http.Client{
		Transport: &boundHeadersTransport{
			headers: headers,
			base:    http.DefaultTransport,
		},
	}
}
