package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

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
	cmd.Flags().Bool("html", false, "Generate a browsable directory (unifi-device-types/) with icons and fuzzy search")
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
		site := cfg.Site
		if site == "" {
			site = "default"
		}
		version, err := client.GetControllerVersion(ctx, site)
		if err != nil {
			version = "unknown"
		}
		return writeDeviceTypesHTML(devices, version)
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

// downloadIcons downloads device icons into imgDir, skipping any that already
// exist on disk. Uses concurrent downloads with progress reporting.
func downloadIcons(devices []provider.FingerprintDevice, imgDir string) {
	const concurrency = 50
	sem := make(chan struct{}, concurrency)

	var mu sync.Mutex
	var wg sync.WaitGroup
	done := 0
	skipped := 0
	total := len(devices)

	for _, d := range devices {
		wg.Add(1)
		sem <- struct{}{}
		go func(id int64) {
			defer wg.Done()
			defer func() { <-sem }()

			path := filepath.Join(imgDir, fmt.Sprintf("%d.png", id))

			// Skip if already downloaded.
			if _, err := os.Stat(path); err == nil {
				mu.Lock()
				done++
				skipped++
				mu.Unlock()
				return
			}

			url := fmt.Sprintf("https://static.ui.com/fingerprint/0/%d_51x51.png", id)
			resp, err := http.Get(url)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return
			}
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return
			}

			os.WriteFile(path, data, 0o644)

			mu.Lock()
			done++
			if done%200 == 0 || done == total {
				fmt.Fprintf(os.Stderr, "Icons: %d / %d (%d already existed)...\n", done, total, skipped)
			}
			mu.Unlock()
		}(d.ID)
	}

	wg.Wait()
	fmt.Fprintf(os.Stderr, "Icons: %d / %d (%d downloaded, %d already existed).\n", done, total, done-skipped, skipped)
}

