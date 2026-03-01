package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// randomMAC generates a random locally-administered MAC address (02:xx:xx:xx:xx:xx).
func randomMAC() string {
	return fmt.Sprintf("02:%02x:%02x:%02x:%02x:%02x",
		rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))
}

// randomVLAN returns a random VLAN ID in the range 100–3999 to avoid conflicts
// with existing networks or other test runs.
func randomVLAN() int {
	return 100 + rand.Intn(3900)
}

// ---------------------------------------------------------------------------
// Unit tests — no TF_ACC, no network, no env vars needed
// ---------------------------------------------------------------------------

func TestClientDeviceModelToAPI(t *testing.T) {
	r := &clientDeviceResource{}

	t.Run("mac only", func(t *testing.T) {
		model := &clientDeviceResourceModel{
			MAC: types.StringValue("AA:BB:CC:DD:EE:FF"),
		}

		c := r.modelToAPI(model)

		assert.Equal(t, "aa:bb:cc:dd:ee:ff", c.MAC, "MAC should be lowercased")
		assert.Empty(t, c.Name)
		assert.Empty(t, c.Note)
		assert.Empty(t, c.FixedIP)
		assert.False(t, c.UseFixedIP)
		assert.Empty(t, c.NetworkID)
		assert.Empty(t, c.LocalDNSRecord)
		assert.False(t, c.LocalDNSRecordEnabled)
		assert.Nil(t, c.VirtualNetworkOverrideEnabled)
		assert.Empty(t, c.VirtualNetworkOverrideID)
		assert.Empty(t, c.UserGroupID)
		assert.Nil(t, c.Blocked)
	})

	t.Run("all fields set", func(t *testing.T) {
		model := &clientDeviceResourceModel{
			MAC:               types.StringValue("aa:bb:cc:dd:ee:ff"),
			Name:              types.StringValue("My Device"),
			Note:              types.StringValue("A note"),
			FixedIP:           types.StringValue("192.168.1.100"),
			NetworkID:         types.StringValue("net-123"),
			NetworkOverrideID: types.StringValue("vlan-456"),
			LocalDNSRecord:    types.StringValue("mydevice.local"),
			ClientGroupID:     types.StringValue("group-789"),
			Blocked:           types.BoolValue(true),
		}

		c := r.modelToAPI(model)

		assert.Equal(t, "aa:bb:cc:dd:ee:ff", c.MAC)
		assert.Equal(t, "My Device", c.Name)
		assert.Equal(t, "A note", c.Note)
		assert.Equal(t, "192.168.1.100", c.FixedIP)
		assert.True(t, c.UseFixedIP)
		assert.Equal(t, "net-123", c.NetworkID)
		assert.Equal(t, "vlan-456", c.VirtualNetworkOverrideID)
		assert.NotNil(t, c.VirtualNetworkOverrideEnabled)
		assert.True(t, *c.VirtualNetworkOverrideEnabled)
		assert.Equal(t, "mydevice.local", c.LocalDNSRecord)
		assert.True(t, c.LocalDNSRecordEnabled)
		assert.Equal(t, "group-789", c.UserGroupID)
		assert.NotNil(t, c.Blocked)
		assert.True(t, *c.Blocked)
	})

	t.Run("fixed_ip sets use_fixedip and network_id", func(t *testing.T) {
		model := &clientDeviceResourceModel{
			MAC:       types.StringValue("aa:bb:cc:dd:ee:ff"),
			FixedIP:   types.StringValue("10.0.0.50"),
			NetworkID: types.StringValue("net-abc"),
		}

		c := r.modelToAPI(model)

		assert.Equal(t, "10.0.0.50", c.FixedIP)
		assert.True(t, c.UseFixedIP)
		assert.Equal(t, "net-abc", c.NetworkID)
	})

	t.Run("fixed_ip with network_override_id but no network_id", func(t *testing.T) {
		model := &clientDeviceResourceModel{
			MAC:               types.StringValue("aa:bb:cc:dd:ee:ff"),
			FixedIP:           types.StringValue("10.0.0.50"),
			NetworkOverrideID: types.StringValue("override-123"),
		}

		c := r.modelToAPI(model)

		assert.Equal(t, "10.0.0.50", c.FixedIP)
		assert.True(t, c.UseFixedIP)
		assert.Empty(t, c.NetworkID, "NetworkID should be empty when only network_override_id is set")
		assert.Equal(t, "override-123", c.VirtualNetworkOverrideID)
		assert.NotNil(t, c.VirtualNetworkOverrideEnabled)
		assert.True(t, *c.VirtualNetworkOverrideEnabled)
	})

	t.Run("local_dns_record sets local_dns_record_enabled", func(t *testing.T) {
		model := &clientDeviceResourceModel{
			MAC:            types.StringValue("aa:bb:cc:dd:ee:ff"),
			LocalDNSRecord: types.StringValue("host.local"),
		}

		c := r.modelToAPI(model)

		assert.Equal(t, "host.local", c.LocalDNSRecord)
		assert.True(t, c.LocalDNSRecordEnabled)
	})

	t.Run("network_override_id sets virtual_network_override_enabled", func(t *testing.T) {
		model := &clientDeviceResourceModel{
			MAC:               types.StringValue("aa:bb:cc:dd:ee:ff"),
			NetworkOverrideID: types.StringValue("override-789"),
		}

		c := r.modelToAPI(model)

		assert.Equal(t, "override-789", c.VirtualNetworkOverrideID)
		assert.NotNil(t, c.VirtualNetworkOverrideEnabled)
		assert.True(t, *c.VirtualNetworkOverrideEnabled)
	})

	t.Run("blocked true", func(t *testing.T) {
		model := &clientDeviceResourceModel{
			MAC:     types.StringValue("aa:bb:cc:dd:ee:ff"),
			Blocked: types.BoolValue(true),
		}

		c := r.modelToAPI(model)

		assert.NotNil(t, c.Blocked)
		assert.True(t, *c.Blocked)
	})

	t.Run("blocked false", func(t *testing.T) {
		model := &clientDeviceResourceModel{
			MAC:     types.StringValue("aa:bb:cc:dd:ee:ff"),
			Blocked: types.BoolValue(false),
		}

		c := r.modelToAPI(model)

		assert.NotNil(t, c.Blocked)
		assert.False(t, *c.Blocked)
	})

	t.Run("uppercase MAC normalized", func(t *testing.T) {
		model := &clientDeviceResourceModel{
			MAC: types.StringValue("AA:BB:CC:DD:EE:FF"),
		}

		c := r.modelToAPI(model)

		assert.Equal(t, "aa:bb:cc:dd:ee:ff", c.MAC)
	})
}

