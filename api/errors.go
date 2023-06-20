package api

import (
	"fmt"
)

var (
	ErrInvalidRegisterAuthToken = fmt.Errorf("invalid register auth token")
	ErrInvalidRequestBodyData   = fmt.Errorf("invalid request body data")
	ErrCouldNotInsertToDatabase = fmt.Errorf("could not insert to database")
	ErrWrongLogin               = fmt.Errorf("wrong password or email")
	ErrInvalidHash              = fmt.Errorf("invalid hash")
	ErrImageNotFound            = fmt.Errorf("image not found")
	ErrInvalidJSON              = fmt.Errorf("invalid json body")
)
