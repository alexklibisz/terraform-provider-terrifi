package generate

import (
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// DNSRecordBlocks generates import + resource blocks for DNS records.
func DNSRecordBlocks(records []unifi.DNSRecord) []ResourceBlock {
	blocks := make([]ResourceBlock, 0, len(records))
	for _, r := range records {
		block := ResourceBlock{
			ResourceType: "terrifi_dns_record",
			ResourceName: ToTerraformName(r.Key),
			ImportID:     r.ID,
		}

		block.Attributes = append(block.Attributes, Attr{Key: "name", Value: HCLString(r.Key)})
		block.Attributes = append(block.Attributes, Attr{Key: "value", Value: HCLString(r.Value)})
		block.Attributes = append(block.Attributes, Attr{Key: "record_type", Value: HCLString(r.RecordType)})

		if !r.Enabled {
			block.Attributes = append(block.Attributes, Attr{Key: "enabled", Value: HCLBool(false)})
		}
		if r.Port != nil && *r.Port != 0 {
			block.Attributes = append(block.Attributes, Attr{Key: "port", Value: HCLInt64(*r.Port)})
		}
		if r.Priority != 0 {
			block.Attributes = append(block.Attributes, Attr{Key: "priority", Value: HCLInt64(r.Priority)})
		}
		if r.Ttl != 0 {
			block.Attributes = append(block.Attributes, Attr{Key: "ttl", Value: HCLInt64(r.Ttl)})
		}
		if r.Weight != 0 {
			block.Attributes = append(block.Attributes, Attr{Key: "weight", Value: HCLInt64(r.Weight)})
		}

		blocks = append(blocks, block)
	}
	DeduplicateNames(blocks)
	return blocks
}