func writeDeviceTypesHTML(devices []provider.FingerprintDevice, controllerVersion string) error {
	const outputDir = "unifi-device-types"
	imgDir := filepath.Join(outputDir, "img")
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", imgDir, err)
	}

	// Download icons into img/ directory (skips existing files).
	downloadIcons(devices, imgDir)

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

	// Build JSON data for devices.
	type deviceJSON struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Type    string `json:"type"`
		Vendor  string `json:"vendor"`
	}
	jsonDevices := make([]deviceJSON, len(devices))
	for i, d := range devices {
		jsonDevices[i] = deviceJSON{
			ID:     d.ID,
			Name:   d.Name,
			Type:   d.DevType,
			Vendor: d.Vendor,
		}
	}
	jsonData, err := json.Marshal(jsonDevices)
	if err != nil {
		return fmt.Errorf("marshaling device data: %w", err)
	}

	outputFile := filepath.Join(outputDir, "index.html")
	f, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("creating %s: %w", outputFile, err)
	}
	defer f.Close()

	// Write the static HTML shell with embedded JSON data.
	// The JS renders rows on demand instead of parsing thousands of DOM nodes.
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
  .header h1 { font-size: 20px; margin-bottom: 4px; }
  .header .subtitle { font-size: 14px; color: #bbb; margin-bottom: 8px; line-height: 1.4; }
  .header .subtitle a { color: #ccc; text-decoration: underline; }
  .header .version { font-size: 13px; color: #888; margin-bottom: 12px; }
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
  table { width: 100%; border-collapse: collapse; background: #fff; table-layout: fixed; }
  col.c-icon { width: 64px; }
  col.c-id { width: 100px; }
  col.c-name { width: 30%; }
  col.c-meta { }
  col.c-action { width: 80px; }
  th { background: #e8e8e8; position: sticky; text-align: left; padding: 10px 14px; font-size: 13px; z-index: 5; }
  td { padding: 8px 14px; border-bottom: 1px solid #eee; vertical-align: middle; }
  .icon-cell { width: 48px; height: 48px; background: #f0f0f0; border-radius: 6px; }
  .icon { width: 48px; height: 48px; object-fit: contain; }
  .icon:not([src]) { visibility: hidden; }
  @keyframes pulse { 0%, 100% { opacity: 0.4; } 50% { opacity: 1; } }
  .icon-cell:has(.icon:not([src]))::after {
    content: ''; display: block; width: 24px; height: 24px; margin: -36px auto 0;
    border-radius: 50%; background: #ddd; animation: pulse 1.2s ease-in-out infinite;
  }
  .id { font-family: monospace; font-size: 14px; color: #666; }
  .name { font-weight: 500; }
  .meta { font-size: 12px; color: #888; }
  .copy-btn {
    background: #e8e8e8; border: none; border-radius: 4px; padding: 4px 10px;
    font-size: 12px; cursor: pointer; color: #555; white-space: nowrap;
  }
  .copy-btn:hover { background: #ddd; }
  .copy-btn.copied { background: #4caf50; color: #fff; }
</style>
</head>
<body>
<div class="header">
  <h1>UniFi Device Types</h1>
  <p class="subtitle">Browse the UniFi controller fingerprint database to find device type IDs for use with <a href="https://github.com/alexklibisz/terraform-provider-terrifi">Terrifi</a>, a Terraform provider for UniFi.</p>
`)
	fmt.Fprintf(f, "  <div class=\"version\">Generated from controller version %s</div>\n", controllerVersion)
	fmt.Fprint(f, `  <div class="controls">
    <input type="text" id="search" placeholder="Search by name or ID..." autofocus>
`)
	fmt.Fprintf(f, "    <select id=\"filter-type\"><option value=\"\">All Types (%d)</option>\n", len(types))
	for _, t := range types {
		fmt.Fprintf(f, "    <option>%s</option>\n", t)
	}
	fmt.Fprint(f, "    </select>\n")
	fmt.Fprintf(f, "    <select id=\"filter-vendor\"><option value=\"\">All Vendors (%d)</option>\n", len(vendors))
	for _, v := range vendors {
		fmt.Fprintf(f, "    <option>%s</option>\n", v)
	}
	fmt.Fprint(f, `    </select>
  </div>
`)
	fmt.Fprintf(f, "  <div class=\"stats\" id=\"stats\">%d device types</div>\n", len(devices))
	fmt.Fprint(f, `</div>
<table>
<colgroup><col class="c-icon"><col class="c-id"><col class="c-name"><col class="c-meta"><col class="c-action"></colgroup>
<thead><tr><th>Icon</th><th>ID</th><th>Name</th><th>Type / Vendor</th><th></th></tr></thead>
<tbody id="tbody"></tbody>
</table>
<script>
const DATA = `)
	f.Write(jsonData)
	fmt.Fprint(f, `;

const tbody = document.getElementById('tbody');
const totalCount = DATA.length;

// Pre-build a DOM row for each device and index by string ID.
const rowById = {};
const rows = [];
const frag = document.createDocumentFragment();
for (const d of DATA) {
  const tr = document.createElement('tr');
  tr.innerHTML =
    '<td class="icon-cell"><img class="icon" data-src="img/' + d.id + '.png" alt="' + d.name + '"></td>' +
    '<td class="id">' + d.id + '</td>' +
    '<td class="name">' + d.name + '</td>' +
    '<td class="meta">' + d.type + ' \u00b7 ' + d.vendor + '</td>' +
    '<td><button class="copy-btn">Copy</button></td>';
  tr._d = d;
  tr._copyText = 'device_type_id = ' + d.id + ' # ' + d.name;
  rowById[String(d.id)] = tr;
  rows.push(tr);
  frag.appendChild(tr);
}
tbody.appendChild(frag);

// Pin table header row just below the sticky page header.
const header = document.querySelector('.header');
const ths = document.querySelectorAll('th');
new ResizeObserver(() => {
  const h = header.offsetHeight + 'px';
  ths.forEach(th => th.style.top = h);
}).observe(header);

// Lazy load images when they scroll into view.
const observer = new IntersectionObserver((entries) => {
  entries.forEach(entry => {
    if (entry.isIntersecting) {
      const img = entry.target;
      if (!img.src && img.dataset.src) {
        img.src = img.dataset.src;
        observer.unobserve(img);
      }
    }
  });
}, { rootMargin: '200px' });
rows.forEach(r => observer.observe(r.querySelector('.icon')));

// Copy button handler.
document.addEventListener('click', (e) => {
  const btn = e.target.closest('.copy-btn');
  if (!btn) return;
  const tr = btn.closest('tr');
  navigator.clipboard.writeText(tr._copyText).then(() => {
    btn.textContent = 'Copied!';
    btn.classList.add('copied');
    setTimeout(() => { btn.textContent = 'Copy'; btn.classList.remove('copied'); }, 1500);
  });
});

const fuse = new Fuse(DATA, {
  keys: [{name: 'id', getFn: d => String(d.id)}, 'name', 'type', 'vendor'],
  threshold: 0.3,
  ignoreLocation: true,
});

const searchInput = document.getElementById('search');
const filterType = document.getElementById('filter-type');
const filterVendor = document.getElementById('filter-vendor');
const stats = document.getElementById('stats');

function updateSelectOptions(sel, allLabel, available) {
  const current = sel.value;
  const sorted = Array.from(available).sort();

  sel.innerHTML = '';
  const allOpt = document.createElement('option');
  allOpt.value = '';
  allOpt.textContent = allLabel + ' (' + sorted.length + ')';
  sel.appendChild(allOpt);

  for (const val of sorted) {
    const opt = document.createElement('option');
    opt.value = val;
    opt.textContent = val;
    if (val === current) opt.selected = true;
    sel.appendChild(opt);
  }

  if (current && !available.has(current)) {
    sel.value = '';
  }
}

function applyFilters() {
  const query = searchInput.value.trim();
  const selectedType = filterType.value;
  const selectedVendor = filterVendor.value;

  let orderedIds = null;
  let matchIdSet = null;
  if (query) {
    orderedIds = fuse.search(query).map(r => String(r.item.id));
    matchIdSet = new Set(orderedIds);
  }

  const typesForVendor = new Set();
  const vendorsForType = new Set();
  const visibleSet = new Set();
  rows.forEach(r => {
    const d = r._d;
    const passSearch = !matchIdSet || matchIdSet.has(String(d.id));
    const passType = !selectedType || d.type === selectedType;
    const passVendor = !selectedVendor || d.vendor === selectedVendor;
    if (passSearch && passType && passVendor) visibleSet.add(r);
    if (passSearch && passVendor && d.type) typesForVendor.add(d.type);
    if (passSearch && passType && d.vendor) vendorsForType.add(d.vendor);
  });

  const fragment = document.createDocumentFragment();
  if (orderedIds) {
    for (const id of orderedIds) {
      const row = rowById[id];
      if (visibleSet.has(row)) fragment.appendChild(row);
    }
  } else {
    rows.forEach(r => {
      if (visibleSet.has(r)) fragment.appendChild(r);
    });
  }
  tbody.textContent = '';
  tbody.appendChild(fragment);

  updateSelectOptions(filterType, 'All Types', typesForVendor);
  updateSelectOptions(filterVendor, 'All Vendors', vendorsForType);

  if (!query && !selectedType && !selectedVendor) {
    stats.textContent = totalCount + ' device types';
  } else {
    stats.textContent = visibleSet.size + ' / ' + totalCount + ' device types';
  }
}

let debounceTimer;
searchInput.addEventListener('input', () => {
  clearTimeout(debounceTimer);
  debounceTimer = setTimeout(applyFilters, 150);
});
filterType.addEventListener('change', applyFilters);
filterVendor.addEventListener('change', applyFilters);
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
