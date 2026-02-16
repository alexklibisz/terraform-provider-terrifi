package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// randomSuffix returns a short random string to make test record names unique,
// avoiding 400 errors from leftover records when destroy fails.
func randomSuffix() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

// ---------------------------------------------------------------------------
// Unit tests — no TF_ACC, no network, no env vars needed
// ---------------------------------------------------------------------------

// TestDNSRecordModelToAPI verifies that our Terraform model converts to the
// correct go-unifi API struct. This is where field mapping bugs hide — e.g.,
// model.Name maps to API Key (not Name), model.TTL maps to API Ttl (not TTL).
func TestDNSRecordModelToAPI(t *testing.T) {
	r := &dnsRecordResource{}

	t.Run("required fields only", func(t *testing.T) {
		model := &dnsRecordResourceModel{
			Name:    types.StringValue("test.home"),
			Value:   types.StringValue("192.168.1.100"),
			Enabled: types.BoolValue(true),
			// All optional fields left as zero values (null)
		}

		rec := r.modelToAPI(model)

		assert.Equal(t, "test.home", rec.Key)
		assert.Equal(t, "192.168.1.100", rec.Value)
		assert.True(t, rec.Enabled)
		assert.Nil(t, rec.Port)
		assert.Zero(t, rec.Priority)
		assert.Empty(t, rec.RecordType)
		assert.Zero(t, rec.Ttl)
		assert.Zero(t, rec.Weight)
	})

	t.Run("all fields set", func(t *testing.T) {
		model := &dnsRecordResourceModel{
			Name:       types.StringValue("_sip._tcp.example.com"),
			Value:      types.StringValue("sipserver.example.com"),
			Enabled:    types.BoolValue(true),
			Port:       types.Int64Value(5060),
			Priority:   types.Int64Value(10),
			RecordType: types.StringValue("SRV"),
			TTL:        types.Int64Value(3600),
			Weight:     types.Int64Value(20),
		}

		rec := r.modelToAPI(model)

		assert.Equal(t, "_sip._tcp.example.com", rec.Key)
		assert.Equal(t, "sipserver.example.com", rec.Value)
		assert.True(t, rec.Enabled)
		require.NotNil(t, rec.Port) // require stops the test if nil (avoids panic on *rec.Port below)
		assert.Equal(t, int64(5060), *rec.Port)
		assert.Equal(t, int64(10), rec.Priority)
		assert.Equal(t, "SRV", rec.RecordType)
		assert.Equal(t, int64(3600), rec.Ttl)
		assert.Equal(t, int64(20), rec.Weight)
	})

	t.Run("disabled record", func(t *testing.T) {
		model := &dnsRecordResourceModel{
			Name:    types.StringValue("disabled.home"),
			Value:   types.StringValue("10.0.0.1"),
			Enabled: types.BoolValue(false),
		}

		rec := r.modelToAPI(model)

		assert.False(t, rec.Enabled)
	})
}

// TestDNSRecordAPIToModel verifies that API responses convert back to our
// Terraform model correctly, including the null handling for optional fields.
func TestDNSRecordAPIToModel(t *testing.T) {
	r := &dnsRecordResource{}

	t.Run("minimal record", func(t *testing.T) {
		rec := &unifi.DNSRecord{
			ID:      "abc123",
			Key:     "test.home",
			Value:   "192.168.1.100",
			Enabled: true,
		}

		var model dnsRecordResourceModel
		r.apiToModel(rec, &model, "default")

		assert.Equal(t, "abc123", model.ID.ValueString())
		assert.Equal(t, "default", model.Site.ValueString())
		assert.Equal(t, "test.home", model.Name.ValueString())
		assert.Equal(t, "192.168.1.100", model.Value.ValueString())
		assert.True(t, model.Enabled.ValueBool())

		// Optional fields with zero API values should be null in the model,
		// not zero. This prevents Terraform from showing spurious diffs like
		// "port: 0 → null" or "ttl: 0 → null".
		assert.True(t, model.Port.IsNull(), "Port should be null")
		assert.True(t, model.Priority.IsNull(), "Priority should be null")
		assert.True(t, model.RecordType.IsNull(), "RecordType should be null")
		assert.True(t, model.TTL.IsNull(), "TTL should be null")
		assert.True(t, model.Weight.IsNull(), "Weight should be null")
	})

	t.Run("full SRV record", func(t *testing.T) {
		port := int64(5060)
		rec := &unifi.DNSRecord{
			ID:         "def456",
			Key:        "_sip._tcp.example.com",
			Value:      "sipserver.example.com",
			Enabled:    true,
			Port:       &port,
			Priority:   10,
			RecordType: "SRV",
			Ttl:        3600,
			Weight:     20,
		}

		var model dnsRecordResourceModel
		r.apiToModel(rec, &model, "mysite")

		assert.Equal(t, "mysite", model.Site.ValueString())
		assert.Equal(t, int64(5060), model.Port.ValueInt64())
		assert.Equal(t, int64(10), model.Priority.ValueInt64())
		assert.Equal(t, "SRV", model.RecordType.ValueString())
		assert.Equal(t, int64(3600), model.TTL.ValueInt64())
		assert.Equal(t, int64(20), model.Weight.ValueInt64())
	})
}

