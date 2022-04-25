// Package runner is responsible for running the command and rendering the output
package runner

import (
	"encoding/json"
	"io"
	"os"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/fatih/color"
	"github.com/psampaz/go-mod-outdated/internal/mod"

	"github.com/olekukonko/tablewriter"
)

// OsExit is use here in order to simplify testing
var OsExit = os.Exit

// OutputStyle specifies the supported table rendering formats
type OutputStyle string

const (
	// StyleDefault represents the default output style
	StyleDefault OutputStyle = "default"
	// StyleMarkdown represents the markdown formatted output style
	StyleMarkdown OutputStyle = "markdown"
	// StyleJSON represents the JSON formatted output style
	StyleJSON OutputStyle = "json"
	// StylePretty represents the pretty list formatted output style
	StylePretty OutputStyle = "pretty"
)

// Run converts the the json output of go list -u -m -json all to table format
func Run(in io.Reader, out io.Writer, update, direct, exitWithNonZero bool, style OutputStyle) error {
	var modules []mod.Module

	dec := json.NewDecoder(in)

	for {
		var v mod.Module
		err := dec.Decode(&v)

		if err != nil {
			if err == io.EOF {
				filteredModules := mod.FilterModules(modules, update, direct)
				if len(filteredModules) > 0 {
					renderOutput(out, filteredModules, style)
				}

				if hasOutdated(filteredModules) && exitWithNonZero {
					OsExit(1)
				}

				return nil
			}

			return err
		}

		modules = append(modules, v)
	}
}

func hasOutdated(filteredModules []mod.Module) bool {
	for m := range filteredModules {
		if filteredModules[m].HasUpdate() {
			return true
		}
	}

	return false
}

func renderOutput(writer io.Writer, modules []mod.Module, style OutputStyle) {
	switch style {
	case StyleJSON:
		renderJSON(writer, modules)
	case StylePretty:
		renderPretty(writer, modules)
	default:
		renderTable(writer, modules, style)
	}
}

func renderTable(writer io.Writer, modules []mod.Module, style OutputStyle) {
	table := tablewriter.NewWriter(writer)
	table.SetHeader([]string{"Module", "Version", "New Version", "Direct", "Valid Timestamps"})

	// Render table as markdown
	if style == StyleMarkdown {
		table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
		table.SetCenterSeparator("|")
	}

	for k := range modules {
		table.Append([]string{
			modules[k].Path,
			modules[k].CurrentVersion(),
			modules[k].NewVersion(),
			strconv.FormatBool(!modules[k].Indirect),
			strconv.FormatBool(!modules[k].InvalidTimestamp()),
		})
	}

	table.Render()
}

func renderJSON(writer io.Writer, modules []mod.Module) {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	encoder.Encode(modules)
}

func renderPretty(writer io.Writer, modules []mod.Module) {
	table := tablewriter.NewWriter(writer)
	table.SetBorder(false)
	table.SetColumnSeparator("")
	table.SetAutoWrapText(false)
	table.SetNoWhiteSpace(true)

	for _, mod := range modules {
		var (
			c          color.Attribute
			newVersion string
		)

		if mod.CurrentVersion() == mod.NewVersion() || mod.NewVersion() == "" {
			c = color.FgGreen
		} else {
			curParsed := semver.MustParse(mod.CurrentVersion())
			newParsed := semver.MustParse(mod.NewVersion())

			if curParsed.Major() == newParsed.Major() {
				c = color.FgYellow
			} else {
				c = color.FgRed
			}

			newVersion = renderVersionDiff(curParsed, newParsed)
		}

		row := []string{
			color.New(c).Sprintf("%s ", mod.Path),
			color.New(color.FgBlue).Sprintf("%s ", mod.CurrentVersion()),
			newVersion,
		}

		table.Append(row)
	}

	table.Render()
}

func renderVersionDiff(cur *semver.Version, next *semver.Version) string {
	output := "-> "
	c := color.FgWhite

	if cur.Major() != next.Major() {
		c = color.FgYellow
	}
	output += color.New(c).Sprintf("%d.", next.Major())

	if cur.Minor() != next.Minor() {
		c = color.FgYellow
	}
	output += color.New(c).Sprintf("%d.", next.Minor())

	if cur.Patch() != next.Patch() {
		c = color.FgYellow
	}
	output += color.New(c).Sprintf("%d", next.Patch())

	if next.Prerelease() != "" {
		output += color.New(c).Sprint("-")

		if cur.Prerelease() != next.Prerelease() {
			c = color.FgYellow
		}
		output += color.New(c).Sprint(next.Prerelease())
	}

	return output
}
