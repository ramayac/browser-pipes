package main

import (
	"os"
	"testing"
)

func TestExecuteWorkflowV2(t *testing.T) {
	cfg := &Config{
		Version: "2",
		Commands: map[string]Command{
			"test_cmd": {
				Parameters: map[string]Parameter{},
				Steps: []Step{
					{Name: "run", Args: "echo 'hello from cmd' > output.txt"},
				},
			},
		},
		Jobs: map[string]Job{
			"simple_job": {
				Steps: []Step{
					{Name: "run", Args: "echo 'step 1' > file1.txt"},
					{Name: "run", Args: "cat file1.txt"},
				},
			},
			"param_job": {
				Steps: []Step{
					{Name: "run", Args: "echo <<parameters.input_val>> > output.txt"},
				},
			},
			"capture_job": {
				Steps: []Step{
					{Name: "run", Params: map[string]string{"command": "echo 'captured_value'", "save_to": "my_result"}},
					{Name: "run", Args: "echo <<parameters.my_result>> > final.txt"},
				},
			},
		},
		Workflows: map[string]Workflow{
			"main": {
				Jobs: []WorkflowJob{
					{Name: "simple_job", Match: ".*example.com.*"},
					{Name: "param_job", Match: ".*params.com.*", Params: map[string]string{"input_val": "hello_params"}},
				},
			},
		},
	}

	t.Run("Success: Workflow Match", func(t *testing.T) {
		err := ExecuteWorkflowV2(cfg, "https://example.com", "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("Error: No Workflow Match", func(t *testing.T) {
		err := ExecuteWorkflowV2(cfg, "https://nomatch.com", "")
		if err == nil {
			t.Fatal("expected error for no matching jobs, got nil")
		}
	})

	t.Run("Success: Parameter Injection", func(t *testing.T) {
		err := ExecuteWorkflowV2(cfg, "https://params.com", "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		// We don't easily check the output here without more complex mocking,
		// but we verify execution completed.
	})
}

func TestParameterResolution(t *testing.T) {
	params := map[string]string{
		"foo": "bar",
		"url": "http://test.com",
	}

	input := "echo <<parameters.foo>> at <<parameters.url>>"
	expected := "echo bar at http://test.com"
	actual := resolveParams(input, params)

	if actual != expected {
		t.Errorf("expected %q, got %q", expected, actual)
	}

	// Test no spaces
	input2 := "<<parameters.foo>>"
	expected2 := "bar"
	actual2 := resolveParams(input2, params)
	if actual2 != expected2 {
		t.Errorf("expected %q, got %q", expected2, actual2)
	}
}

func TestExecuteJob_Workspace(t *testing.T) {
	// Verify that files share the same workspace across steps
	cfg := &Config{}
	job := Job{
		Steps: []Step{
			{Name: "run", Args: "echo 'cross-step-data' > shared.txt"},
			{Name: "run", Args: "grep 'cross-step-data' shared.txt"},
		},
	}

	err := executeJob(cfg, job, nil, "http://test.com", "")
	if err != nil {
		t.Errorf("expected success in workspace sharing test, got %v", err)
	}
}

func TestExecuteStep_SaveTo(t *testing.T) {
	cfg := &Config{}
	scopeParams := make(map[string]string)

	// Step 1: Save output
	step1 := Step{
		Name: "run",
		Params: map[string]string{
			"command": "echo 'important_data'",
			"save_to": "captured",
		},
	}

	tmpDir, _ := os.MkdirTemp("", "plumber-test-*")
	defer os.RemoveAll(tmpDir)

	err := executeStep(cfg, step1, scopeParams, "http://test.com", "", tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if scopeParams["captured"] != "important_data" {
		t.Errorf("expected 'important_data' in scopeParams, got %q", scopeParams["captured"])
	}

	// Step 2: Use saved output
	step2 := Step{
		Name: "run",
		Args: "echo <<parameters.captured>>",
	}
	err = executeStep(cfg, step2, scopeParams, "http://test.com", "", tmpDir)
	if err != nil {
		t.Errorf("expected success using captured param, got %v", err)
	}
}

func TestExecuteStep_HTML(t *testing.T) {
	cfg := &Config{}
	htmlContent := "<html><body>Test</body></html>"

	// Create a script that checks if the file provided by {html} exists and contains the content
	step := Step{
		Name: "run",
		Args: "cat {html} | grep 'Test'",
	}

	tmpDir, _ := os.MkdirTemp("", "plumber-test-*")
	defer os.RemoveAll(tmpDir)

	err := executeStep(cfg, step, nil, "http://test.com", htmlContent, tmpDir)
	if err != nil {
		t.Errorf("expected success and match in HTML substitution, got %v", err)
	}
}

func TestInjectSystemParams(t *testing.T) {
	params := map[string]string{"user": "alice"}
	url := "http://example.com"

	res := injectSystemParams(params, url)

	if res["user"] != "alice" {
		t.Error("lost user param")
	}
	if res["url"] != url {
		t.Error("missing url")
	}
	if res["url_hash"] == "" {
		t.Error("missing url_hash")
	}
}