func TestClientDeviceAPIToModel(t *testing.T) {
	r := &clientDeviceResource{}

	t.Run("minimal client", func(t *testing.T) {
		c := &unifi.Client{
			ID:  "client-123",
			MAC: "aa:bb:cc:dd:ee:ff",
		}

		var model clientDeviceResourceModel
		r.apiToModel(c, &model, "default")

		assert.Equal(t, "client-123", model.ID.ValueString())
		assert.Equal(t, "default", model.Site.ValueString())
		assert.Equal(t, "aa:bb:cc:dd:ee:ff", model.MAC.ValueString())
		assert.True(t, model.Name.IsNull(), "Name should be null")
		assert.True(t, model.Note.IsNull(), "Note should be null")
		assert.True(t, model.FixedIP.IsNull(), "FixedIP should be null")
		assert.True(t, model.NetworkID.IsNull(), "NetworkID should be null")
		assert.True(t, model.NetworkOverrideID.IsNull(), "NetworkOverrideID should be null")
		assert.True(t, model.LocalDNSRecord.IsNull(), "LocalDNSRecord should be null")
		assert.True(t, model.ClientGroupID.IsNull(), "ClientGroupID should be null")
		assert.True(t, model.Blocked.IsNull(), "Blocked should be null")
	})

	t.Run("full client", func(t *testing.T) {
		blocked := true
		overrideEnabled := true
		c := &unifi.Client{
			ID:                            "client-456",
			MAC:                           "11:22:33:44:55:66",
			Name:                          "My Device",
			Note:                          "Some note",
			FixedIP:                       "192.168.1.50",
			UseFixedIP:                    true,
			NetworkID:                     "net-789",
			VirtualNetworkOverrideEnabled: &overrideEnabled,
			VirtualNetworkOverrideID:      "vlan-abc",
			LocalDNSRecord:                "mydevice.local",
			LocalDNSRecordEnabled:         true,
			UserGroupID:                   "group-xyz",
			Blocked:                       &blocked,
		}

		var model clientDeviceResourceModel
		r.apiToModel(c, &model, "mysite")

		assert.Equal(t, "client-456", model.ID.ValueString())
		assert.Equal(t, "mysite", model.Site.ValueString())
		assert.Equal(t, "11:22:33:44:55:66", model.MAC.ValueString())
		assert.Equal(t, "My Device", model.Name.ValueString())
		assert.Equal(t, "Some note", model.Note.ValueString())
		assert.Equal(t, "192.168.1.50", model.FixedIP.ValueString())
		assert.Equal(t, "net-789", model.NetworkID.ValueString())
		assert.Equal(t, "vlan-abc", model.NetworkOverrideID.ValueString())
		assert.Equal(t, "mydevice.local", model.LocalDNSRecord.ValueString())
		assert.Equal(t, "group-xyz", model.ClientGroupID.ValueString())
		assert.True(t, model.Blocked.ValueBool())
	})

	t.Run("use_fixedip false with stale fixed_ip", func(t *testing.T) {
		c := &unifi.Client{
			ID:         "client-789",
			MAC:        "aa:bb:cc:dd:ee:ff",
			FixedIP:    "192.168.1.99",
			UseFixedIP: false,
			NetworkID:  "net-old",
		}

		var model clientDeviceResourceModel
		r.apiToModel(c, &model, "default")

		assert.True(t, model.FixedIP.IsNull(), "FixedIP should be null when use_fixedip is false")
		assert.True(t, model.NetworkID.IsNull(), "NetworkID should be null when use_fixedip is false")
	})

	t.Run("local_dns_record_enabled false with stale record", func(t *testing.T) {
		c := &unifi.Client{
			ID:                    "client-aaa",
			MAC:                   "aa:bb:cc:dd:ee:ff",
			LocalDNSRecord:        "stale.local",
			LocalDNSRecordEnabled: false,
		}

		var model clientDeviceResourceModel
		r.apiToModel(c, &model, "default")

		assert.True(t, model.LocalDNSRecord.IsNull(), "LocalDNSRecord should be null when disabled")
	})

	t.Run("blocked nil", func(t *testing.T) {
		c := &unifi.Client{
			ID:      "client-bbb",
			MAC:     "aa:bb:cc:dd:ee:ff",
			Blocked: nil,
		}

		var model clientDeviceResourceModel
		r.apiToModel(c, &model, "default")

		assert.True(t, model.Blocked.IsNull(), "Blocked should be null when nil")
	})

	t.Run("blocked false", func(t *testing.T) {
		blocked := false
		c := &unifi.Client{
			ID:      "client-ccc",
			MAC:     "aa:bb:cc:dd:ee:ff",
			Blocked: &blocked,
		}

		var model clientDeviceResourceModel
		r.apiToModel(c, &model, "default")

		assert.False(t, model.Blocked.IsNull(), "Blocked should not be null when explicitly false")
		assert.False(t, model.Blocked.ValueBool(), "Blocked should be false")
	})
}

