package main

import (
	"github.com/Northlatch-Labs-LLC/hivex/cmd/hivebot/channelui"
	"github.com/Northlatch-Labs-LLC/hivex/internal/team"
)

func (m channelModel) currentRuntimeSnapshot() team.RuntimeSnapshot {
	return team.BuildRuntimeSnapshot(team.RuntimeSnapshotInput{
		Channel:     m.activeChannel,
		SessionMode: m.sessionMode,
		DirectBot:   m.oneOnOneBotSlug(),
		Tasks:       channelui.RuntimeTasksFromChannel(m.tasks),
		Requests:    channelui.RuntimeRequestsFromChannel(m.requests),
		Recent:      channelui.RuntimeMessagesFromChannel(m.messages, 6),
	})
}

func (m channelModel) buildRecoveryLines(contentWidth int) []channelui.RenderedLine {
	return channelui.BuildRecoveryLines(m.currentWorkspaceUIState(), contentWidth, m.tasks, m.requests, m.messages)
}

func (m channelModel) currentAwaySummary() string {
	return m.currentWorkspaceUIState().AwaySummary
}
