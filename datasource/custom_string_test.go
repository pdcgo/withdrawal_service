package datasource_test

import (
	"testing"

	"github.com/pdcgo/withdrawal_service/datasource"
	"github.com/stretchr/testify/assert"
)

func TestCustomString(t *testing.T) {
	dd := datasource.OrderRefList{}

	dd.Add("")
	dd.Add("-")
	dd.Add("asdasd")

	assert.Len(t, dd, 1)
}