// ---------------------------------------------------------------------------
// Acceptance tests — require TF_ACC=1 and a UniFi controller
// ---------------------------------------------------------------------------

func TestAccClientDevice_basic(t *testing.T) {
	mac := randomMAC()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac  = %q
  name = "tfacc-basic"
}
`, mac),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "mac", mac),
					resource.TestCheckResourceAttr("terrifi_client_device.test", "name", "tfacc-basic"),
					resource.TestCheckResourceAttr("terrifi_client_device.test", "site", "default"),
					resource.TestCheckResourceAttrSet("terrifi_client_device.test", "id"),
				),
			},
		},
	})
}

func TestAccClientDevice_note(t *testing.T) {
	mac := randomMAC()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac  = %q
  name = "tfacc-note"
  note = "This is a test note"
}
`, mac),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "name", "tfacc-note"),
					resource.TestCheckResourceAttr("terrifi_client_device.test", "note", "This is a test note"),
				),
			},
		},
	})
}

func TestAccClientDevice_fixedIP(t *testing.T) {
	mac := randomMAC()
	netName := fmt.Sprintf("tfacc-fixip-%s", randomSuffix())
	vlan := randomVLAN()
	third := vlan % 256
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name         = %q
  purpose      = "corporate"
  vlan_id      = %d
  subnet       = "10.%d.0.1/24"
  dhcp_enabled = true
  dhcp_start   = "10.%d.0.6"
  dhcp_stop    = "10.%d.0.254"
}

