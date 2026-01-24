package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// ExecuteWorkflowV2 finds the matching job in the workflow and executes it.
func ExecuteWorkflowV2(cfg *Config, url string) error {
	// 1. Iterate over workflows (Currently assuming single active workflow or checking all)
	// CircleCI usually runs all workflows that match triggers.
	// For Plumber, we likely want the first match or all matches?
	// Given "browser-pipes", let's assume we check all workflows.

	matched := false
	for wfName, wf := range cfg.Workflows {
		log.Printf("🔍 Checking workflow: %s", wfName)
		for _, jobRef := range wf.Jobs {
			// jobRef.Match contains the regex.
			// If match is empty, treat as "match all" or fallback?
			// User example has:
			// - my-job:
			//     filters: ...

			// But user also said: "Instead of branches we can have the regex for matching a target"
			// And showed:
			// jobs:
			//   - my-job
			// (Implying simplest case)

			// Let's assume jobRef.Match is the regex.
			// If empty, does it match? Maybe yes, if it's the only job?
			// Or maybe we strictly require match?
			// Let's assume empty match = catch-all if explicitly defined as such, generally regex should be provided.
			// Actually, in the user design prompt: "And instead of branches we can have the regex for matching a target (job or command)."

			isMatch := matches(jobRef.Match, url)
			if jobRef.Match == "" {
				// matches() returns false for empty pattern.
				// Should we treat empty match as false? Or true?
				// If no match rule, maybe it always runs?
				// CAUTION: If always runs, we might loop.
				// Let's assume empty regex = match everything (fallback)
				isMatch = true
			}

			if isMatch {
				log.Printf("   ✅ Matched Job Ref: %s (Regex: '%s')", jobRef.Name, jobRef.Match)

				// Find the actual job definition
				jobDef, ok := cfg.Jobs[jobRef.Name]
				if !ok {
					log.Printf("   ❌ Job definition not found: %s", jobRef.Name)
					continue
				}

				// Execute Job
				if err := executeJob(cfg, jobDef, jobRef.Params, url); err != nil {
					log.Printf("   ❌ Job matched but failed: %v", err)
					// Verify Next? Or stop?
					// CircleCI stops on failure usually.
					return err
				}
				matched = true
				// Should we break after one match per workflow? Or execute all matches?
				// "Pipes" -> maybe multiple?
				// But "Plumber" usually routes to ONE destination.
				// Let's assume FIRST match wins per workflow for now, or maybe all matches run.
				// For safety, let's run ALL matches across workflows, but within a workflow?
				// Users might define chain?
				// Let's assume independent checks.
			}
		}
	}

	if !matched {
		return fmt.Errorf("no matching jobs found for url: %s", url)
	}
	return nil
}

func executeJob(cfg *Config, job Job, params map[string]string, url string) error {
	for _, step := range job.Steps {
		if err := executeStep(cfg, step, params, url); err != nil {
			return err
		}
	}
	return nil
}

func executeCommand(cfg *Config, cmdName string, cmdDef Command, callParams map[string]string, url string) error {
	// 1. Resolve Parameters
	// Merge callParams with defaults
	finalParams := make(map[string]string)

	// Apply defaults
	for pName, pDef := range cmdDef.Parameters {
		finalParams[pName] = pDef.Default
	}

	// Override with called params
	for k, v := range callParams {
		finalParams[k] = v
	}

	// 2. Execute Steps
	for _, step := range cmdDef.Steps {
		if err := executeStep(cfg, step, finalParams, url); err != nil {
			return err
		}
	}
	return nil
}

func executeStep(cfg *Config, step Step, scopeParams map[string]string, url string) error {
	// Case 1: "run" command
	if step.Name == "run" {
		// The script is in step.Args
		script := step.Args

		// Substitute parameters
		// 1. Resolve << parameters.x >>
		script = resolveParams(script, scopeParams)
		// 2. Resolve {url} (legacy/convenience)
		script = strings.ReplaceAll(script, "{url}", url)

		// Execute
		log.Printf("   🏃 Running: %s", script)
		// Use sh -c for complex commands
		cmd := exec.Command("sh", "-c", script)
		cmd.Env = os.Environ() // Pass env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("run step failed: %w", err)
		}
		return nil
	}

	// Case 2: Reference to another command
	cmdDef, ok := cfg.Commands[step.Name]
	if ok {
		// Resolve parameters for this call
		// The params passed to THIS step call need to be resolved against the CURRENT scope
		// e.g. - open_browser: { browser: "<< parameters.browser >>" }
		resolvedCallParams := make(map[string]string)
		for k, v := range step.Params {
			resolvedCallParams[k] = resolveParams(v, scopeParams)
		}

		return executeCommand(cfg, step.Name, cmdDef, resolvedCallParams, url)
	}

	return fmt.Errorf("unknown command or step: %s", step.Name)
}

// resolveParams replaces instances of << parameters.key >> or <<parameters.key>> with values
func resolveParams(input string, params map[string]string) string {
	// We can use a simple replace loop or regex.
	// Valid formats:
	// << parameters.key >>
	// <<parameters.key>>

	result := input
	for k, v := range params {
		// Replace variations
		result = strings.ReplaceAll(result, fmt.Sprintf("<< parameters.%s >>", k), v)
		result = strings.ReplaceAll(result, fmt.Sprintf("<<parameters.%s>>", k), v)
	}
	return result
}
