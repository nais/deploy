// Package githubactions provides small helpers for emitting GitHub Actions
// workflow commands (log groups, annotations) and writing to the job step
// summary, without pulling in an external dependency.
//
// See https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions
package githubactions

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Group starts a collapsible log group with the given title and returns a
// function that ends the group. Typical usage:
//
//	end := githubactions.Group("Template resources")
//	defer end()
func Group(title string, a ...any) func() {
	fmt.Fprintf(os.Stdout, "::group::%s\n", fmt.Sprintf(title, a...))
	return func() {
		fmt.Fprintln(os.Stdout, "::endgroup::")
	}
}

// Error emits an error annotation.
func Error(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "::error::%s\n", escape(fmt.Sprintf(format, a...)))
}

// Warning emits a warning annotation.
func Warning(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "::warning::%s\n", escape(fmt.Sprintf(format, a...)))
}

// Notice emits a notice annotation.
func Notice(format string, a ...any) {
	fmt.Fprintf(os.Stdout, "::notice::%s\n", escape(fmt.Sprintf(format, a...)))
}

// escape replaces characters that have special meaning in workflow command
// annotations, per GitHub's documented escaping rules.
func escape(s string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		"\r", "%0D",
		"\n", "%0A",
	)
	return replacer.Replace(s)
}

// Summary appends markdown content to the job step summary, if
// GITHUB_STEP_SUMMARY is set in the environment. It is a no-op otherwise
// (e.g. when running outside of GitHub Actions).
func Summary(format string, a ...any) {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if len(path) == 0 {
		return
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644) // #nosec G304 G302 -- path from GitHub Actions environment
	if err != nil {
		return
	}
	defer f.Close()

	writeSummary(f, format, a...)
}

func writeSummary(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, format+"\n", a...)
}
