package unifi

// NetworkMembersGroup mirrors entries at
// /v2/api/site/{site}/network-members-group[s]. The "client group" name in
// the resource layer maps onto Type="CLIENTS" entries here; Type="USERS"
// entries are legacy QoS user groups that the provider ignores.
type NetworkMembersGroup struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
	Type    string   `json:"type"`
}
