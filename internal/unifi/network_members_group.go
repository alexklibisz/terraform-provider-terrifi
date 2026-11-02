package unifi

// NetworkMembersGroup is the local controller API contract.
type NetworkMembersGroup struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
	Type    string   `json:"type"`
}
