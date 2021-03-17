package deployclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/aymerick/raymond"
	"github.com/ghodss/yaml"
	yamlv2 "gopkg.in/yaml.v2"
)

func MultiDocumentFileAsJSON(path string, ctx TemplateVariables) ([]json.RawMessage, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: open file: %s", path, err)
	}

	templated, err := templatedFile(file, ctx)
	if err != nil {
		errMsg := strings.ReplaceAll(err.Error(), "\n", ": ")
		return nil, fmt.Errorf("%s: %s", path, errMsg)
	}

	var content interface{}
	messages := make([]json.RawMessage, 0)

	decoder := yamlv2.NewDecoder(bytes.NewReader(templated))
	for {
		err = decoder.Decode(&content)
		if err == io.EOF {
			err = nil
			break
		} else if err != nil {
			return nil, err
		}

		rawdocument, err := yamlv2.Marshal(content)
		if err != nil {
			return nil, err
		}

		data, err := yaml.YAMLToJSON(rawdocument)
		if err != nil {
			errMsg := strings.ReplaceAll(err.Error(), "\n", ": ")
			return nil, fmt.Errorf("%s: %s", path, errMsg)
		}

		messages = append(messages, data)
	}

	return messages, err
}

func detectTeam(resource json.RawMessage) string {
	type teamMeta struct {
		Metadata struct {
			Labels struct {
				Team string `json:"team"`
			} `json:"labels"`
		} `json:"metadata"`
	}
	buf := &teamMeta{}
	err := json.Unmarshal(resource, buf)

	if err != nil {
		return ""
	}

	return buf.Metadata.Labels.Team
}

func detectNamespace(resource json.RawMessage) string {
	type namespaceMeta struct {
		Metadata struct {
			Namespace string `json:"namespace"`
		} `json:"metadata"`
	}
	buf := &namespaceMeta{}
	err := json.Unmarshal(resource, buf)

	if err != nil {
		return ""
	}

	return buf.Metadata.Namespace
}

// Wrap JSON resources in a JSON array.
func wrapResources(resources []json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(resources)
}

func templatedFile(data []byte, ctx TemplateVariables) ([]byte, error) {
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

func templateVariablesFromFile(path string) (TemplateVariables, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: open file: %s", path, err)
	}

	vars := TemplateVariables{}
	err = yaml.Unmarshal(file, &vars)

	return vars, err
}

func templateVariablesFromSlice(vars []string) TemplateVariables {
	tv := TemplateVariables{}
	for _, keyval := range vars {
		tokens := strings.SplitN(keyval, "=", 2)
		switch len(tokens) {
		case 2: // KEY=VAL
			tv[tokens[0]] = tokens[1]
		case 1: // KEY
			tv[tokens[0]] = true
		default:
			continue
		}
	}

	return tv
}

func detectErrorLine(e string) (int, error) {
	var line int
	_, err := fmt.Sscanf(e, "yaml: line %d:", &line)
	return line, err
}

func errorContext(content string, line int) []string {
	ctx := make([]string, 0)
	lines := strings.Split(content, "\n")
	format := "%03d: %s"
	for l := range lines {
		ctx = append(ctx, fmt.Sprintf(format, l+1, lines[l]))
		if l+1 == line {
			helper := "     " + strings.Repeat("^", len(lines[l])) + " <--- error near this line"
			ctx = append(ctx, helper)
		}
	}
	return ctx
}
