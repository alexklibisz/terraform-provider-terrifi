// Package unifi defines the API contracts and error types used by the
// Terrifi provider to talk to the UniFi controller.
package unifi

// NotFoundError is returned by Client read methods when the requested resource
// does not exist on the controller. Callers compare with errors.As / type
// assertion to translate into Terraform's "resource gone" handling.
type NotFoundError struct{}

func (e *NotFoundError) Error() string { return "not found" }