// TestDNSRecordApplyPlanToState verifies the merge logic that handles updates.
// When a user changes some fields, applyPlanToState should update those fields
// in state while leaving unchanged fields (null/unknown in the plan) alone.
func TestDNSRecordApplyPlanToState(t *testing.T) {
	r := &dnsRecordResource{}

	t.Run("partial update preserves unchanged fields", func(t *testing.T) {
		state := &dnsRecordResourceModel{
			Name:       types.StringValue("test.home"),
			Value:      types.StringValue("192.168.1.100"),
			Enabled:    types.BoolValue(true),
			RecordType: types.StringValue("A"),
			TTL:        types.Int64Value(300),
		}

		// Plan only changes the value — everything else is null (not in config).
		plan := &dnsRecordResourceModel{
			Name:       types.StringValue("test.home"),
			Value:      types.StringValue("192.168.1.200"), // changed
			Enabled:    types.BoolValue(true),
			RecordType: types.StringNull(), // not in plan
			TTL:        types.Int64Null(),  // not in plan
		}

		r.applyPlanToState(plan, state)

		// Changed field should be updated.
		assert.Equal(t, "192.168.1.200", state.Value.ValueString())
		// Unchanged fields should be preserved from state.
		assert.Equal(t, "A", state.RecordType.ValueString())
		assert.Equal(t, int64(300), state.TTL.ValueInt64())
	})

	t.Run("all fields updated", func(t *testing.T) {
		state := &dnsRecordResourceModel{
			Name:       types.StringValue("old.home"),
			Value:      types.StringValue("10.0.0.1"),
			Enabled:    types.BoolValue(true),
			RecordType: types.StringValue("A"),
		}

		plan := &dnsRecordResourceModel{
			Name:       types.StringValue("new.home"),
			Value:      types.StringValue("10.0.0.2"),
			Enabled:    types.BoolValue(false),
			RecordType: types.StringValue("CNAME"),
		}

		r.applyPlanToState(plan, state)

		assert.Equal(t, "new.home", state.Name.ValueString())
		assert.Equal(t, "10.0.0.2", state.Value.ValueString())
		assert.False(t, state.Enabled.ValueBool())
		assert.Equal(t, "CNAME", state.RecordType.ValueString())
	})
}

// ---------------------------------------------------------------------------
// Acceptance tests — require TF_ACC=1 and a UniFi controller (Docker or hardware)
// ---------------------------------------------------------------------------

// TestAccDNSRecord_basic tests the full lifecycle: create, read-back, destroy.
func TestAccDNSRecord_basic(t *testing.T) {
	name := fmt.Sprintf("tfacc-basic-%s.home", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "192.168.1.200"
  record_type = "A"
  enabled     = true
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "name", name),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "value", "192.168.1.200"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "record_type", "A"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "enabled", "true"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "site", "default"),
					resource.TestCheckResourceAttrSet("terrifi_dns_record.test", "id"),
				),
			},
		},
	})
}

// TestAccDNSRecord_update tests that changing a field triggers an update (not recreate).
func TestAccDNSRecord_update(t *testing.T) {
	name := fmt.Sprintf("tfacc-update-%s.home", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "192.168.1.201"
  record_type = "A"
  enabled     = true
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "value", "192.168.1.201"),
				),
			},
			{
				// Change the value — should update in place, not destroy+recreate.
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "192.168.1.202"
  record_type = "A"
  enabled     = true
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "value", "192.168.1.202"),
				),
			},
		},
	})
}

// TestAccDNSRecord_import tests that an existing record can be imported by ID.
func TestAccDNSRecord_import(t *testing.T) {
	name := fmt.Sprintf("tfacc-import-%s.home", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "192.168.1.203"
  record_type = "A"
  enabled     = true
}
`, name),
			},
			{
				ResourceName:      "terrifi_dns_record.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccDNSRecord_importSiteID tests import using the "site:id" format.
func TestAccDNSRecord_importSiteID(t *testing.T) {
	name := fmt.Sprintf("tfacc-impsid-%s.home", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "192.168.1.204"
  record_type = "A"
}
`, name),
			},
			{
				ResourceName:            "terrifi_dns_record.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["terrifi_dns_record.test"]
					if rs == nil {
						return "", fmt.Errorf("resource not found in state")
					}
					return fmt.Sprintf("%s:%s", rs.Primary.Attributes["site"], rs.Primary.Attributes["id"]), nil
				},
			},
		},
	})
}

