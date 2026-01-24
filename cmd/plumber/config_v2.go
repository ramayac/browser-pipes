package main

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"gopkg.in/yaml.v3"
)

// Config represents the new CircleCI-inspired configuration (V2).
type Config struct {
	Version   string              `yaml:"version" json:"version" jsonschema:"enum=2,description=Configuration version must be '2'"`
	Commands  map[string]Command  `yaml:"commands" json:"commands" jsonschema:"description=Reusable command definitions"`
	Jobs      map[string]Job      `yaml:"jobs" json:"jobs" jsonschema:"description=Job definitions"`
	Workflows map[string]Workflow `yaml:"workflows" json:"workflows" jsonschema:"description=Workflow definitions mapping jobs to URL patterns"`
}

// Validate checks the configuration for consistency.
func (c *Config) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("version is missing")
	}

	// 1. Validate Workflows
	for wfName, wf := range c.Workflows {
		for _, jobRef := range wf.Jobs {
			// Check if job exists
			if _, ok := c.Jobs[jobRef.Name]; !ok {
				return fmt.Errorf("workflow '%s' references undefined job '%s'", wfName, jobRef.Name)
			}
			// Validate Match Regex
			if jobRef.Match != "" {
				if _, err := regexp.Compile(jobRef.Match); err != nil {
					return fmt.Errorf("workflow '%s' job '%s' has invalid match regex '%s': %v", wfName, jobRef.Name, jobRef.Match, err)
				}
			}
		}
	}

	// 2. Validate Jobs
	for jobName, job := range c.Jobs {
		for i, step := range job.Steps {
			if step.Name == "run" {
				continue
			}
			// Check if command exists
			cmd, ok := c.Commands[step.Name]
			if !ok {
				return fmt.Errorf("job '%s' step %d references undefined command '%s'", jobName, i+1, step.Name)
			}
			// Check params (optional, could be stricter)
			for paramName := range step.Params {
				if _, ok := cmd.Parameters[paramName]; !ok {
					// Is this an error? Or just extra param? CircleCI errors on unknown params.
					return fmt.Errorf("job '%s' step %d passes unknown parameter '%s' to command '%s'", jobName, i+1, paramName, step.Name)
				}
			}
		}
	}

	return nil
}

// GenerateJSONSchema returns a JSON Schema as a string describing the configuration.
func GenerateJSONSchema() string {
	r := new(jsonschema.Reflector)
	r.ExpandedStruct = true // Expand structs for better readability in schema if needed
	if err := r.AddGoComments("github.com/browser-pipes/plumber", "./cmd/plumber"); err != nil {
		// Ignore error if comment parsing fails (not critical)
	}

	schema := r.Reflect(&Config{})

	// Pretty print
	bytes, _ := json.MarshalIndent(schema, "", "  ")
	return string(bytes)
}

type Command struct {
	Parameters map[string]Parameter `yaml:"parameters" json:"parameters,omitempty"`
	Steps      []Step               `yaml:"steps" json:"steps"`
}

type Parameter struct {
	Type    string `yaml:"type" json:"type" jsonschema:"enum=string"`
	Default string `yaml:"default" json:"default"`
}

type Job struct {
	Steps []Step `yaml:"steps" json:"steps"`
}

type Workflow struct {
	Jobs []WorkflowJob `yaml:"jobs" json:"jobs"`
}

type WorkflowJob struct {
	Name   string            `yaml:"-" json:"-"` // The key in the list or map
	Match  string            `yaml:"match" json:"match,omitempty" jsonschema:"format=regex"`
	Params map[string]string `yaml:",inline" json:"params,omitempty"`
}

// JSONSchema implements the jsonschema.JSONSchemaer interface for WorkflowJob
// to describe its polymorphic nature (String or Object).
func (WorkflowJob) JSONSchema() *jsonschema.Schema {
	props := orderedmap.New[string, *jsonschema.Schema]()
	props.Set("match", &jsonschema.Schema{
		Type:        "string",
		Format:      "regex",
		Description: "Regex pattern to match URLs",
	})

	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{
				Type:        "string",
				Description: "Job name reference",
			},
			{
				Type:        "object",
				Description: "Job reference with configuration",
				Properties:  props,
				// We enforce string keys:
				AdditionalProperties: &jsonschema.Schema{
					Type: "string",
				},
				// Ensure min properties to disambiguate? No, user might just conform to struct.
			},
		},
	}
}

