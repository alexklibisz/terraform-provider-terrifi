package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexklibisz/terrifi/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"
)

var cliBinary string

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"terrifi": providerserver.NewProtocol6WithError(provider.New()),
}

func TestMain(m *testing.M) {
	if os.Getenv("TF_ACC") == "" {
		fmt.Fprintln(os.Stderr, "TF_ACC not set, skipping acceptance tests")
		os.Exit(0)
	}

	preCheck()

	// Build the CLI binary to a temp path
	tmpDir, err := os.MkdirTemp("", "terrifi-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %s\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	cliBinary = filepath.Join(tmpDir, "terrifi")
	cmd := exec.Command("go", "build", "-o", cliBinary, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build CLI: %s\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func preCheck() {
	if os.Getenv("UNIFI_API") == "" {
		fmt.Fprintln(os.Stderr, "UNIFI_API must be set for acceptance tests")
		os.Exit(1)
	}
	hasAPIKey := os.Getenv("UNIFI_API_KEY") != ""
	hasCredentials := os.Getenv("UNIFI_USERNAME") != "" && os.Getenv("UNIFI_PASSWORD") != ""
	if !hasAPIKey && !hasCredentials {
		fmt.Fprintln(os.Stderr, "either UNIFI_API_KEY or both UNIFI_USERNAME and UNIFI_PASSWORD must be set for acceptance tests")
		os.Exit(1)
	}
}

func requireHardware(t *testing.T) {
	t.Helper()
	target := os.Getenv("TERRIFI_ACC_TARGET")
	if target == "" || target == "docker" {
		t.Skip("CLI acceptance tests require hardware (TERRIFI_ACC_TARGET=hardware)")
	}
}

func randomSuffix() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

func randomMAC() string {
	return fmt.Sprintf("02:%02x:%02x:%02x:%02x:%02x",
		rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))
}

func randomVLAN() int {
	return 100 + rand.Intn(3900)
}

func runCLI(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command(cliBinary, args...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "CLI failed: %s", string(out))
	return string(out)
}

// resourceID extracts the ID of a resource from Terraform state.
func resourceID(s *terraform.State, addr string) (string, error) {
	rs, ok := s.RootModule().Resources[addr]
	if !ok {
		return "", fmt.Errorf("resource %s not found in state", addr)
	}
	return rs.Primary.ID, nil
}

// assertCLIOutput runs the CLI and checks that the output contains all expected strings.
func assertCLIOutput(t *testing.T, resourceType string, expected ...string) error {
	t.Helper()
	output := runCLI(t, "generate-imports", resourceType)
	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			return fmt.Errorf("CLI output missing expected string %q\n\nFull output:\n%s", exp, output)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Acceptance tests — require TF_ACC=1 and a live UniFi controller
// ---------------------------------------------------------------------------

func TestAccGenerateImports_ClientDevice(t *testing.T) {
	requireHardware(t)
	mac := randomMAC()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_device" "test" {
  mac  = %q
  name = "terrifi-test-client"
}
`, mac),
				Check: func(s *terraform.State) error {
					id, err := resourceID(s, "terrifi_client_device.test")
					if err != nil {
						return err
					}
					return assertCLIOutput(t, "terrifi_client_device",
						fmt.Sprintf(`id = "%s"`, id),
						`to = terrifi_client_device.terrifi_test_client`,
						fmt.Sprintf(`mac = "%s"`, mac),
						`name = "terrifi-test-client"`,
					)
				},
			},
		},
	})
}

func TestAccGenerateImports_ClientGroup(t *testing.T) {
	requireHardware(t)
	name := fmt.Sprintf("terrifi-test-group-%s", randomSuffix())

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_client_group" "test" {
  name = %q
}
`, name),
				Check: func(s *terraform.State) error {
					id, err := resourceID(s, "terrifi_client_group.test")
					if err != nil {
						return err
					}
					tfName := strings.ReplaceAll(name, "-", "_")
					return assertCLIOutput(t, "terrifi_client_group",
						fmt.Sprintf(`id = "%s"`, id),
						fmt.Sprintf(`terrifi_client_group.%s`, tfName),
						fmt.Sprintf(`name = "%s"`, name),
					)
				},
			},
		},
	})
}

