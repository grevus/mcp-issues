package register

import (
	"github.com/grevus/mcp-jira/internal/handlers"
	"github.com/grevus/mcp-jira/internal/tenant"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register регистрирует все MCP-инструменты в srv.
// reg — реестр тенантов; каждый вызов tool резолвит тенанта по имени API-ключа из контекста.
func Register(srv *mcp.Server, reg *tenant.Registry) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_issues",
		Description: "Search Jira issues using JQL filters (project, status, assignee).",
	}, adaptTenant(reg, func(t *tenant.Tenant) handlers.Handler[handlers.ListIssuesInput, handlers.ListIssuesOutput] {
		return handlers.ListIssues(t.Provider)
	}))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_sprint_health",
		Description: "Return health metrics for the active sprint of a Jira Software board.",
	}, adaptTenant(reg, func(t *tenant.Tenant) handlers.Handler[handlers.SprintHealthInput, handlers.SprintHealthOutput] {
		return handlers.SprintHealth(t.Provider)
	}))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_jira_knowledge",
		Description: "Semantic search over indexed Jira issues for a given project. Use for free-text questions when you don't have a specific issue key. Not a substitute for live Jira filters — data is as fresh as the last indexer run.",
	}, adaptTenant(reg, func(t *tenant.Tenant) handlers.Handler[handlers.SearchKnowledgeInput, handlers.SearchKnowledgeOutput] {
		return handlers.SearchKnowledge(t.Retriever)
	}))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "similar_issues",
		Description: "Find Jira issues semantically similar to a given one. Use for duplicate detection, incident correlation, known-fix discovery. Relies on the RAG index — issue must belong to an indexed project.",
	}, adaptTenant(reg, func(t *tenant.Tenant) handlers.Handler[handlers.SimilarIssuesInput, handlers.SimilarIssuesOutput] {
		return handlers.SimilarIssues(t.Provider, t.Retriever)
	}))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "sprint_health_report",
		Description: "Return a deterministic health report for a Jira Software sprint: aggregates, blocked issues, risk level, action items, and scope_added/scope_removed issue keys (when sprint_id is provided, derived from changelog). If sprint_id is 0, the active sprint of the board is used.",
	}, adaptTenant(reg, func(t *tenant.Tenant) handlers.Handler[handlers.SprintHealthReportInput, handlers.SprintHealthReportOutput] {
		return handlers.SprintHealthReport(t.Provider, t.Provider)
	}))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "standup_digest",
		Description: "Produce an async standup digest for a team over a time window: grouped by done / in-progress / blocked / other. team_key is a Jira project key; from/to are dates (YYYY-MM-DD[ HH:MM]).",
	}, adaptTenant(reg, func(t *tenant.Tenant) handlers.Handler[handlers.StandupDigestInput, handlers.StandupDigestOutput] {
		return handlers.StandupDigest(t.Provider)
	}))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "engineering_qa",
		Description: "Answer an engineering question by returning relevant Jira issues from the RAG index as citations for the LLM client. Jira-only until Phase 3 adds Confluence.",
	}, adaptTenant(reg, func(t *tenant.Tenant) handlers.Handler[handlers.EngineeringQAInput, handlers.EngineeringQAOutput] {
		return handlers.EngineeringQA(t.Retriever)
	}))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "incident_context",
		Description: "Gather incident context for a Jira issue: basic fields, similar past incidents from RAG, plus deterministically extracted suspected causes and recommended checks from the description and comments.",
	}, adaptTenant(reg, func(t *tenant.Tenant) handlers.Handler[handlers.IncidentContextInput, handlers.IncidentContextOutput] {
		return handlers.IncidentContext(t.Provider, t.Provider, t.Retriever)
	}))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ticket_triage",
		Description: "Primary triage of an incoming Jira issue: finds similar issues via RAG and infers a suggested owning team (frequency of assignees) and priority (keyword heuristic). Deterministic, no LLM generation.",
	}, adaptTenant(reg, func(t *tenant.Tenant) handlers.Handler[handlers.TicketTriageInput, handlers.TicketTriageOutput] {
		return handlers.TicketTriage(t.Provider, t.Retriever)
	}))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "release_risk_check",
		Description: "Assess the risk of an upcoming release by fix_version: collects open and blocked Jira issues and semantically-similar postmortems from RAG, returning a deterministic risk level and short summary.",
	}, adaptTenant(reg, func(t *tenant.Tenant) handlers.Handler[handlers.ReleaseRiskCheckInput, handlers.ReleaseRiskCheckOutput] {
		return handlers.ReleaseRiskCheck(t.Provider, t.Retriever)
	}))
}
