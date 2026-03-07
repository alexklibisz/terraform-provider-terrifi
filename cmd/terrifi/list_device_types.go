package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"html"
	"os"
	"sort"
	"strconv"

	"github.com/alexklibisz/terrifi/internal/provider"
	"github.com/spf13/cobra"
)

func listDeviceTypesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-device-types",
		Short: "List available device types (fingerprint IDs) from the UniFi controller",
		Long: "Queries the UniFi controller's fingerprint database and outputs all known " +
			"device types with their IDs as CSV. These IDs can be used as dev_id_override " +
			"values to set custom icons on client devices.\n\n" +
			"Use --html to generate a browsable HTML page with icons, fuzzy search, " +
			"and filterable type/vendor dropdowns.\n\n" +
			"Requires UNIFI_* environment variables to be configured.",
		Args: cobra.NoArgs,
		RunE: runListDeviceTypes,
	}
	cmd.Flags().Bool("html", false, "Generate a self-contained HTML page (unifi-device-types.html) with icons and fuzzy search")
	return cmd
}

func runListDeviceTypes(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	cfg := provider.ClientConfigFromEnv()
	client, err := provider.NewClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connecting to UniFi controller: %w", err)
	}

	devices, err := client.ListFingerprintDevices(ctx, 0)
	if err != nil {
		return fmt.Errorf("listing device types: %w", err)
	}

	if len(devices) == 0 {
		fmt.Fprintln(os.Stderr, "No device types found.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Found %d device types.\n", len(devices))

	htmlFlag, _ := cmd.Flags().GetBool("html")
	if htmlFlag {
		return writeDeviceTypesHTML(devices)
	}

	return writeDeviceTypesCSV(devices)
}

func iconURL(id int64) string {
	return fmt.Sprintf("https://static.ui.com/fingerprint/0/%d_257x257.png", id)
}

