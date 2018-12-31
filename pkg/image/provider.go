package image

import (
	"fmt"
	"net/http"
)

var (
	registry = make(map[string]Provider)
	priority = []string{}
)

// Provider provides an authenticated client for a given url.
type Provider interface {

	// GetClient returns an net/http Client that is authenticated
	// to interact with the repository for all urls the provider supports.
	//
	// It is called once for each new url. It is up to the provider to reuse
	// clients when called multiple times as this depends on the registry.
	//
	// The 'auth' parameter is an optional string used for authentication. Its
	// meaning is determined by the provider itself. It may be a path, a token
	// a username and password etc. - The cli passes the auth value as is.
	GetClient(url URL, auth string) (*http.Client, error)

	// Supports returns true if the provider supports the given url - multiple
	// providers may support the same url - in this case, the first provider
	// in order of registration is chosen
	Supports(url URL) bool
}

// LookupProvider takes an image.URL and returns the associated provider
func LookupProvider(url URL) (Provider, error) {
	for _, name := range priority {
		provider := registry[name]

		if provider.Supports(url) {
			return provider, nil
		}
	}

	return nil, fmt.Errorf("no provider for %s", url)
}

// RegisterProvider registers a provider with the given name. Providers are
// meant to be registered once during initialisation and doing so concurrently
// is not safe. If a provider with the same name exists, it is overwritten.
func RegisterProvider(name string, provider Provider) {
	registry[name] = provider
	priority = append(priority, name)
}

// ClearProviderRegistry clears the provider registry (mainly useful for tests)
func ClearProviderRegistry() {
	registry = make(map[string]Provider)
	priority = []string{}
}
