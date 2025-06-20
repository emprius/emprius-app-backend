package test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/emprius/emprius-app-backend/notifications/mailtemplates"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/test/utils"
	qt "github.com/frankban/quicktest"
)

func TestLogin(t *testing.T) {
	c := utils.NewTestService(t)
	var resp []byte
	var code int

	_, code = c.Request(http.MethodPost, "",
		&api.Register{
			UserEmail:         "foo@test.com",
			RegisterAuthToken: utils.RegisterToken,
			UserProfile: api.UserProfile{
				Name:      "testuser",
				Community: "testCommunity",
				Password:  "testpassword",
			},
		},
		"register",
	)
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("Response: %s", string(resp)))

	// Check welcome mail is received
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mailBody, err := c.MailService().FindEmail(ctx, "foo@test.com")
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, mailBody, qt.Contains, mailtemplates.AppName)

	// try wrong auth token
	_, code = c.Request(http.MethodPost, "",
		&api.Register{
			UserEmail:         "foo2@test.com",
			RegisterAuthToken: utils.RegisterToken + "wrong",
			UserProfile: api.UserProfile{
				Name:      "testuser",
				Community: "testCommunity",
				Password:  "testpassword",
			},
		},
		"register",
	)
	qt.Assert(t, code, qt.Equals, 400)

	// try wrong login
	_, code = c.Request(http.MethodPost, "",
		&api.Login{
			Email:    "foo@test.com",
			Password: "testpasswordwrong",
		},
		"login",
	)
	qt.Assert(t, code, qt.Equals, 400)

	// try correct login
	resp, code = c.Request(http.MethodPost, "",
		&api.Login{
			Email:    "foo@test.com",
			Password: "testpassword",
		},
		"login",
	)
	qt.Assert(t, code, qt.Equals, 200)

	logResp := &api.LoginResponse{}
	err = json.Unmarshal(resp, logResp)
	qt.Assert(t, err, qt.IsNil)
}
