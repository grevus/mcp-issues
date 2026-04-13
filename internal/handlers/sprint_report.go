package handlers

import (
	"context"
	"fmt"
	"log"

	"github.com/grevus/mcp-jira/internal/tracker"
)

// SprintReporter — узкий интерфейс для sprint_health_report.
type SprintReporter interface {
	GetSprintReport(ctx context.Context, boardID, sprintID int) (tracker.SprintReport, error)
}

// ScopeReader — узкий интерфейс для чтения scope changes спринта через changelog.
// *jira.HTTPClient реализует его через GetSprintScopeChanges.
type ScopeReader interface {
	GetSprintScopeChanges(ctx context.Context, sprintID int) (added, removed []string, err error)
}

// SprintHealthReportInput — параметры MCP tool sprint_health_report.
// SprintID опционален: если 0 — используется активный спринт доски.
type SprintHealthReportInput struct {
	BoardID  int `json:"board_id"`
	SprintID int `json:"sprint_id,omitempty"`
}

// RiskLevel — категориальный риск спринта.
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// SprintHealthReportOutput — детерминированный человекочитаемый отчёт.
type SprintHealthReportOutput struct {
	Report       tracker.SprintReport `json:"report"`
	Summary      string            `json:"summary"`
	RiskLevel    RiskLevel         `json:"risk_level"`
	ActionItems  []string          `json:"action_items"`
	ScopeAdded   []string          `json:"scope_added"`
	ScopeRemoved []string          `json:"scope_removed"`
}

// SprintHealthReport — handler для sprint_health_report tool.
// sr — источник scope changes (changelog). Может быть nil — тогда scope поля
// остаются пустыми слайсами. Ошибка получения scope не валит handler: она
// пишется в stderr через log, а scope возвращается пустым. Такое поведение
// обеспечивает обратную совместимость, если Jira не выдаёт expand=changelog
// или используется токен без permission.
func SprintHealthReport(r SprintReporter, sr ScopeReader) Handler[SprintHealthReportInput, SprintHealthReportOutput] {
	return func(ctx context.Context, in SprintHealthReportInput) (SprintHealthReportOutput, error) {
		if in.BoardID <= 0 {
			return SprintHealthReportOutput{}, fmt.Errorf("sprint_health_report: board_id is required")
		}
		rep, err := r.GetSprintReport(ctx, in.BoardID, in.SprintID)
		if err != nil {
			return SprintHealthReportOutput{}, err
		}

		risk := computeRisk(rep.Health)
		summary := fmt.Sprintf(
			"Sprint %q: %d total, %d done, %d in progress, %d blocked. Velocity %.1f. Risk: %s.",
			rep.Health.SprintName, rep.Health.Total, rep.Health.Done, rep.Health.InProgress,
			rep.Health.Blocked, rep.Health.Velocity, risk,
		)

		actions := make([]string, 0, len(rep.BlockedIssues))
		for _, b := range rep.BlockedIssues {
			actions = append(actions, fmt.Sprintf("Unblock %s: %s", b.Key, b.Summary))
		}

		scopeAdded := []string{}
		scopeRemoved := []string{}
		if sr != nil && in.SprintID > 0 {
			added, removed, scopeErr := sr.GetSprintScopeChanges(ctx, in.SprintID)
			if scopeErr != nil {
				log.Printf("sprint_health_report: scope changes unavailable for sprint %d: %v", in.SprintID, scopeErr)
			} else {
				if added != nil {
					scopeAdded = added
				}
				if removed != nil {
					scopeRemoved = removed
				}
			}
		}

		return SprintHealthReportOutput{
			Report:       rep,
			Summary:      summary,
			RiskLevel:    risk,
			ActionItems:  actions,
			ScopeAdded:   scopeAdded,
			ScopeRemoved: scopeRemoved,
		}, nil
	}
}

// computeRisk — детерминированная оценка риска по health-агрегатам.
// >20% blocked → high; >10% blocked → medium; иначе low.
// При Velocity < 0.5 от Done понижаем уровень максимум до medium.
func computeRisk(h tracker.SprintHealth) RiskLevel {
	if h.Total == 0 {
		return RiskLow
	}
	blockedRatio := float64(h.Blocked) / float64(h.Total)
	switch {
	case blockedRatio > 0.20:
		return RiskHigh
	case blockedRatio > 0.10:
		return RiskMedium
	default:
		return RiskLow
	}
}