type Step struct {
	Name   string            `json:"-"`
	Args   string            `json:"-"`
	Params map[string]string `json:"-"`
}

// JSONSchema implements the jsonschema.JSONSchemaer interface for Step.
func (Step) JSONSchema() *jsonschema.Schema {
	minProps := uint64(1)
	maxProps := uint64(1)

	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{
				Type:        "string",
				Description: "Command name (e.g. 'checkout')",
			},
			{
				Type:          "object",
				Description:   "Command with parameters (e.g. 'run: ...' or 'my_command: ...')",
				MinProperties: &minProps,
				MaxProperties: &maxProps,
				AdditionalProperties: &jsonschema.Schema{
					OneOf: []*jsonschema.Schema{
						{
							Type:        "string",
							Description: "For 'run' command, the script to execute",
						},
						{
							Type:        "object",
							Description: "Parameters for the command",
							AdditionalProperties: &jsonschema.Schema{
								Type: "string",
							},
						},
					},
				},
			},
		},
	}
}

// UnmarshalYAML implements custom unmarshalling for Step to handle
// string ("command_name") vs map ({"command_name": params}).
func (s *Step) UnmarshalYAML(value *yaml.Node) error {
	// Case 1: Step is just a string (e.g. "- checkout")
	if value.Kind == yaml.ScalarNode {
		s.Name = value.Value
		return nil
	}

	// Case 2: Step is a map (e.g. "- greeting: ...")
	if value.Kind == yaml.MappingNode {
		if len(value.Content) != 2 {
			return fmt.Errorf("step map must have exactly one key (the command name)")
		}
		keyNode := value.Content[0]
		valNode := value.Content[1]

		s.Name = keyNode.Value

		// If the value is a scalar, it depends on the command.
		// For "run", it's the script.
		if valNode.Kind == yaml.ScalarNode {
			if s.Name == "run" {
				s.Args = valNode.Value
			} else {
				return fmt.Errorf("unexpected string argument for command '%s' (only 'run' supports this)", s.Name)
			}
			return nil
		}

		// If value is a map, these are parameters
		if valNode.Kind == yaml.MappingNode {
			s.Params = make(map[string]string)
			// Decode the map into s.Params
			if err := valNode.Decode(&s.Params); err != nil {
				return fmt.Errorf("failed to decode parameters for command '%s': %v", s.Name, err)
			}
			return nil
		}
	}

	return fmt.Errorf("invalid step format")
}

// UnmarshalYAML for WorkflowJob to handle list of maps where key is name and value is details
func (wj *WorkflowJob) UnmarshalYAML(value *yaml.Node) error {
	// A job in a workflow is typically a map item:
	// - my_job:
	//     match: "..."
	// Or just a string:
	// - my_job

	if value.Kind == yaml.ScalarNode {
		wj.Name = value.Value
		return nil
	}

	if value.Kind == yaml.MappingNode {
		if len(value.Content) != 2 {
			return fmt.Errorf("workflow job must have single key")
		}
		wj.Name = value.Content[0].Value

		// Decode the rest into the struct fields (Match, Params)
		// We can't decode directly into wj because we're inside UnmarshalYAML for wj.
		// We'll decode into an alias to avoid recursion/inline issues or just a map.
		type alias WorkflowJob
		var tmp alias
		if err := value.Content[1].Decode(&tmp); err != nil {
			return err
		}
		wj.Match = tmp.Match
		wj.Params = tmp.Params
		return nil
	}

	return fmt.Errorf("invalid workflow job format")
}

// Helper to check if a regular expression matches the input string
func matches(pattern, input string) bool {
	if pattern == "" {
		return false
	}
	matched, err := regexp.MatchString(pattern, input)
	if err != nil {
		return false
	}
	return matched
}
