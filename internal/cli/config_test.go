package cli

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestParseConfigGroupedRealCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cfg, err := parseConfig([]string{cmdReal, cmdCoastline}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}

	if cfg.Command != cmdCoastline {
		t.Fatalf("expected command %q, got %q", cmdCoastline, cfg.Command)
	}
}

func TestParseConfigSourceCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cfg, err := parseConfig([]string{cmdSource, "--refresh", "--output", "data/snapshots"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}

	if cfg.Command != cmdSource {
		t.Fatalf("expected command %q, got %q", cmdSource, cfg.Command)
	}
	if !cfg.Refresh {
		t.Fatal("expected refresh flag to be true")
	}
	if cfg.OutputPath != "data/snapshots" {
		t.Fatalf("expected output path to be preserved, got %q", cfg.OutputPath)
	}
}

func TestParseConfigGroupedModelCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cfg, err := parseConfig([]string{cmdModel, cmdKoch, "--iterations", "2"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}

	if cfg.Command != cmdKoch {
		t.Fatalf("expected command %q, got %q", cmdKoch, cfg.Command)
	}
	if cfg.Iterations != 2 {
		t.Fatalf("expected iterations 2, got %d", cfg.Iterations)
	}
}

func TestParseConfigRefreshFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cfg, err := parseConfig([]string{cmdReal, cmdCoastline, "--refresh"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}

	if !cfg.Refresh {
		t.Fatal("expected refresh flag to be true")
	}
}

func TestParseConfigSupportsLegacyAlias(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cfg, err := parseConfig([]string{cmdDimension, "--iterations", "1"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}

	if cfg.Command != cmdDimension {
		t.Fatalf("expected command %q, got %q", cmdDimension, cfg.Command)
	}
}

func TestParseConfigShowsGroupHelpWithoutSubcommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	_, err := parseConfig([]string{cmdReal}, &stdout, &stderr)
	if err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	if !strings.Contains(stdout.String(), "Использование:") {
		t.Fatalf("expected help output, got %q", stdout.String())
	}
}

func TestParseConfigRejectsWrongGroupCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	_, err := parseConfig([]string{cmdReal, cmdKoch}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown real command") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseConfigErosionWaveFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cfg, err := parseConfig([]string{
		cmdModel,
		cmdErosion,
		"--steps", "3",
		"--erosion-strength", "75",
		"--wave-direction", "45",
		"--wind-speed", "14",
		"--fetch-spread", "60",
		"--fetch-samples", "11",
		"--max-fetch-km", "220",
		"--depth-scale", "5000",
		"--exposure-power", "2",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}

	if cfg.Command != cmdErosion {
		t.Fatalf("expected command %q, got %q", cmdErosion, cfg.Command)
	}
	if cfg.WaveDirection != 45 || cfg.WindSpeed != 14 {
		t.Fatalf("expected wave settings to be parsed, got direction=%.1f wind=%.1f", cfg.WaveDirection, cfg.WindSpeed)
	}
	if cfg.FetchSamples != 11 || cfg.MaxFetchKM != 220 {
		t.Fatalf("expected fetch settings to be parsed, got samples=%d maxFetch=%.1f", cfg.FetchSamples, cfg.MaxFetchKM)
	}
	if cfg.DepthScale != 5000 || cfg.ExposurePower != 2 {
		t.Fatalf("expected depth/exposure settings to be parsed, got depth=%.1f exposure=%.1f", cfg.DepthScale, cfg.ExposurePower)
	}
}