resource "terrifi_client_device" "test" {
  mac        = %q
  name       = "tfacc-fixedip"
  fixed_ip   = "10.%d.0.100"
  network_id = terrifi_network.test.id
}
`, netName, vlan, third, third, third, mac, third),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "fixed_ip", fmt.Sprintf("10.%d.0.100", third)),
					resource.TestCheckResourceAttrSet("terrifi_client_device.test", "network_id"),
				),
			},
		},
	})
}

func TestAccClientDevice_localDNSRecord(t *testing.T) {
	mac := randomMAC()
	netName := fmt.Sprintf("tfacc-dns-%s", randomSuffix())
	dnsName := fmt.Sprintf("tfacc-dns-%s.local", randomSuffix())
	vlan := randomVLAN()
	third := vlan % 256
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name         = %q
  purpose      = "corporate"
  vlan_id      = %d
  subnet       = "10.%d.4.1/24"
  dhcp_enabled = true
  dhcp_start   = "10.%d.4.6"
  dhcp_stop    = "10.%d.4.254"
}

resource "terrifi_client_device" "test" {
  mac              = %q
  name             = "tfacc-dns"
  fixed_ip         = "10.%d.4.100"
  network_id       = terrifi_network.test.id
  local_dns_record = %q
}
`, netName, vlan, third, third, third, mac, third, dnsName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "local_dns_record", dnsName),
					resource.TestCheckResourceAttr("terrifi_client_device.test", "fixed_ip", fmt.Sprintf("10.%d.4.100", third)),
				),
			},
		},
	})
}

func TestAccClientDevice_networkOverride(t *testing.T) {
	mac := randomMAC()
	netName := fmt.Sprintf("tfacc-override-%s", randomSuffix())
	vlan := randomVLAN()
	third := vlan % 256
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "override" {
  name         = %q
  purpose      = "corporate"
  vlan_id      = %d
  subnet       = "10.%d.1.1/24"
  dhcp_enabled = true
  dhcp_start   = "10.%d.1.6"
  dhcp_stop    = "10.%d.1.254"
}

resource "terrifi_client_device" "test" {
  mac                 = %q
  name                = "tfacc-override"
  network_override_id = terrifi_network.override.id
}
`, netName, vlan, third, third, third, mac),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("terrifi_client_device.test", "network_override_id"),
				),
			},
		},
	})
}

func TestAccClientDevice_fixedIPWithNetworkOverride(t *testing.T) {
	mac := randomMAC()
	netName := fmt.Sprintf("tfacc-fipovr-%s", randomSuffix())
	vlan := randomVLAN()
	third := vlan % 256
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name         = %q
  purpose      = "corporate"
  vlan_id      = %d
  subnet       = "10.%d.0.1/24"
  dhcp_enabled = true
  dhcp_start   = "10.%d.0.6"
  dhcp_stop    = "10.%d.0.254"
}

resource "terrifi_client_device" "test" {
  mac                 = %q
  name                = "tfacc-fixedip-override"
  fixed_ip            = "10.%d.0.100"
  network_override_id = terrifi_network.test.id
}
`, netName, vlan, third, third, third, mac, third),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "fixed_ip", fmt.Sprintf("10.%d.0.100", third)),
					resource.TestCheckResourceAttrSet("terrifi_client_device.test", "network_override_id"),
					resource.TestCheckNoResourceAttr("terrifi_client_device.test", "network_id"),
				),
			},
		},
	})
}

