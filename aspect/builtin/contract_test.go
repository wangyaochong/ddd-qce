package builtin_test

import (
	"testing"

	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/aspect/builtin/builtintest"
)

func TestInMemoryMessageStore_Contract(t *testing.T) {
	builtintest.TestMessageStoreContract(t, builtin.NewInMemoryMessageStore())
}
