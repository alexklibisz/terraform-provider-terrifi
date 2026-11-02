package unifi

import "fmt"

// NotFoundError is returned by Client read methods when the requested resource
// does not exist on the controller. Callers compare with errors.As / type
// assertion to translate into Terraform's "resource gone" handling.
type NotFoundError struct{}

func (e *NotFoundError) Error() string { return "not found" }

// HTTPError describes a failed controller request. Request payloads are omitted
// because they may contain credentials or wireless passphrases.
type HTTPError struct {
	StatusCode int
	Method     string
	URL        string
	Response   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("(%d) for %s %s\nresponse: %s", e.StatusCode, e.Method, e.URL, e.Response)
}
