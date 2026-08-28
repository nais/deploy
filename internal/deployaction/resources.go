package deployaction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aymerick/raymond"
	yamlv2 "gopkg.in/yaml.v2"
	"sigs.k8s.io/yaml"
)

type templateVariables map[string]any

func multiDocumentFileAsJSON(path string, ctx templateVariables) ([]json.RawMessage, error) {
	fileContents, err := os.ReadFile(path) // #nosec G304 -- user-supplied deployment resource path, intentional
	if err != nil {
		return nil, fmt.Errorf("%s: open file: %s", path, err)
	}

	templated, err := templatedFile(fileContents, ctx)
	if err != nil {
		errMsg := strings.ReplaceAll(err.Error(), "\n", ": ")
		return nil, fmt.Errorf("%s: %s", path, errMsg)
	}

	var content any
	messages := make([]json.RawMessage, 0)
	decoder := yamlv2.NewDecoder(bytes.NewReader(templated))
	for {
		err = decoder.Decode(&content)
		if err == io.EOF {
			return messages, nil
		}
		if err != nil {
			return nil, err
		}

		rawDocument, err := yamlv2.Marshal(content)
		if err != nil {
			return nil, err
		}
		data, err := yaml.YAMLToJSON(rawDocument)
		if err != nil {
			errMsg := strings.ReplaceAll(err.Error(), "\n", ": ")
			return nil, fmt.Errorf("%s: %s", path, errMsg)
		}
		messages = append(messages, data)
	}
}

func templatedFile(data []byte, ctx templateVariables) ([]byte, error) {
	if len(ctx) == 0 {
		return data, nil
	}
	template, err := raymond.Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse template file: %s", err)
	}
	output, err := template.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute template: %s", err)
	}
	return []byte(output), nil
}

func templateVariablesFromFile(path string) (templateVariables, error) {
	file, err := os.ReadFile(path) // #nosec G304 -- user-supplied vars file path, intentional
	if err != nil {
		return nil, fmt.Errorf("%s: open file: %s", path, err)
	}
	vars := templateVariables{}
	err = yaml.Unmarshal(file, &vars)
	if vars == nil {
		vars = templateVariables{}
	}
	return vars, err
}

func templateVariablesFromSlice(vars []string) templateVariables {
	result := templateVariables{}
	for _, keyValue := range vars {
		tokens := strings.SplitN(keyValue, "=", 2)
		switch len(tokens) {
		case 2:
			result[tokens[0]] = tokens[1]
		case 1:
			result[tokens[0]] = true
		}
	}
	return result
}

func detectTeamFromResource(resource json.RawMessage) string {
	var value struct {
		Metadata struct {
			Labels struct {
				Team string `json:"team"`
			} `json:"labels"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(resource, &value); err != nil {
		return ""
	}
	return value.Metadata.Labels.Team
}

func detectNamespace(resource json.RawMessage) string {
	var value struct {
		Metadata struct {
			Namespace string `json:"namespace"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(resource, &value); err != nil {
		return ""
	}
	return value.Metadata.Namespace
}