// TestAccDNSRecord_srvRecord tests creating an SRV record with port, priority, weight, and ttl.
func TestAccDNSRecord_srvRecord(t *testing.T) {
	name := fmt.Sprintf("_tfacc._tcp.srv-%s.home", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "target.example.com"
  record_type = "SRV"
  port        = 8080
  priority    = 10
  weight      = 50
  ttl         = 3600
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "name", name),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "value", "target.example.com"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "record_type", "SRV"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "port", "8080"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "priority", "10"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "weight", "50"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "ttl", "3600"),
				),
			},
		},
	})
}

// TestAccDNSRecord_disabled tests creating a record with enabled = false.
func TestAccDNSRecord_disabled(t *testing.T) {
	name := fmt.Sprintf("tfacc-disabled-%s.home", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "192.168.1.205"
  record_type = "A"
  enabled     = false
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "name", name),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "enabled", "false"),
				),
			},
		},
	})
}

// TestAccDNSRecord_updateOptionalFields tests updating TTL and toggling enabled.
func TestAccDNSRecord_updateOptionalFields(t *testing.T) {
	name := fmt.Sprintf("tfacc-updopt-%s.home", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "192.168.1.206"
  record_type = "A"
  ttl         = 300
  enabled     = true
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "ttl", "300"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "enabled", "true"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "192.168.1.206"
  record_type = "A"
  ttl         = 600
  enabled     = false
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "ttl", "600"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "enabled", "false"),
				),
			},
		},
	})
}

// TestAccDNSRecord_updateAddTTL tests adding a TTL to a record that was created without one.
func TestAccDNSRecord_updateAddTTL(t *testing.T) {
	name := fmt.Sprintf("tfacc-addttl-%s.home", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "192.168.1.207"
  record_type = "A"
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "name", name),
					resource.TestCheckNoResourceAttr("terrifi_dns_record.test", "ttl"),
				),
			},
			{
				// Add TTL where there wasn't one before.
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "192.168.1.207"
  record_type = "A"
  ttl         = 900
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "ttl", "900"),
				),
			},
		},
	})
}

// TestAccDNSRecord_updateReEnable tests toggling a record from disabled back to enabled.
func TestAccDNSRecord_updateReEnable(t *testing.T) {
	name := fmt.Sprintf("tfacc-reenable-%s.home", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "192.168.1.208"
  record_type = "A"
  enabled     = false
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "enabled", "false"),
				),
			},
			{
				// Re-enable the record.
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "192.168.1.208"
  record_type = "A"
  enabled     = true
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "enabled", "true"),
				),
			},
		},
	})
}

// TestAccDNSRecord_updateSRVFields tests updating port, priority, and weight on an SRV record.
func TestAccDNSRecord_updateSRVFields(t *testing.T) {
	name := fmt.Sprintf("_tfacc._tcp.updsrv-%s.home", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "srv1.example.com"
  record_type = "SRV"
  port        = 8080
  priority    = 10
  weight      = 50
  ttl         = 3600
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "port", "8080"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "priority", "10"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "weight", "50"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "ttl", "3600"),
				),
			},
			{
				// Update all SRV-specific fields and the target value.
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "srv2.example.com"
  record_type = "SRV"
  port        = 9090
  priority    = 20
  weight      = 100
  ttl         = 7200
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "value", "srv2.example.com"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "port", "9090"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "priority", "20"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "weight", "100"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "ttl", "7200"),
				),
			},
		},
	})
}

// TestAccDNSRecord_updateMultipleFields tests changing value, TTL, and enabled in one step.
func TestAccDNSRecord_updateMultipleFields(t *testing.T) {
	name := fmt.Sprintf("tfacc-updmulti-%s.home", randomSuffix())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "192.168.1.210"
  record_type = "A"
  ttl         = 300
  enabled     = true
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "value", "192.168.1.210"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "ttl", "300"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "enabled", "true"),
				),
			},
			{
				// Change value, TTL, and enabled all at once.
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "10.0.0.50"
  record_type = "A"
  ttl         = 1800
  enabled     = false
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "value", "10.0.0.50"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "ttl", "1800"),
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "enabled", "false"),
				),
			},
		},
	})
}
