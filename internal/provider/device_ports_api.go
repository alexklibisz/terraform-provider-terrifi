package provider

// Local methods for reading a device's ports and writing its port_overrides,
// using the local internal/unifi types rather than the go-unifi SDK — see
// issue #157.
//
// port_overrides is a whole-array field: a PUT replaces every entry. Writes
// are therefore a read-modify-write of the raw array that touches only the
// managed ports (see mergePortOverrides).

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/alexklibisz/terrifi/internal/unifi"
)

// ListDevicePorts reads the ports of every device on the site, or of the
// device with the given MAC.
func (c *Client) ListDevicePorts(ctx context.Context, site, mac string) ([]unifi.DevicePorts, error) {
	var resp struct {
		Meta json.RawMessage     `json:"meta"`
		Data []unifi.DevicePorts `json:"data"`
	}
	url := fmt.Sprintf("%s%s/api/s/%s/stat/device", c.BaseURL, c.APIPath, site)
	if mac != "" {
		url += "/" + strings.ToLower(mac)
	}
	if err := c.doRequest(ctx, http.MethodGet, url, nil, &resp, false); err != nil {
		if strings.Contains(err.Error(), "(404)") {
			return nil, &unifi.NotFoundError{}
		}
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetDevicePorts reads a device's ports by MAC.
func (c *Client) GetDevicePorts(ctx context.Context, site, mac string) (*unifi.DevicePorts, error) {
	devices, err := c.ListDevicePorts(ctx, site, mac)
	if err != nil {
		return nil, err
	}
	if len(devices) != 1 {
		return nil, &unifi.NotFoundError{}
	}
	return &devices[0], nil
}

// PutPortOverrides replaces a device's port_overrides array. Only that field
// is sent; the controller leaves the rest of the device as it is.
func (c *Client) PutPortOverrides(ctx context.Context, site, deviceID string, overrides []map[string]json.RawMessage) error {
	var resp struct {
		Meta json.RawMessage `json:"meta"`
	}
	url := fmt.Sprintf("%s%s/api/s/%s/rest/device/%s", c.BaseURL, c.APIPath, site, deviceID)
	body := map[string]any{"port_overrides": overrides}
	if err := c.doPortProfileRequest(ctx, http.MethodPut, url, body, &resp); err != nil {
		return err
	}
	return checkV1Meta(resp.Meta)
}

// portWant is the desired state of one managed port. nil fields are unset.
type portWant struct {
	Name            *string
	ProfileID       *string
	NativeNetworkID *string
	TaggedVLANMode  *string
	ResetVLAN       bool
}

// mergePortOverrides applies want to the raw port_overrides array and returns
// the new array. Ports not in want are left untouched. For each port in want:
//
//   - a nil *portWant removes the entire entry, restoring port defaults;
//   - a profile replaces the whole entry with {port_idx, name, portconf_id},
//     as the UI does: the profile then owns every setting, and a leftover
//     poe_mode would shadow the profile's;
//   - no profile removes portconf_id and keeps any inline settings;
//   - no name removes the name (the controller shows "Port N").
//
// An entry left with only port_idx is dropped: the port is back to default.
// portconf_id is never sent as "" (the controller rejects it).
func mergePortOverrides(current []map[string]json.RawMessage, want map[int64]*portWant) ([]map[string]json.RawMessage, error) {
	byIdx := map[int64]map[string]json.RawMessage{}
	out := []map[string]json.RawMessage{} // never null: the PUT must send []
	for _, e := range current {
		idx, err := portIdx(e)
		if err != nil {
			return nil, err
		}
		if _, managed := want[idx]; !managed {
			out = append(out, e)
			continue
		}
		byIdx[idx] = e
	}

	for idx, w := range want {
		if w == nil {
			continue
		} // Removing the override restores controller defaults.
		e := byIdx[idx]
		if e == nil {
			e = map[string]json.RawMessage{}
		}
		if w != nil && w.ProfileID != nil {
			e = map[string]json.RawMessage{"portconf_id": rawString(*w.ProfileID)}
		} else {
			delete(e, "portconf_id")
		}
		if w.ResetVLAN || w.NativeNetworkID != nil {
			for _, key := range []string{"native_networkconf_id", "tagged_vlan_mgmt", "excluded_networkconf_ids", "forward"} {
				delete(e, key)
			}
		}
		if w.NativeNetworkID != nil && w.TaggedVLANMode != nil {
			e["native_networkconf_id"] = rawString(*w.NativeNetworkID)
			mode, forward := "auto", "all"
			if *w.TaggedVLANMode == "block_all" {
				mode, forward = "block_all", "native"
			}
			e["tagged_vlan_mgmt"] = rawString(mode)
			e["forward"] = rawString(forward)
			e["excluded_networkconf_ids"] = json.RawMessage(`[]`)
			e["setting_preference"] = rawString("manual")
		}
		delete(e, "name")
		if w != nil && w.Name != nil {
			e["name"] = rawString(*w.Name)
		}
		e["port_idx"] = json.RawMessage(fmt.Sprint(idx))
		if len(e) > 1 {
			out = append(out, e)
		}
	}

	slices.SortFunc(out, func(a, b map[string]json.RawMessage) int {
		i, _ := portIdx(a)
		j, _ := portIdx(b)
		return cmp.Compare(i, j)
	})
	return out, nil
}

func portIdx(e map[string]json.RawMessage) (int64, error) {
	idx, _, _, err := unifi.ParsePortOverride(e)
	return idx, err
}

// portsFromOverrides reads name and portconf_id per port_idx.
func portsFromOverrides(overrides []map[string]json.RawMessage) map[int64]portWant {
	out := map[int64]portWant{}
	for _, e := range overrides {
		idx, name, profileID, err := unifi.ParsePortOverride(e)
		if err != nil {
			continue
		}
		var w portWant
		if name != "" {
			w.Name = &name
		}
		if profileID != "" {
			w.ProfileID = &profileID
		}
		native, mode := unifi.PortVLANSettings(e)
		if native != "" {
			w.NativeNetworkID = &native
		}
		if mode != "" {
			w.TaggedVLANMode = &mode
		}
		out[idx] = w
	}
	return out
}

// checkPortsAssignable returns an error for a port the device doesn't have,
// or a profile assigned to a gateway WAN port (becoming a WAN port strips the
// profile, so it would never stick).
func checkPortsAssignable(d *unifi.DevicePorts, want map[int64]*portWant) error {
	for idx, w := range want {
		if w == nil {
			continue
		}
		i := slices.IndexFunc(d.PortTable, func(p unifi.DevicePort) bool { return p.PortIdx == idx })
		if i < 0 {
			return fmt.Errorf("device %s has no port %d", d.MAC, idx)
		}
		if d.Model == "USMINI" && w.ProfileID != nil {
			return fmt.Errorf("device %s is a USW Flex Mini, which does not support custom port profiles; use inline VLAN settings", d.MAC)
		}
		if w.ProfileID == nil && w.NativeNetworkID == nil {
			continue
		}
		for _, eo := range d.EthernetOverrides {
			if eo.Ifname == d.PortTable[i].Ifname && eo.Networkgroup != "" && eo.Networkgroup != "LAN" {
				return fmt.Errorf("port %d on device %s is a %s port; port VLAN settings and profiles only apply to LAN ports", idx, d.MAC, eo.Networkgroup)
			}
		}
	}
	return nil
}

func rawString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

// validatePortNetworks checks references using local contracts. A Flex Mini
// cannot combine an alternate native network with an allow-all trunk.
func validatePortNetworks(d *unifi.DevicePorts, want map[int64]*portWant, networks []unifi.PortNetwork) error {
	for idx, w := range want {
		if w == nil || w.NativeNetworkID == nil {
			continue
		}
		i := slices.IndexFunc(networks, func(n unifi.PortNetwork) bool {
			return n.ID == *w.NativeNetworkID && (n.Purpose == "corporate" || n.Purpose == "guest" || n.Purpose == "vlan-only")
		})
		if i < 0 {
			return fmt.Errorf("port %d: native network %q is not a LAN network on this site", idx, *w.NativeNetworkID)
		}
		if d.Model == "USMINI" && w.TaggedVLANMode != nil && *w.TaggedVLANMode == "allow_all" && networks[i].VLANEnabled {
			return fmt.Errorf("port %d: USW Flex Mini supports allow_all only with the default native network; use block_all for a VLAN access port", idx)
		}
	}
	return nil
}

func (c *Client) listPortNetworks(ctx context.Context, site string) ([]unifi.PortNetwork, error) {
	var resp struct {
		Meta json.RawMessage     `json:"meta"`
		Data []unifi.PortNetwork `json:"data"`
	}
	url := fmt.Sprintf("%s%s/api/s/%s/rest/networkconf", c.BaseURL, c.APIPath, site)
	if err := c.doRequest(ctx, http.MethodGet, url, nil, &resp, false); err != nil {
		return nil, err
	}
	if err := checkV1Meta(resp.Meta); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