func TestAccClientDevice_fixedIPWithBothNetworks(t *testing.T) {
	mac := randomMAC()
	netName := fmt.Sprintf("tfacc-fipboth-%s", randomSuffix())
	overrideName := fmt.Sprintf("tfacc-fipbothovr-%s", randomSuffix())
	vlan1 := randomVLAN()
	vlan2 := vlan1 + 1
	third1 := vlan1 % 256
	third2 := vlan2 % 256
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "fixedip_net" {
  name         = %q
  purpose      = "corporate"
  vlan_id      = %d
  subnet       = "10.%d.0.1/24"
  dhcp_enabled = true
  dhcp_start   = "10.%d.0.6"
  dhcp_stop    = "10.%d.0.254"
}

resource "terrifi_network" "override_net" {
  name         = %q
  purpose      = "corporate"
  vlan_id      = %d
  subnet       = "10.%d.0.1/24"
  dhcp_enabled = true
  dhcp_start   = "10.%d.0.6"
  dhcp_stop    = "10.%d.0.254"
}

resource "terrifi_client_device" "test" {
  mac                 = %q
  name                = "tfacc-fixedip-both"
  fixed_ip            = "10.%d.0.100"
  network_id          = terrifi_network.fixedip_net.id
  network_override_id = terrifi_network.override_net.id
}
`, netName, vlan1, third1, third1, third1, overrideName, vlan2, third2, third2, third2, mac, third1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "fixed_ip", fmt.Sprintf("10.%d.0.100", third1)),
					resource.TestCheckResourceAttrSet("terrifi_client_device.test", "network_id"),
					resource.TestCheckResourceAttrSet("terrifi_client_device.test", "network_override_id"),
				),
			},
		},
	})
}

func TestAccClientDevice_updateFixedIPNetworkToOverride(t *testing.T) {
	mac := randomMAC()
	netName := fmt.Sprintf("tfacc-fipswitch-%s", randomSuffix())
	vlan := randomVLAN()
	third := vlan % 256
	netConfig := fmt.Sprintf(`
resource "terrifi_network" "test" {
  name         = %q
  purpose      = "corporate"
  vlan_id      = %d
  subnet       = "10.%d.0.1/24"
  dhcp_enabled = true
  dhcp_start   = "10.%d.0.6"
  dhcp_stop    = "10.%d.0.254"
}
`, netName, vlan, third, third, third)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: fixed_ip + network_id
			{
				Config: netConfig + fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac        = %q
  name       = "tfacc-fipswitch"
  fixed_ip   = "10.%d.0.100"
  network_id = terrifi_network.test.id
}
`, mac, third),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "fixed_ip", fmt.Sprintf("10.%d.0.100", third)),
					resource.TestCheckResourceAttrSet("terrifi_client_device.test", "network_id"),
					resource.TestCheckNoResourceAttr("terrifi_client_device.test", "network_override_id"),
				),
			},
			// Step 2: switch to fixed_ip + network_override_id (drop network_id)
			{
				Config: netConfig + fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac                 = %q
  name                = "tfacc-fipswitch"
  fixed_ip            = "10.%d.0.100"
  network_override_id = terrifi_network.test.id
}
`, mac, third),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "fixed_ip", fmt.Sprintf("10.%d.0.100", third)),
					resource.TestCheckResourceAttrSet("terrifi_client_device.test", "network_override_id"),
					resource.TestCheckNoResourceAttr("terrifi_client_device.test", "network_id"),
				),
			},
			// Step 3: switch back to fixed_ip + network_id
			{
				Config: netConfig + fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac        = %q
  name       = "tfacc-fipswitch"
  fixed_ip   = "10.%d.0.100"
  network_id = terrifi_network.test.id
}
`, mac, third),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "fixed_ip", fmt.Sprintf("10.%d.0.100", third)),
					resource.TestCheckResourceAttrSet("terrifi_client_device.test", "network_id"),
					resource.TestCheckNoResourceAttr("terrifi_client_device.test", "network_override_id"),
				),
			},
		},
	})
}

