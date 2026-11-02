package provider

import (
	"testing"

	"github.com/alexklibisz/terrifi/internal/unifi"
	"github.com/hashicorp/go-retryablehttp"
)

func newTestClient(t *testing.T, serverURL string, enableCache bool) *Client {
	t.Helper()
	httpClient := retryablehttp.NewClient()
	httpClient.Logger = nil
	httpClient.RetryMax = 0
	api := &unifi.APIClient{BaseURL: serverURL, APIPath: "/proxy/network", HTTP: httpClient}
	if enableCache {
		api.EnableResponseCaching()
	}
	return &Client{APIClient: api}
}
