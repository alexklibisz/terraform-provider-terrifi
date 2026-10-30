package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

var (
	firewallPolicyFreshNetworkAge = time.Minute
	firewallPolicySettleDelay     = 3 * time.Second
	firewallPolicyNow             = time.Now
)

// settleFreshNetworkPolicy verifies policies matching newly created networks.
// The controller can accept a policy and then asynchronously remove its network
// matches and disable it while processing the network changes. As with port
// profile tags, verify after the background work and repair the accepted write.
// Policies referencing only established networks do not wait or read again.
func (r *firewallPolicyResource) settleFreshNetworkPolicy(ctx context.Context, site string, sent *unifi.FirewallPolicy, sched *firewallPolicyScheduleRequest, written *firewallPolicyFull) (*firewallPolicyFull, error) {
	var networks []string
	if sent.Source != nil && sent.Source.MatchingTarget == "NETWORK" {
		networks = append(networks, sent.Source.IPs...)
	}
	if sent.Destination != nil && sent.Destination.MatchingTarget == "NETWORK" {
		networks = append(networks, sent.Destination.IPs...)
	}
	if !anyCreatedWithin(networks, firewallPolicyNow(), firewallPolicyFreshNetworkAge) {
		return written, nil
	}

	tflog.Debug(ctx, "Waiting to verify a firewall policy matching newly created networks",
		map[string]any{"policy_id": written.ID, "delay": firewallPolicySettleDelay.String()})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(firewallPolicySettleDelay):
	}

	got, err := r.client.getFirewallPolicy(ctx, site, written.ID, false)
	if err != nil {
		return nil, err
	}
	if networkPolicyMatches(sent, got.FirewallPolicy) {
		return got, nil
	}

	tflog.Info(ctx, "Controller changed a newly created network match, writing the firewall policy again",
		map[string]any{"policy_id": written.ID})
	repair := *sent
	repair.ID = written.ID
	if repair.Index == nil {
		repair.Index = got.Index
	}
	if _, err := r.client.UpdateFirewallPolicy(ctx, site, &repair, sched); err != nil {
		return nil, err
	}
	got, err = r.client.getFirewallPolicy(ctx, site, written.ID, false)
	if err != nil {
		return nil, err
	}
	if !networkPolicyMatches(sent, got.FirewallPolicy) {
		return nil, fmt.Errorf("controller did not retain network matches or enabled status for firewall policy %s after retry", written.ID)
	}
	return got, nil
}

// networkPolicyMatches checks the fields the controller's background work can
// change. Inactive IP/MAC/client filters are deliberately outside this check.
func networkPolicyMatches(sent, got *unifi.FirewallPolicy) bool {
	if sent.Enabled != got.Enabled {
		return false
	}
	if src := sent.Source; src != nil && src.MatchingTarget == "NETWORK" {
		if got.Source == nil || got.Source.MatchingTarget != "NETWORK" ||
			got.Source.ZoneID != src.ZoneID || got.Source.MatchOppositeIPs != src.MatchOppositeIPs ||
			!networkIDsMatch(src.IPs, got.Source.IPs) {
			return false
		}
	}
	if dst := sent.Destination; dst != nil && dst.MatchingTarget == "NETWORK" {
		if got.Destination == nil || got.Destination.MatchingTarget != "NETWORK" ||
			got.Destination.ZoneID != dst.ZoneID || got.Destination.MatchOppositeIPs != dst.MatchOppositeIPs ||
			!networkIDsMatch(dst.IPs, got.Destination.IPs) {
			return false
		}
	}
	return true
}
