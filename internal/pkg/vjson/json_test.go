package vjson

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJson(t *testing.T) {
	t.Run("Test Marshal time.Time", func(t *testing.T) {
		output, err := Marshal(time.Unix(1, 1002))
		if err != nil {
			t.Fatal(err)
		}
		require.Equal(t, "\"1970-01-01T08:00:01.000001002+08:00\"", string(output))
	})
}
