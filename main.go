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
	format := flag.String("format", "", "Output format override: csv, json, excel, html")
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
		fmt.Printf("%-30s %-15s %-10s %s\n", "ID", "Data Source", "Builtin", "Name")
		fmt.Println(strings.Repeat("-", 80))
		for _, t := range templates {
			builtin := ""
			if t.Builtin {
				builtin = "yes"
			}
			fmt.Printf("%-30s %-15s %-10s %s\n", t.ID, t.DataSource, builtin, t.Name)
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

	req := GenerateRequest{
		Name:       t.Name,
		DataSource: t.DataSource,
		AppID:      *appID,
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
	data, err := ExecuteReport(client, req, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll("output", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	safeName := sanitizeFilename(t.Name)
	ts := data.Metadata["Generated At"]
	ts = strings.ReplaceAll(ts, " ", "_")
	ts = strings.ReplaceAll(ts, ":", "")
	baseName := fmt.Sprintf("c1_%s_%s", safeName, ts)

	files, digest, err := writeReport(data, "output", baseName, req.Format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for _, f := range files {
		fmt.Printf("  output/%s\n", f)
	}
	fmt.Printf("  SHA-256: %s\n", digest)
	fmt.Println("\nDone.")
}