func TestAccGenerateImports_DNSRecord(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	name := fmt.Sprintf("terrifi-test-%s.example.com", suffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_dns_record" "test" {
  name        = %q
  value       = "10.0.0.1"
  record_type = "A"
}
`, name),
				Check: func(s *terraform.State) error {
					id, err := resourceID(s, "terrifi_dns_record.test")
					if err != nil {
						return err
					}
					tfName := strings.ReplaceAll(strings.ReplaceAll(name, "-", "_"), ".", "_")
					return assertCLIOutput(t, "terrifi_dns_record",
						fmt.Sprintf(`id = "%s"`, id),
						fmt.Sprintf(`terrifi_dns_record.%s`, tfName),
						fmt.Sprintf(`name = "%s"`, name),
						`value = "10.0.0.1"`,
						`record_type = "A"`,
					)
				},
			},
		},
	})
}

func TestAccGenerateImports_FirewallZone(t *testing.T) {
	requireHardware(t)
	name := fmt.Sprintf("terrifi-test-zone-%s", randomSuffix())

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_zone" "test" {
  name = %q
}
`, name),
				Check: func(s *terraform.State) error {
					id, err := resourceID(s, "terrifi_firewall_zone.test")
					if err != nil {
						return err
					}
					tfName := strings.ReplaceAll(name, "-", "_")
					return assertCLIOutput(t, "terrifi_firewall_zone",
						fmt.Sprintf(`id = "%s"`, id),
						fmt.Sprintf(`terrifi_firewall_zone.%s`, tfName),
						fmt.Sprintf(`name = "%s"`, name),
					)
				},
			},
		},
	})
}

func TestAccGenerateImports_FirewallPolicy(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	zone1Name := fmt.Sprintf("terrifi-test-src-%s", suffix)
	zone2Name := fmt.Sprintf("terrifi-test-dst-%s", suffix)
	policyName := fmt.Sprintf("terrifi-test-policy-%s", suffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_firewall_zone" "src" {
  name = %q
}

resource "terrifi_firewall_zone" "dst" {
  name = %q
}

resource "terrifi_firewall_policy" "test" {
  name   = %q
  action = "ALLOW"

  source {
    zone_id = terrifi_firewall_zone.src.id
  }

  destination {
    zone_id = terrifi_firewall_zone.dst.id
  }
}
`, zone1Name, zone2Name, policyName),
				Check: func(s *terraform.State) error {
					id, err := resourceID(s, "terrifi_firewall_policy.test")
					if err != nil {
						return err
					}
					tfName := strings.ReplaceAll(policyName, "-", "_")
					return assertCLIOutput(t, "terrifi_firewall_policy",
						fmt.Sprintf(`id = "%s"`, id),
						fmt.Sprintf(`terrifi_firewall_policy.%s`, tfName),
						fmt.Sprintf(`name = "%s"`, policyName),
						`action = "ALLOW"`,
					)
				},
			},
		},
	})
}

func TestAccGenerateImports_Network(t *testing.T) {
	requireHardware(t)
	name := fmt.Sprintf("terrifi-test-net-%s", randomSuffix())
	vlan := randomVLAN()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "test" {
  name         = %q
  purpose      = "corporate"
  vlan_id      = %d
  subnet       = "10.%d.%d.1/24"
  dhcp_enabled = false
}
`, name, vlan, vlan/256, vlan%256),
				Check: func(s *terraform.State) error {
					id, err := resourceID(s, "terrifi_network.test")
					if err != nil {
						return err
					}
					tfName := strings.ReplaceAll(name, "-", "_")
					return assertCLIOutput(t, "terrifi_network",
						fmt.Sprintf(`id = "%s"`, id),
						fmt.Sprintf(`terrifi_network.%s`, tfName),
						fmt.Sprintf(`name = "%s"`, name),
						`purpose = "corporate"`,
					)
				},
			},
		},
	})
}

func TestAccGenerateImports_WLAN(t *testing.T) {
	requireHardware(t)
	suffix := randomSuffix()
	netName := fmt.Sprintf("terrifi-test-wlan-net-%s", suffix)
	wlanName := fmt.Sprintf("terrifi-test-wlan-%s", suffix)
	vlan := randomVLAN()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "terrifi_network" "wlan_test" {
  name         = %q
  purpose      = "corporate"
  vlan_id      = %d
  subnet       = "10.%d.%d.1/24"
  dhcp_enabled = false
}

resource "terrifi_wlan" "test" {
  name       = %q
  passphrase = "testpassword123"
  network_id = terrifi_network.wlan_test.id
}
`, netName, vlan, vlan/256, vlan%256, wlanName),
				Check: func(s *terraform.State) error {
					id, err := resourceID(s, "terrifi_wlan.test")
					if err != nil {
						return err
					}
					tfName := strings.ReplaceAll(wlanName, "-", "_")
					return assertCLIOutput(t, "terrifi_wlan",
						fmt.Sprintf(`id = "%s"`, id),
						fmt.Sprintf(`terrifi_wlan.%s`, tfName),
						fmt.Sprintf(`name = "%s"`, wlanName),
						`passphrase = "REPLACE_ME"`,
					)
				},
			},
		},
	})
}
