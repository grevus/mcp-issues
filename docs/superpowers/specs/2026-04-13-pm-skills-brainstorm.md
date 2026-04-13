# Новые PM-скиллы для mcp-jira: Jira + Confluence

## Context

mcp-jira имеет 10 tools (Phase 0-2). Phase 3 планирует Confluence-коннектор (`internal/docs/`, `internal/docs/confluence/`, `doc_chunks` в pgvector). Для конкурентного преимущества нужны PM-ориентированные tools, использующие оба источника. Остальные источники (Git, Slack, календари) — в последующих фазах.

Источники данных:
- **Jira REST API v3** — issues, changelogs, sprints, boards, issue links, components, fix versions
- **Jira Agile API v1** — boards, sprints, backlog, velocity
- **Confluence REST API** — pages, spaces, search (CQL), page content
- **RAG (pgvector)** — существующий индекс Jira issues + новый индекс Confluence pages (Phase 3)

---

## Направление 1: Планирование и Roadmap

### `capacity_forecast`
**Боль PM:** "Сколько реально влезет в следующий спринт?"
**Входы:** `board_id`, `num_sprints_history` (default 5)
**Выходы:**
- `avg_velocity` (SP за спринт)
- `velocity_trend` (up/down/flat — линейная регрессия)
- `completed_vs_committed_ratio` (% выполнения обязательств)
- `recommended_commitment` (SP)
- `confidence` (low/medium/high — по разбросу velocity)
**Источники:** Jira Agile API — `GET /board/{id}/sprint?state=closed` → для каждого спринта `GET /sprint/{id}/issue` с SP.
**Confluence-обогащение:** Ссылки на ретро-страницы спринтов (CQL: `type=page AND label=retrospective AND text~"Sprint N"`), чтобы PM мог понять *почему* velocity менялась.

### `epic_progress`
**Боль PM:** "Когда закончим фичу X?"
**Входы:** `epic_key`
**Выходы:**
- `total_issues`, `done/in_progress/todo` (counts + %)
- `total_sp`, `completed_sp`, `percent_done`
- `weekly_velocity` (SP/week за последние 4 недели в этом эпике)
- `estimated_completion_date` (линейная экстраполяция)
- `risks[]`: блокеры, issues без SP, scope creep (новые issues за последние 2 недели)
- `related_docs[]`: Confluence-страницы, связанные с эпиком (PRD, RFC, ADR)
**Источники:** Jira — JQL `"Epic Link" = KEY OR parent = KEY`, `expand=changelog` для velocity и scope creep detection. Confluence — CQL `text~"EPIC-KEY"` или linked pages.

### `sprint_planning_suggest`
**Боль PM:** "Что взять в спринт? Planning meeting тянется час."
**Входы:** `board_id`, `target_sp` (optional, default = avg velocity из capacity_forecast)
**Выходы:**
- `suggested_issues[]` (key, summary, sp, priority, reason для включения)
- `total_sp`, `remaining_capacity`
- `warnings[]` (зависимости от незакрытых issues, overcommit risk)
- `carryover[]` (незакрытые из предыдущего спринта — кандидаты на перенос)
**Источники:** Jira Agile — backlog ranking, sprint history. Jira REST — issue links для dependency warnings.
**Логика:** Берём issues по rank из backlog, пока не заполнен target_sp. Проверяем блокирующие зависимости. Carryover из предыдущего спринта добавляется первым.

---

## Направление 2: Метрики и Аналитика

### `team_metrics`
**Боль PM:** "Как у нас дела? Мне нужны цифры для стейкхолдеров."
**Входы:** `project_key`, `from`, `to` (YYYY-MM-DD)
**Выходы:**
- `throughput` (issues closed за период)
- `avg_cycle_time` (дни: In Progress → Done, median)
- `avg_lead_time` (дни: Created → Done, median)
- `wip_avg` (среднее кол-во In Progress одновременно)
- `bug_ratio` (bugs / total closed)
- `sp_completed`
- `comparison` (vs предыдущий аналогичный период — delta %)
**Источники:** Jira REST — JQL search + `expand=changelog` для transition timestamps.
**Confluence-обогащение:** Ссылки на team health check pages если есть.

### `bottleneck_detect`
**Боль PM:** "Почему всё тормозит? Где узкое место?"
**Входы:** `project_key`, `from`, `to`
**Выходы:**
- `slowest_status` (статус с наибольшим avg dwell time)
- `stuck_issues[]` (в одном статусе > 5 дней, с assignee)
- `recurring_blockers[]` (issues, чаще всего встречающиеся в blocker-links)
- `hot_components[]` (компоненты с наибольшим cycle time)
- `recommendation` (шаблонный текст: "Code Review — avg 3.2 days, consider...")
**Источники:** Jira changelog (transition timestamps), issue links (type=Blocks), components.

### `quality_pulse`
**Боль PM:** "Качество падает или растёт? Можно ли релизить?"
**Входы:** `project_key`, `from`, `to`
**Выходы:**
- `bugs_created`, `bugs_resolved`, `net_bug_delta`
- `bug_feature_ratio`
- `avg_bug_resolution_time` (дни)
- `regressions` (count + keys, label=regression)
- `top_affected_components[]`
- `trend` (improving/degrading/stable vs предыдущий период)
- `postmortem_refs[]` (RAG: Confluence postmortem pages по affected components)
**Источники:** Jira — JQL issuetype=Bug + changelog. RAG — семантический поиск postmortems в Confluence index.

