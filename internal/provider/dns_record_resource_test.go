package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

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
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
terraform {
  required_providers {
    terrifi = {
      source = "alexklibisz/terrifi"
    }
  }
}

resource "terrifi_dns_record" "test" {
  name        = "tfacc-basic.home"
  value       = "192.168.1.200"
  record_type = "A"
  enabled     = true
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "name", "tfacc-basic.home"),
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
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
terraform {
  required_providers {
    terrifi = {
      source = "alexklibisz/terrifi"
    }
  }
}

resource "terrifi_dns_record" "test" {
  name        = "tfacc-update.home"
  value       = "192.168.1.201"
  record_type = "A"
  enabled     = true
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "value", "192.168.1.201"),
				),
			},
			{
				// Change the value — should update in place, not destroy+recreate.
				Config: `
terraform {
  required_providers {
    terrifi = {
      source = "alexklibisz/terrifi"
    }
  }
}

resource "terrifi_dns_record" "test" {
  name        = "tfacc-update.home"
  value       = "192.168.1.202"
  record_type = "A"
  enabled     = true
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("terrifi_dns_record.test", "value", "192.168.1.202"),
				),
			},
		},
	})
}

// TestAccDNSRecord_import tests that an existing record can be imported by ID.
func TestAccDNSRecord_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
terraform {
  required_providers {
    terrifi = {
      source = "alexklibisz/terrifi"
    }
  }
}

resource "terrifi_dns_record" "test" {
  name        = "tfacc-import.home"
  value       = "192.168.1.203"
  record_type = "A"
  enabled     = true
}
`,
			},
			{
				ResourceName:      "terrifi_dns_record.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
