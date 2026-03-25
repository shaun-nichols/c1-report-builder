package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// Version is set at build time via ldflags
var Version = "dev"

func main() {
	templateID := flag.String("template", "", "Run a report template by ID")
	appID := flag.String("app-id", "", "ConductorOne App ID")
	appName := flag.String("app-name", "", "ConductorOne App display name")
	allApps := flag.Bool("all-apps", false, "Run report across all apps")
	format := flag.String("format", "", "Output format override: csv, json, excel, html")
	outputDir := flag.String("output-dir", "output", "Directory to write report files")
	listTemplates := flag.Bool("list-templates", false, "List available templates")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("c1-report-builder " + Version)
		return
	}

	// No flags = launch web UI
	if !*listTemplates && *templateID == "" {
		startWebServer()
		return
	}

	// CLI mode
	initDataSources()
	loadEnv()

	store := NewTemplateStore("templates")

	if *listTemplates {
		templates := store.List()
		fmt.Printf("%-35s %-15s %-10s %s\n", "ID", "Data Source", "Builtin", "Name")
		fmt.Println(strings.Repeat("-", 90))
		for _, t := range templates {
			builtin := ""
			if t.Builtin {
				builtin = "yes"
			}
			fmt.Printf("%-35s %-15s %-10s %s\n", t.ID, t.DataSource, builtin, t.Name)
		}
		return
	}

	// Credentials required for running templates
	clientID := os.Getenv("C1_CLIENT_ID")
	clientSecret := os.Getenv("C1_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		fmt.Fprintln(os.Stderr, "Error: C1_CLIENT_ID and C1_CLIENT_SECRET must be set for CLI mode.")
		os.Exit(1)
	}
	client, err := NewC1Client(clientID, clientSecret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	t, ok := store.Get(*templateID)
	if !ok {
		fmt.Fprintf(os.Stderr, "Template '%s' not found. Use --list-templates to see available templates.\n", *templateID)
		os.Exit(1)
	}

	// Resolve app selection
	resolvedAppID := *appID
	var appIDs []string

	if *allApps {
		appIDs = []string{"all"}
	} else if *appName != "" && resolvedAppID == "" {
		// Look up app by name
		fmt.Printf("Looking up app '%s'...\n", *appName)
		appViews, err := client.ListApps()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing apps: %v\n", err)
			os.Exit(1)
		}
		found := false
		for _, av := range appViews {
			app := av.App()
			if strings.EqualFold(app.DisplayName, *appName) {
				resolvedAppID = app.ID
				found = true
				fmt.Printf("  Found: %s (%s)\n", app.DisplayName, app.ID)
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Error: no app found matching '%s'\n", *appName)
			os.Exit(1)
		}
	}

	// Build apps list for multi-app resolution
	var apps []appJSON
	if len(appIDs) > 0 && appIDs[0] == "all" {
		fmt.Println("Fetching app list for all-apps mode...")
		appViews, err := client.ListApps()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing apps: %v\n", err)
			os.Exit(1)
		}
		for _, av := range appViews {
			app := av.App()
			apps = append(apps, appJSON{ID: app.ID, DisplayName: app.DisplayName, UserCount: app.UserCount})
		}
		fmt.Printf("  Running across %d apps\n", len(apps))
	}

	req := GenerateRequest{
		Name:       t.Name,
		DataSource: t.DataSource,
		AppID:      resolvedAppID,
		AppIDs:     appIDs,
		Columns:    t.Columns,
		Filters:    t.Filters,
		SortBy:     t.SortBy,
		SortDesc:   t.SortDesc,
		Format:     t.Format,
	}
	if *format != "" {
		req.Format = *format
	}

	fmt.Printf("Running template: %s\n", t.Name)
	data, err := ExecuteReport(client, req, apps)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	safeName := sanitizeFilename(t.Name)
	ts := data.Metadata["Generated At"]
	ts = strings.ReplaceAll(ts, " ", "_")
	ts = strings.ReplaceAll(ts, ":", "")
	baseName := fmt.Sprintf("c1_%s_%s", safeName, ts)

	files, digest, err := writeReport(data, *outputDir, baseName, req.Format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for _, f := range files {
		fmt.Printf("  %s/%s\n", *outputDir, f)
	}
	fmt.Printf("  SHA-256: %s\n", digest)
	fmt.Printf("  %s rows\n", data.Metadata["Total Rows"])
	fmt.Println("\nDone.")
}