---

## Направление 3: Приоритизация и Принятие Решений

### `priority_matrix`
**Боль PM:** "У меня 80 тикетов в backlog, что делать первым?"
**Входы:** `project_key`, `jql` (optional, default = незакрытые issues)
**Выходы:**
- `quadrants`:
  - `quick_wins[]` (high impact, low effort)
  - `strategic[]` (high impact, high effort)
  - `fill_ins[]` (low impact, low effort)
  - `deprioritize[]` (low impact, high effort)
- Каждый issue: key, summary, impact_score, effort_score, scoring_breakdown
**Scoring (детерминированный):**
- effort = story_points (или 1 если нет)
- impact = priority_weight (Blocker=5..Lowest=1) + watchers_count*0.5 + linked_issues_count*0.3 + votes*0.2
- Пороги: median effort/impact делят на 4 квадранта
**Источники:** Jira — priority, SP, links, watchers, votes.

### `dependency_map`
**Боль PM:** "Что блокирует релиз? Кто от кого зависит?"
**Входы:** `keys[]` (issue/epic keys) ИЛИ `jql`, `depth` (default 2)
**Выходы:**
- `nodes[]` (key, summary, status, assignee, team/component)
- `edges[]` (from, to, link_type)
- `critical_path[]` (самая длинная цепочка блокеров)
- `risks[]`:
  - circular dependencies
  - cross-team blockers (разные components/assignees)
  - external blockers (другой project_key)
- `blocked_count`, `blocking_count`
**Источники:** Jira — issue links (blocks/is-blocked-by/relates-to), assignee, components. Рекурсивный обход до `depth`.

### `tech_debt_radar`
**Боль PM:** "Сколько техдолга? Как аргументировать рефакторинг?"
**Входы:** `project_key`, `labels` (default ["tech-debt","technical-debt","refactor"]), `from`, `to`
**Выходы:**
- `total_debt_issues`, `created_vs_resolved` (за период)
- `debt_age_distribution` (0-30d, 30-90d, 90d+)
- `oldest_unresolved[]` (top 5)
- `debt_by_component[]` (component → count)
- `estimated_effort_sp` (sum SP)
- `bug_correlation[]` (компоненты где debt issues коррелируют с высоким bug count)
- `rag_insights[]` (RAG по Jira comments: "hack", "workaround", "temporary fix")
- `confluence_refs[]` (ADR/RFC pages, связанные с компонентами-лидерами по debt)
**Источники:** Jira — labels, components, SP, changelog. RAG — Jira index (keyword patterns) + Confluence index (ADR/RFC).

---

## Сводная таблица

| Tool | Jira API | Confluence | RAG | Новые методы в `internal/jira/` | Сложность |
|------|----------|------------|-----|---------------------------------|-----------|
| capacity_forecast | Agile: sprints, sprint issues | CQL: ретро-страницы | нет | GetClosedSprints, GetSprintIssues | средняя |
| epic_progress | REST: JQL + changelog | CQL: linked pages | нет | GetEpicIssues, GetIssueChangelog | средняя |
| sprint_planning_suggest | Agile: backlog, sprints | нет | нет | GetBacklog | высокая |
| team_metrics | REST: JQL + changelog | optional | нет | ListIssuesWithChangelog | средняя |
| bottleneck_detect | REST: changelog, links | нет | нет | переиспользование | средняя |
| quality_pulse | REST: JQL issuetype=Bug | postmortems | да | переиспользование | низкая |
| priority_matrix | REST: priority, SP, links, votes | нет | нет | расширение ListIssues (votes, watchers) | низкая |
| dependency_map | REST: issue links | нет | нет | GetIssueLinks (рекурсивный) | средняя |
| tech_debt_radar | REST: labels, components | ADR/RFC pages | да | переиспользование | средняя |

## Confluence-зависимости

Tools с **обязательным** Confluence: нет (все работают без Confluence).
Tools с **опциональным** Confluence-обогащением:
- `capacity_forecast` — ссылки на ретро
- `epic_progress` — PRD/RFC/ADR страницы
- `quality_pulse` — postmortem pages
- `tech_debt_radar` — ADR/RFC по компонентам

Это значит: **все 9 tools можно реализовать в Phase 4 (Jira-only)**, а Confluence-обогащение добавить после Phase 3, когда коннектор готов.

## Рекомендуемый порядок

**Wave 1 (quick wins, Jira-only):** `quality_pulse`, `priority_matrix` — мало нового кода, высокий PM-value.
**Wave 2 (core PM):** `epic_progress`, `team_metrics`, `capacity_forecast` — ключевые вопросы PM.
**Wave 3 (advanced):** `bottleneck_detect`, `dependency_map`, `tech_debt_radar`, `sprint_planning_suggest`.
**Wave 4 (Confluence enrichment):** добавить `related_docs`, `postmortem_refs`, `confluence_refs` к tools из Wave 1-3 после Phase 3.

## Verification

- Unit-тест каждого handler с fake Jira/Confluence client
- Integration-тест через testcontainers (pgvector для RAG-tools)
- E2E: вызов через MCP Inspector или Claude Desktop на реальном проекте
- Сравнение выходов с ручным подсчётом в Jira UI
