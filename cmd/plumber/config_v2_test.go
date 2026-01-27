package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfigValidation(t *testing.T) {
	t.Run("Success: Valid Config", func(t *testing.T) {
		yamlData := `
version: "2"
commands:
  say_hello:
    parameters:
      name:
        type: string
        default: "world"
    steps:
      - run: "echo hello <<parameters.name>>"
jobs:
  my_job:
    steps:
      - say_hello:
          name: "human"
workflows:
  main:
    jobs:
      - my_job:
          match: ".*example.com.*"
`
		var cfg Config
		err := yaml.Unmarshal([]byte(yamlData), &cfg)
		if err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if err := cfg.Validate(); err != nil {
			t.Errorf("expected valid config, got error: %v", err)
		}
	})

	t.Run("Error: Undefined Job", func(t *testing.T) {
		yamlData := `
version: "2"
workflows:
  main:
    jobs:
      - non_existent_job
`
		var cfg Config
		err := yaml.Unmarshal([]byte(yamlData), &cfg)
		if err != nil {
			t.Fatal(err)
		}

		err = cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "references undefined job") {
			t.Errorf("expected undefined job error, got %v", err)
		}
	})

	t.Run("Error: Undefined Command", func(t *testing.T) {
		yamlData := `
version: "2"
jobs:
  my_job:
    steps:
      - undefined_command
`
		var cfg Config
		err := yaml.Unmarshal([]byte(yamlData), &cfg)
		if err != nil {
			t.Fatal(err)
		}

		err = cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "references undefined command") {
			t.Errorf("expected undefined command error, got %v", err)
		}
	})

	t.Run("Error: Invalid Regex", func(t *testing.T) {
		yamlData := `
version: "2"
jobs:
  my_job:
    steps:
      - run: "id"
workflows:
  main:
    jobs:
      - my_job:
          match: "[invalid regex"
`
		var cfg Config
		err := yaml.Unmarshal([]byte(yamlData), &cfg)
		if err != nil {
			t.Fatal(err)
		}

		err = cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "invalid match regex") {
			t.Errorf("expected invalid regex error, got %v", err)
		}
	})
}

func TestStepUnmarshaling(t *testing.T) {
	t.Run("Simple Run Step", func(t *testing.T) {
		yamlData := "- run: 'echo hi'"
		var steps []Step
		err := yaml.Unmarshal([]byte(yamlData), &steps)
		if err != nil {
			t.Fatal(err)
		}
		if len(steps) != 1 || steps[0].Name != "run" || steps[0].Args != "echo hi" {
			t.Errorf("unexpected step: %+v", steps[0])
		}
	})

	t.Run("Command Step with Params", func(t *testing.T) {
		yamlData := `
- my_cmd:
    param1: val1
    param2: val2
`
		var steps []Step
		err := yaml.Unmarshal([]byte(yamlData), &steps)
		if err != nil {
			t.Fatal(err)
		}
		if len(steps) != 1 || steps[0].Name != "my_cmd" || steps[0].Params["param1"] != "val1" {
			t.Errorf("unexpected step: %+v", steps[0])
		}
	})

	t.Run("Error: Malformed Step", func(t *testing.T) {
		yamlData := `
- run: "hi"
  extra: "invalid"
`
		var steps []Step
		err := yaml.Unmarshal([]byte(yamlData), &steps)
		if err == nil {
			t.Error("expected error for multi-key step mapping, got nil")
		}
	})
}

func TestMatches(t *testing.T) {
	if !matches(".*google.*", "https://google.com") {
		t.Error("expected match")
	}
	if matches(".*google.*", "https://bing.com") {
		t.Error("expected no match")
	}
	if matches("", "anything") {
		t.Error("empty pattern should not match anything by default in matches function")
	}
}
