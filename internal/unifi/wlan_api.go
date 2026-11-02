package unifi

import (
	"context"
	"fmt"
	"net/http"
)

func (c *APIClient) ListWLAN(ctx context.Context, site string) ([]WLAN, error) {
	return restList[WLAN](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf", c.BaseURL, c.APIPath, site), nil)
}
func (c *APIClient) GetWLAN(ctx context.Context, site, id string) (*WLAN, error) {
	return restOne[WLAN](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf/%s", c.BaseURL, c.APIPath, site, id), nil)
}
func (c *APIClient) CreateWLAN(ctx context.Context, site string, d *WLAN) (*WLAN, error) {
	schedule := d.ScheduleWithDuration
	if schedule == nil {
		schedule = []WLANScheduleWithDuration{}
	}
	payload := struct {
		*WLAN
		Schedule []WLANScheduleWithDuration `json:"schedule_with_duration"`
	}{WLAN: d, Schedule: schedule}
	return restOne[WLAN](ctx, c, http.MethodPost, fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf", c.BaseURL, c.APIPath, site), payload)
}
func (c *APIClient) UpdateWLAN(ctx context.Context, site string, d *WLAN) (*WLAN, error) {
	url := fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf/%s", c.BaseURL, c.APIPath, site, d.ID)
	payload, err := c.mergeRESTObject(ctx, url, d)
	if err != nil {
		return nil, err
	}
	return restOne[WLAN](ctx, c, http.MethodPut, url, payload)
}
func (c *APIClient) DeleteWLAN(ctx context.Context, site, id string) error {
	return restDelete(ctx, c, fmt.Sprintf("%s%s/api/s/%s/rest/wlanconf/%s", c.BaseURL, c.APIPath, site, id))
}

type ControllerGroup struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

func (c *APIClient) ListWLANGroup(ctx context.Context, site string) ([]ControllerGroup, error) {
	return restList[ControllerGroup](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/wlangroup", c.BaseURL, c.APIPath, site), nil)
}
func (c *APIClient) ListClientGroup(ctx context.Context, site string) ([]ControllerGroup, error) {
	return restList[ControllerGroup](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/s/%s/rest/usergroup", c.BaseURL, c.APIPath, site), nil)
}
func (c *APIClient) ListAPGroup(ctx context.Context, site string) ([]ControllerGroup, error) {
	var data []ControllerGroup
	err := c.Do(ctx, http.MethodGet, fmt.Sprintf("%s%s/v2/api/site/%s/apgroups", c.BaseURL, c.APIPath, site), nil, &data)
	return data, err
}
func (c *APIClient) ListSites(ctx context.Context) ([]ControllerGroup, error) {
	return restList[ControllerGroup](ctx, c, http.MethodGet, fmt.Sprintf("%s%s/api/self/sites", c.BaseURL, c.APIPath), nil)
}
