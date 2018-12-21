package image

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/seantis/roots/pkg/image"
	"github.com/stretchr/testify/assert"
)

type nullProvider struct{}

func (p *nullProvider) GetClient(url image.URL, auth string) (*http.Client, error) {
	return nil, fmt.Errorf("not implemented")
}

type falseProvider struct {
	*nullProvider
}

func (p *nullProvider) Supports(url image.URL) bool {
	return false
}

type trueProvider struct {
	*nullProvider
}

func (p *trueProvider) Supports(url image.URL) bool {
	return true
}

// TestRegistryLookup tests the registry lookup priority
func TestRegistryLookup(t *testing.T) {
	defer image.ClearProviderRegistry()

	foo := &falseProvider{}
	bar := &trueProvider{}
	baz := &falseProvider{}

	image.RegisterProvider("foo", foo)
	image.RegisterProvider("bar", bar)
	image.RegisterProvider("baz", baz)

	provider, _ := image.LookupProvider(image.URL{})

	assert.Equal(t, provider, bar, "provider registry lookup failure")
}
