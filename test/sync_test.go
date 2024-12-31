package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/test/utils"
	qt "github.com/frankban/quicktest"
)

func TestSync(t *testing.T) {
	c := utils.NewTestService(t)

	t.Run("Get Sync Data", func(t *testing.T) {
		// Get sync data
		resp, code := c.Request(http.MethodGet, "", nil, "info")
		qt.Assert(t, code, qt.Equals, 200)

		var syncResp struct {
			Data api.Info `json:"data"`
		}
		err := json.Unmarshal(resp, &syncResp)
		qt.Assert(t, err, qt.IsNil)

		// Verify sync data contains required information
		qt.Assert(t, syncResp.Data.Users, qt.Equals, 0)
		qt.Assert(t, syncResp.Data.Tools, qt.Equals, 0)
		qt.Assert(t, len(syncResp.Data.Categories), qt.Not(qt.Equals), 0)
		qt.Assert(t, len(syncResp.Data.Transports), qt.Not(qt.Equals), 0)

		// Create a user and verify user count increases
		c.RegisterAndLogin("test@test.com", "test", "testpass")

		resp, code = c.Request(http.MethodGet, "", nil, "info")
		qt.Assert(t, code, qt.Equals, 200)
		err = json.Unmarshal(resp, &syncResp)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, syncResp.Data.Users, qt.Equals, 1)
	})
}