func writeDeviceTypesCSV(devices []provider.FingerprintDevice) error {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	if err := w.Write([]string{"id", "name", "dev_type", "family", "vendor", "icon_url"}); err != nil {
		return err
	}

	for _, d := range devices {
		record := []string{
			strconv.FormatInt(d.ID, 10),
			d.Name,
			d.DevType,
			d.Family,
			d.Vendor,
			iconURL(d.ID),
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	return nil
}

func writeDeviceTypesHTML(devices []provider.FingerprintDevice) error {
	// Collect unique types and vendors for the filter dropdowns.
	typeSet := map[string]bool{}
	vendorSet := map[string]bool{}
	for _, d := range devices {
		if d.DevType != "" {
			typeSet[d.DevType] = true
		}
		if d.Vendor != "" {
			vendorSet[d.Vendor] = true
		}
	}
	types := sortedKeys(typeSet)
	vendors := sortedKeys(vendorSet)

	const outputFile = "unifi-device-types.html"
	f, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("creating %s: %w", outputFile, err)
	}
	defer f.Close()

	fmt.Fprint(f, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>UniFi Device Types</title>
<script src="https://cdn.jsdelivr.net/npm/fuse.js@7.0.0/dist/fuse.min.js"></script>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #333; }
  .header { background: #1a1a2e; color: #fff; padding: 24px; position: sticky; top: 0; z-index: 10; }
  .header h1 { font-size: 20px; margin-bottom: 12px; }
  .controls { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; }
  .controls input {
    flex: 1; min-width: 200px; max-width: 500px; padding: 10px 14px; font-size: 16px;
    border: none; border-radius: 6px; outline: none;
  }
  .controls select {
    padding: 10px 14px; font-size: 14px; border: none; border-radius: 6px;
    background: #fff; color: #333; cursor: pointer; outline: none;
  }
  .header .stats { font-size: 13px; color: #aaa; margin-top: 8px; }
  table { width: 100%; border-collapse: collapse; background: #fff; }
  th { background: #e8e8e8; position: sticky; top: 130px; text-align: left; padding: 10px 14px; font-size: 13px; }
  td { padding: 8px 14px; border-bottom: 1px solid #eee; vertical-align: middle; }
  tr.hidden { display: none; }
  .icon { width: 48px; height: 48px; object-fit: contain; }
  .id { font-family: monospace; font-size: 14px; color: #666; }
  .name { font-weight: 500; }
  .meta { font-size: 12px; color: #888; }
</style>
</head>
<body>
<div class="header">
  <h1>UniFi Device Types</h1>
  <div class="controls">
    <input type="text" id="search" placeholder="Fuzzy search by name..." autofocus>
    <select id="filter-type"><option value="">All Types</option>
`)

	for _, t := range types {
		fmt.Fprintf(f, "    <option value=\"%s\">%s</option>\n", html.EscapeString(t), html.EscapeString(t))
	}

	fmt.Fprint(f, `    </select>
    <select id="filter-vendor"><option value="">All Vendors</option>
`)

	for _, v := range vendors {
		fmt.Fprintf(f, "    <option value=\"%s\">%s</option>\n", html.EscapeString(v), html.EscapeString(v))
	}

	fmt.Fprint(f, `    </select>
  </div>
  <div class="stats" id="stats"></div>
</div>
<table>
<thead><tr><th>Icon</th><th>ID</th><th>Name</th><th>Type / Vendor</th></tr></thead>
<tbody id="tbody">
`)

	for _, d := range devices {
		fmt.Fprintf(f, `<tr data-id="%d" data-type="%s" data-vendor="%s"><td><img class="icon" src="%s" alt="%s" loading="lazy"></td><td class="id">%d</td><td class="name">%s</td><td class="meta">%s · %s</td></tr>
`,
			d.ID,
			html.EscapeString(d.DevType),
			html.EscapeString(d.Vendor),
			iconURL(d.ID),
			html.EscapeString(d.Name),
			d.ID,
			html.EscapeString(d.Name),
			html.EscapeString(d.DevType),
			html.EscapeString(d.Vendor),
		)
	}

	fmt.Fprint(f, `</tbody>
</table>
<script>
const tbody = document.getElementById('tbody');
const rows = Array.from(tbody.querySelectorAll('tr'));
const totalCount = rows.length;
const rowById = {};
rows.forEach(r => rowById[r.dataset.id] = r);

const items = rows.map(row => ({
  id: row.dataset.id,
  name: row.querySelector('.name')?.textContent || '',
  meta: row.querySelector('.meta')?.textContent || '',
}));

const fuse = new Fuse(items, {
  keys: ['name', 'meta'],
  threshold: 0.3,
  ignoreLocation: true,
});

const searchInput = document.getElementById('search');
const filterType = document.getElementById('filter-type');
const filterVendor = document.getElementById('filter-vendor');
const stats = document.getElementById('stats');

function updateSelectOptions(select, attr, allLabel, visibleRows) {
  const current = select.value;
  const available = new Set();
  visibleRows.forEach(r => {
    const v = r.dataset[attr];
    if (v) available.add(v);
  });
  const sorted = Array.from(available).sort();

  select.innerHTML = '';
  const allOpt = document.createElement('option');
  allOpt.value = '';
  allOpt.textContent = allLabel + ' (' + sorted.length + ')';
  select.appendChild(allOpt);

  for (const val of sorted) {
    const opt = document.createElement('option');
    opt.value = val;
    opt.textContent = val;
    if (val === current) opt.selected = true;
    select.appendChild(opt);
  }

  if (current && !available.has(current)) {
    select.value = '';
  }
}

function applyFilters() {
  const query = searchInput.value.trim();
  const selectedType = filterType.value;
  const selectedVendor = filterVendor.value;

  let orderedIds;
  if (query) {
    orderedIds = fuse.search(query).map(r => r.item.id);
  } else {
    orderedIds = rows.map(r => r.dataset.id);
  }

  const matchIdSet = query ? new Set(orderedIds) : null;

  let shown = 0;
  rows.forEach(r => {
    const passSearch = !matchIdSet || matchIdSet.has(r.dataset.id);
    const passType = !selectedType || r.dataset.type === selectedType;
    const passVendor = !selectedVendor || r.dataset.vendor === selectedVendor;

    if (passSearch && passType && passVendor) {
      r.classList.remove('hidden');
      shown++;
    } else {
      r.classList.add('hidden');
    }
  });

  if (query) {
    for (const id of orderedIds) {
      const row = rowById[id];
      if (!row.classList.contains('hidden')) {
        tbody.appendChild(row);
      }
    }
  } else {
    rows.forEach(r => tbody.appendChild(r));
  }

  const rowsForType = rows.filter(r => {
    const passSearch = !matchIdSet || matchIdSet.has(r.dataset.id);
    const passVendor = !selectedVendor || r.dataset.vendor === selectedVendor;
    return passSearch && passVendor;
  });
  const rowsForVendor = rows.filter(r => {
    const passSearch = !matchIdSet || matchIdSet.has(r.dataset.id);
    const passType = !selectedType || r.dataset.type === selectedType;
    return passSearch && passType;
  });

  updateSelectOptions(filterType, 'type', 'All Types', rowsForType);
  updateSelectOptions(filterVendor, 'vendor', 'All Vendors', rowsForVendor);

  if (!query && !selectedType && !selectedVendor) {
    stats.textContent = totalCount + ' device types';
  } else {
    stats.textContent = shown + ' / ' + totalCount + ' device types';
  }
}

searchInput.addEventListener('input', applyFilters);
filterType.addEventListener('change', applyFilters);
filterVendor.addEventListener('change', applyFilters);
applyFilters();
</script>
</body>
</html>
`)

	fmt.Fprintf(os.Stderr, "Wrote %s\n", outputFile)
	return nil
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
