package unifi

import (
	"context"
	"encoding/json"
	"net/http"
)

type restEnvelope[T any] struct {
	Meta json.RawMessage `json:"meta"`
	Data []T             `json:"data"`
}

func restList[T any](ctx context.Context, c *APIClient, method, url string, body any) ([]T, error) {
	var response restEnvelope[T]
	if err := c.doV1Request(ctx, method, url, body, &response); err != nil {
		return nil, err
	}
	if err := CheckV1Meta(response.Meta); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func restOne[T any](ctx context.Context, c *APIClient, method, url string, body any) (*T, error) {
	data, err := restList[T](ctx, c, method, url, body)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 && method == http.MethodPut {
		return restOne[T](ctx, c, http.MethodGet, url, nil)
	}
	if len(data) != 1 {
		return nil, &NotFoundError{}
	}
	return &data[0], nil
}

func restDelete(ctx context.Context, c *APIClient, url string) error {
	var response struct {
		Meta json.RawMessage `json:"meta"`
	}
	if err := c.doV1Request(ctx, http.MethodDelete, url, nil, &response); err != nil {
		return err
	}
	return CheckV1Meta(response.Meta)
}

// mergeRESTObject preserves controller fields outside the local write contract.
func (c *APIClient) mergeRESTObject(ctx context.Context, url string, managed any) (map[string]json.RawMessage, error) {
	current, err := restOne[map[string]json.RawMessage](ctx, c, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(managed)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return nil, err
	}
	for k, v := range fields {
		(*current)[k] = v
	}
	return *current, nil
}
