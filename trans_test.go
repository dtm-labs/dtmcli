package dtmcli

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuery(t *testing.T) {
	qs, err := url.ParseQuery("a=b")
	assert.Nil(t, err)
	_, err = XaFromQuery(qs)
	assert.Error(t, err)
	_, err = TccFromQuery(qs)
	assert.Error(t, err)
	_, err = BarrierFromQuery(qs)
	assert.Error(t, err)
}

func TestXa(t *testing.T) {
	_, err := NewXaClient("http://localhost:8080", map[string]string{}, ":::::", nil)
	assert.Error(t, err)
}