func TestAccClientDevice_blocked(t *testing.T) {
	mac := randomMAC()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac     = %q
  name    = "tfacc-blocked"
  blocked = true
}
`, mac),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "blocked", "true"),
				),
			},
		},
	})
}

func TestAccClientDevice_allFields(t *testing.T) {
	mac := randomMAC()
	netName := fmt.Sprintf("tfacc-all-%s", randomSuffix())
	dnsName := fmt.Sprintf("tfacc-all-%s.local", randomSuffix())
	vlan := randomVLAN()
	third := vlan % 256
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name         = %q
  purpose      = "corporate"
  vlan_id      = %d
  subnet       = "10.%d.2.1/24"
  dhcp_enabled = true
  dhcp_start   = "10.%d.2.6"
  dhcp_stop    = "10.%d.2.254"
}

resource "terrifi_client_device" "test" {
  mac              = %q
  name             = "tfacc-all"
  note             = "Full test"
  fixed_ip         = "10.%d.2.42"
  network_id       = terrifi_network.test.id
  local_dns_record = %q
  blocked          = true
}
`, netName, vlan, third, third, third, mac, third, dnsName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "mac", mac),
					resource.TestCheckResourceAttr("terrifi_client_device.test", "name", "tfacc-all"),
					resource.TestCheckResourceAttr("terrifi_client_device.test", "note", "Full test"),
					resource.TestCheckResourceAttr("terrifi_client_device.test", "fixed_ip", fmt.Sprintf("10.%d.2.42", third)),
					resource.TestCheckResourceAttrSet("terrifi_client_device.test", "network_id"),
					resource.TestCheckResourceAttr("terrifi_client_device.test", "local_dns_record", dnsName),
					resource.TestCheckResourceAttr("terrifi_client_device.test", "blocked", "true"),
					resource.TestCheckResourceAttrSet("terrifi_client_device.test", "id"),
					resource.TestCheckResourceAttr("terrifi_client_device.test", "site", "default"),
				),
			},
		},
	})
}

func TestAccClientDevice_updateName(t *testing.T) {
	mac := randomMAC()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac  = %q
  name = "tfacc-name-v1"
}
`, mac),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "name", "tfacc-name-v1"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac  = %q
  name = "tfacc-name-v2"
}
`, mac),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "name", "tfacc-name-v2"),
				),
			},
		},
	})
}

