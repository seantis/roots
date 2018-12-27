package image

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type nullProvider struct{}

func (p *nullProvider) GetClient(url URL, auth string) (*http.Client, error) {
	return nil, fmt.Errorf("not implemented")
}

type falseProvider struct {
	*nullProvider
}

func (p *nullProvider) Supports(url URL) bool {
	return false
}

type trueProvider struct {
	*nullProvider
}

func (p *trueProvider) Supports(url URL) bool {
	return true
}

// TestRegistryLookup tests the registry lookup priority
func TestRegistryLookup(t *testing.T) {
	defer ClearProviderRegistry()

	foo := &falseProvider{}
	bar := &trueProvider{}
	baz := &falseProvider{}

	RegisterProvider("foo", foo)
	RegisterProvider("bar", bar)
	RegisterProvider("baz", baz)

	provider, _ := LookupProvider(URL{})

	assert.Equal(t, provider, bar, "provider registry lookup failure")
}
