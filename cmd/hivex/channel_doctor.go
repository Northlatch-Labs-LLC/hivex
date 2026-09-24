package main

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Northlatch-Labs-LLC/hivex/cmd/hivex/channelui"
	"github.com/Northlatch-Labs-LLC/hivex/internal/team"
)

type channelDoctorDoneMsg struct {
	report channelui.DoctorReport
	err    error
}

var detectRuntimeCapabilitiesFn = func(opts team.CapabilityProbeOptions) team.RuntimeCapabilities {
	return team.DetectRuntimeCapabilitiesWithOptions(opts)
}

func runDoctorChecks() tea.Cmd {
	return func() tea.Msg {
		report, err := inspectDoctor()
		return channelDoctorDoneMsg{report: report, err: err}
	}
}

func inspectDoctor() (channelui.DoctorReport, error) {
	capabilities := detectRuntimeCapabilitiesFn(team.CapabilityProbeOptions{
		IncludeConnections: true,
		ConnectionLimit:    5,
		ConnectionTimeout:  5 * time.Second,
	})
	report := channelui.DoctorReport{
		GeneratedAt: time.Now(),
		Registry:    capabilities.Registry,
	}
	for _, entry := range capabilities.Registry.Entries {
		report.Checks = append(report.Checks, channelui.DoctorCheck{
			Label:     entry.Label,
			Severity:  channelui.DoctorSeverityForCapability(entry),
			Lifecycle: entry.Lifecycle,
			Detail:    entry.Detail,
			NextStep:  entry.NextStep,
		})
	}
	return report, nil
}