func TestAccClientDevice_updateAddRemoveFixedIP(t *testing.T) {
	mac := randomMAC()
	netName := fmt.Sprintf("tfacc-arfip-%s", randomSuffix())
	vlan := randomVLAN()
	third := vlan % 256
	netConfig := fmt.Sprintf(`
resource "terrifi_network" "test" {
  name         = %q
  purpose      = "corporate"
  vlan_id      = %d
  subnet       = "10.%d.3.1/24"
  dhcp_enabled = true
  dhcp_start   = "10.%d.3.6"
  dhcp_stop    = "10.%d.3.254"
}
`, netName, vlan, third, third, third)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: no fixed IP
			{
				Config: netConfig + fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac  = %q
  name = "tfacc-fixip-toggle"
}
`, mac),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("terrifi_client_device.test", "fixed_ip"),
				),
			},
			// Step 2: add fixed IP
			{
				Config: netConfig + fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac        = %q
  name       = "tfacc-fixip-toggle"
  fixed_ip   = "10.%d.3.50"
  network_id = terrifi_network.test.id
}
`, mac, third),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "fixed_ip", fmt.Sprintf("10.%d.3.50", third)),
					resource.TestCheckResourceAttrSet("terrifi_client_device.test", "network_id"),
				),
			},
			// Step 3: remove fixed IP
			{
				Config: netConfig + fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac  = %q
  name = "tfacc-fixip-toggle"
}
`, mac),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("terrifi_client_device.test", "fixed_ip"),
					resource.TestCheckNoResourceAttr("terrifi_client_device.test", "network_id"),
				),
			},
		},
	})
}

func TestAccClientDevice_updateAddRemoveLocalDNS(t *testing.T) {
	mac := randomMAC()
	netName := fmt.Sprintf("tfacc-ardns-%s", randomSuffix())
	dnsName := fmt.Sprintf("tfacc-toggle-%s.local", randomSuffix())
	vlan := randomVLAN()
	third := vlan % 256
	netConfig := fmt.Sprintf(`
resource "terrifi_network" "test" {
  name         = %q
  purpose      = "corporate"
  vlan_id      = %d
  subnet       = "10.%d.5.1/24"
  dhcp_enabled = true
  dhcp_start   = "10.%d.5.6"
  dhcp_stop    = "10.%d.5.254"
}
`, netName, vlan, third, third, third)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: fixed IP only, no DNS record
			{
				Config: netConfig + fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac        = %q
  name       = "tfacc-dns-toggle"
  fixed_ip   = "10.%d.5.50"
  network_id = terrifi_network.test.id
}
`, mac, third),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("terrifi_client_device.test", "local_dns_record"),
				),
			},
			// Step 2: add DNS record (fixed IP still present — required by controller)
			{
				Config: netConfig + fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac              = %q
  name             = "tfacc-dns-toggle"
  fixed_ip         = "10.%d.5.50"
  network_id       = terrifi_network.test.id
  local_dns_record = %q
}
`, mac, third, dnsName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "local_dns_record", dnsName),
				),
			},
			// Step 3: remove DNS record (keep fixed IP)
			{
				Config: netConfig + fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac        = %q
  name       = "tfacc-dns-toggle"
  fixed_ip   = "10.%d.5.50"
  network_id = terrifi_network.test.id
}
`, mac, third),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("terrifi_client_device.test", "local_dns_record"),
				),
			},
		},
	})
}

func TestAccClientDevice_updateBlockUnblock(t *testing.T) {
	mac := randomMAC()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: blocked
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac     = %q
  name    = "tfacc-block-toggle"
  blocked = true
}
`, mac),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "blocked", "true"),
				),
			},
			// Step 2: explicitly unblock with blocked = false
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac     = %q
  name    = "tfacc-block-toggle"
  blocked = false
}
`, mac),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "blocked", "false"),
				),
			},
		},
	})
}

func TestAccClientDevice_import(t *testing.T) {
	mac := randomMAC()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac  = %q
  name = "tfacc-import"
}
`, mac),
			},
			{
				ResourceName:      "terrifi_client_device.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccClientDevice_importSiteID(t *testing.T) {
	mac := randomMAC()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac  = %q
  name = "tfacc-import-site"
}
`, mac),
			},
			{
				ResourceName:      "terrifi_client_device.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["terrifi_client_device.test"]
					if rs == nil {
						return "", fmt.Errorf("resource not found in state")
					}
					return fmt.Sprintf("%s:%s", rs.Primary.Attributes["site"], rs.Primary.Attributes["id"]), nil
				},
			},
		},
	})
}

func TestAccClientDevice_idempotent(t *testing.T) {
	mac := randomMAC()
	config := fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac  = %q
  name = "tfacc-idempotent"
}
`, mac)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_client_device.test", "name", "tfacc-idempotent"),
				),
			},
			{
				// Apply the same config again — should produce no diff.
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}
