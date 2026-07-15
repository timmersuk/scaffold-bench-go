package report

import (
	"encoding/json"
	"math"
	"sort"
	"time"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
	"github.com/timmersuk/scaffold-bench-go/internal/runner"
	"github.com/timmersuk/scaffold-bench-go/internal/storage"
)

// Category represents a scenario category.
type Category string

const (
	CategorySurgicalEdit       Category = "surgical-edit"
	CategoryScopeDiscipline    Category = "scope-discipline"
	CategoryVerifyAndRepair    Category = "verify-and-repair"
	CategoryImplementation     Category = "implementation"
	CategoryReadOnlyAnalysis   Category = "read-only-analysis"
	CategoryResponsiveness     Category = "responsiveness"
	CategoryLongContext        Category = "long-context"
)

// AllCategories returns all valid categories in canonical order.
func AllCategories() []Category {
	return []Category{
		CategorySurgicalEdit,
		CategoryScopeDiscipline,
		CategoryVerifyAndRepair,
		CategoryImplementation,
		CategoryReadOnlyAnalysis,
		CategoryResponsiveness,
		CategoryLongContext,
	}
}

// Difficulty represents a scenario difficulty tier.
type Difficulty string

const (
	DifficultyLow    Difficulty = "low"
	DifficultyMedium Difficulty = "medium"
	DifficultyHigh   Difficulty = "high"
)

// AllDifficulties returns all valid difficulties in canonical order.
func AllDifficulties() []Difficulty {
	return []Difficulty{DifficultyLow, DifficultyMedium, DifficultyHigh}
}

// ContextCap represents a context window cap for retrospective analysis.
type ContextCap int

const (
	ContextCap8K   ContextCap = 8192
	ContextCap16K  ContextCap = 16384
	ContextCap32K  ContextCap = 32768
	ContextCap64K  ContextCap = 65536
	ContextCap128K ContextCap = 131072
)

// AllContextCaps returns all context caps in ascending order.
func AllContextCaps() []ContextCap {
	return []ContextCap{ContextCap8K, ContextCap16K, ContextCap32K, ContextCap64K, ContextCap128K}
}

// CategoryScore represents aggregated scores for a category.
type CategoryScore struct {
	Points    int      `json:"points"`
	MaxPoints int      `json:"maxPoints"`
	Pct       *float64 `json:"pct"`
}

// DifficultyScore represents aggregated scores for a difficulty tier.
type DifficultyScore struct {
	Points    int      `json:"points"`
	MaxPoints int      `json:"maxPoints"`
	Pct       *float64 `json:"pct"`
}

// ModelAggregate represents aggregated metrics for a model.
type ModelAggregate struct {
	Model              string                    `json:"model"`
	Source             string                    `json:"source"`
	Runs               int                       `json:"runs"`
	ScorePct           float64                   `json:"scorePct"`
	SolveAttempts      int                       `json:"solveAttempts"`
	SolveCount         int                       `json:"solveCount"`
	SolveRatePct       float64                   `json:"solveRatePct"`
	SolveCiLowPct      float64                   `json:"solveCiLowPct"`
	SolveCiHighPct     float64                   `json:"solveCiHighPct"`
	DisciplinePct      float64                   `json:"disciplinePct"`
	VerifyRatePct      *float64                  `json:"verifyRatePct"`
	VerifyEligibleRuns int                       `json:"verifyEligibleRuns"`
	BashCallsPerRun    *float64                  `json:"bashCallsPerRun"`
	VerifyPassesPerRun *float64                  `json:"verifyPassesPerRun"`
	PointsAvg          float64                   `json:"pointsAvg"`
	MaxAvg             float64                   `json:"maxAvg"`
	TotalWallSeconds   float64                   `json:"totalWallSeconds"`
	AvgScenarioSeconds float64                   `json:"avgScenarioSeconds"`
	AvgFirstTokenSec   *float64                  `json:"avgFirstTokenSeconds"`
	CompletionTps      *float64                  `json:"completionTps"`
	CompletionTpsApprox bool                     `json:"completionTpsApprox"`
	PromptTps          *float64                  `json:"promptTps"`
	PromptTpsApprox    bool                      `json:"promptTpsApprox"`
	AvgTokensPerScenario float64                 `json:"avgTokensPerScenario"`
	AvgTokensPerRun    float64                   `json:"avgTokensPerRun"`
	PromptTokensAvg    float64                   `json:"promptTokensAvg"`
	CompletionTokensAvg float64                  `json:"completionTokensAvg"`
	ParetoFrontier     bool                      `json:"paretoFrontier"`
	ToolCallsTotal     int                       `json:"toolCallsTotal"`
	Requests           int                       `json:"requests"`
	Timeouts           int                       `json:"timeouts"`
	ExemptScenarios    int                       `json:"exemptScenarios"`
	Categories         map[string]CategoryScore  `json:"categories"`
	Tiers              map[string]DifficultyScore `json:"tiers"`
	ScenarioCount      int                       `json:"scenarioCount"`
	LatestTimestamp    string                    `json:"latestTimestamp"`
	AvgContextPerTurn  *float64                  `json:"avgContextPerTurn"`
	ContextPerTurnByHarness map[string]float64   `json:"contextPerTurnByHarness,omitempty"`
	ContextByTurn      []ContextByTurn           `json:"contextByTurn,omitempty"`
	SolveRateByContextCap *SolveRateByContextCap `json:"solveRateByContextCap,omitempty"`
}

// ContextByTurn represents positional mean of prompt tokens at each turn index.
type ContextByTurn struct {
	Turn             int     `json:"turn"`
	MeanPromptTokens float64 `json:"meanPromptTokens"`
	Runs             int     `json:"runs"`
}

// SolveRateByContextCap represents retrospective solve rate under context caps.
type SolveRateByContextCap struct {
	Attempts int              `json:"attempts"`
	Points   []ContextCapPoint `json:"points"`
}

// ContextCapPoint represents solve rate at a specific context cap.
type ContextCapPoint struct {
	Cap     int     `json:"cap"`
	Solved  int     `json:"solved"`
	Pct     float64 `json:"pct"`
}

// ParetoPoint represents a scenario-run for Pareto analysis.
type ParetoPoint struct {
	Model        string  `json:"model"`
	Source       string  `json:"source"`
	ScenarioID   string  `json:"scenarioId"`
	Category     string  `json:"category"`
	Points       int     `json:"points"`
	MaxPoints    int     `json:"maxPoints"`
	ScorePct     float64 `json:"scorePct"`
	Correctness  *int    `json:"correctness"`
	TotalTokens  int     `json:"totalTokens"`
}

// Totals represents aggregate totals across all models.
type Totals struct {
	Models       int `json:"models"`
	Runs         int `json:"runs"`
	Local        int `json:"local"`
	Remote       int `json:"remote"`
	ScenarioRuns int `json:"scenarioRuns"`
}

// Awards represents leaderboard awards.
type Awards struct {
	BestOverall      *ModelAggregate `json:"bestOverall,omitempty"`
	BestEfficiency   *ModelAggregate `json:"bestEfficiency,omitempty"`
	FastestGeneration *ModelAggregate `json:"fastestGeneration,omitempty"`
	FastestPrompt    *ModelAggregate `json:"fastestPrompt,omitempty"`
}

// Data represents the complete report data.
type Data struct {
	Models     []ModelAggregate `json:"models"`
	Categories []string         `json:"categories"`
	Totals     Totals           `json:"totals"`
	Snapshot   string           `json:"snapshot"`
	Awards     Awards           `json:"awards"`
	Pareto     []ParetoPoint    `json:"pareto"`
}

// Builder constructs report data from stored runs.
type Builder struct {
	store    *storage.Store
	registry *runner.Registry
}

// NewBuilder creates a report builder.
func NewBuilder(store *storage.Store, registry *runner.Registry) *Builder {
	return &Builder{store: store, registry: registry}
}

// Build computes the report data.
func (b *Builder) Build() (Data, error) {
	runs, err := b.store.ListRuns()
	if err != nil {
		return Data{}, err
	}

	// Filter to completed runs
	var completedRuns []model.Run
	for _, r := range runs {
		if r.Status == model.RunDone && r.TotalPoints != nil && r.MaxPoints != nil && r.FinishedAt != nil {
			completedRuns = append(completedRuns, r)
		}
	}

	// Build scenario runs map
	scenarioRunsByRun := make(map[string][]model.ScenarioRun)
	for _, r := range completedRuns {
		_, scenarios, err := b.store.GetRunWithScenarios(r.ID)
		if err != nil {
			return Data{}, err
		}
		scenarioRunsByRun[r.ID] = scenarios
	}

	// Aggregate by model
	accByModel := make(map[string]*modelAccumulator)
	var paretoPoints []ParetoPoint

	for _, run := range completedRuns {
		modelName := run.Model
		if modelName == "" {
			modelName = "unknown"
		}
		acc := accByModel[modelName]
		if acc == nil {
			acc = newModelAccumulator()
			accByModel[modelName] = acc
		}
		acc.runIDs[run.ID] = struct{}{}
		acc.totalPoints += *run.TotalPoints
		acc.maxPoints += *run.MaxPoints
		if run.FinishedAt != nil && *run.FinishedAt > acc.latestFinishedAt {
			acc.latestFinishedAt = *run.FinishedAt
		}

		scenarios := scenarioRunsByRun[run.ID]
		for _, sr := range scenarios {
			b.accumulateScenario(acc, &run, &sr, &paretoPoints)
		}
	}

	// Finalize models
	var models []ModelAggregate
	for modelName, acc := range accByModel {
		m := b.finalizeModel(modelName, acc)
		models = append(models, m)
	}

	// Sort by solve rate desc, then score pct desc
	sort.Slice(models, func(i, j int) bool {
		if models[i].SolveRatePct != models[j].SolveRatePct {
			return models[i].SolveRatePct > models[j].SolveRatePct
		}
		return models[i].ScorePct > models[j].ScorePct
	})

	// Compute Pareto frontier
	frontierIdx := paretoFrontier(models)
	for _, idx := range frontierIdx {
		models[idx].ParetoFrontier = true
	}

	// Compute awards
	awards := b.computeAwards(models)

	// Compute categories from registry
	categories := b.computeCategories()

	// Compute totals
	totals := b.computeTotals(models, completedRuns)

	return Data{
		Models:     models,
		Categories: categories,
		Totals:     totals,
		Snapshot:   time.Now().UTC().Format("2006-01-02 15:04:05") + " UTC",
		Awards:     awards,
		Pareto:     paretoPoints,
	}, nil
}

func (b *Builder) accumulateScenario(acc *modelAccumulator, run *model.Run, sr *model.ScenarioRun, pareto *[]ParetoPoint) {
	acc.scenarioWallMs += int64(ptrOrZero(sr.WallTimeMs))
	acc.totalWallMs += int64(ptrOrZero(sr.WallTimeMs))
	acc.scenarioRuns++
	acc.scenarioIDs[sr.ScenarioID] = struct{}{}
	acc.toolCalls += ptrOrZero(sr.ToolCallCount)

	if sr.ErrorKind == "timeout" {
		acc.timeouts++
	} else if sr.ErrorKind == "infra" || sr.ErrorKind == "aborted" {
		acc.exemptScenarios++
	}

	// Solve rows (10pt rubric only)
	if sr.RubricKind == "10pt" && sr.Correctness != nil && sr.ErrorKind != "infra" && sr.ErrorKind != "aborted" {
		acc.solveRows = append(acc.solveRows, solveDimRow{
			Correctness:  *sr.Correctness,
			Scope:        sr.Scope,
			Pattern:      sr.Pattern,
			Verification: sr.Verification,
			Cleanup:      sr.Cleanup,
		})
	}

	// Verify eligibility
	if sr.Mutated != nil && (sr.Status == model.ScenarioPass || sr.Status == model.ScenarioPartial || sr.Status == model.ScenarioFail) && sr.ErrorKind != "infra" && sr.ErrorKind != "aborted" {
		acc.verify.eligible++
		acc.verify.bashCallsSum += ptrOrZero(sr.BashCalls)
		acc.verify.verifyPassesSum += ptrOrZero(sr.VerifyPasses)
		if *sr.Mutated {
			acc.verify.mutating++
			if ptrOrZero(sr.VerifyPasses) >= 1 {
				acc.verify.verified++
			}
		}
	}

	// First token
	if sr.FirstTokenMs != nil {
		acc.firstTokenSumMs += *sr.FirstTokenMs
		acc.firstTokenCount++
	}

	// Category
	categoryName := sr.Category
	if categoryName == "" {
		categoryName = "unknown"
	}
	cat := acc.categories[categoryName]
	cat.Points += ptrOrZero(sr.Points)
	cat.MaxPoints += sr.MaxPoints
	acc.categories[categoryName] = cat

	// Tier (difficulty from registry)
	if sc, ok := b.registry.Get(sr.ScenarioID); ok {
		diff := Difficulty(sc.Manifest.Meta.Difficulty)
		if diff == DifficultyLow || diff == DifficultyMedium || diff == DifficultyHigh {
			tier := acc.tiers[string(diff)]
			tier.Points += ptrOrZero(sr.Points)
			tier.MaxPoints += sr.MaxPoints
			acc.tiers[string(diff)] = tier
		}
	}

	// Metrics
	if sr.ModelMetricsJSON != "" {
		var metrics modelMetrics
		if err := json.Unmarshal([]byte(sr.ModelMetricsJSON), &metrics); err == nil {
			addMetrics(acc, &metrics)
			exempt := sr.ErrorKind == "infra" || sr.ErrorKind == "aborted" || sr.ErrorKind == "timeout"
			if !exempt {
				acc.metricScenarioRuns++
				prompt := finiteFloat(metrics.PromptTokens)
				reqs := finiteFloat(metrics.RequestCount)
				if prompt > 0 && reqs > 0 {
					acc.contextRows = append(acc.contextRows, contextRow{
						Harness: sr.RubricKind,
						Ratio:   prompt / reqs,
					})
				}
			}
			if metrics.Requests != nil && len(metrics.Requests) > 0 {
				series := make([]requestPoint, len(metrics.Requests))
				for i, r := range metrics.Requests {
					series[i] = requestPoint{PromptTokens: r.PromptTokens}
				}
				acc.seriesRuns = append(acc.seriesRuns, series)
				if sr.RubricKind == "10pt" && sr.Correctness != nil && sr.ErrorKind != "infra" && sr.ErrorKind != "aborted" {
					peak := peakContextTokens(metrics.Requests)
					acc.capRows = append(acc.capRows, capRow{
						Solved: *sr.Correctness == 3,
						Peak:   peak,
					})
				}
			}

			// Pareto point
			if metrics.TotalTokens > 0 && sr.ErrorKind != "infra" && sr.ErrorKind != "aborted" && sr.ErrorKind != "timeout" {
				*pareto = append(*pareto, ParetoPoint{
					Model:       run.Model,
					Source:      run.Source,
					ScenarioID:  sr.ScenarioID,
					Category:    sr.Category,
					Points:      ptrOrZero(sr.Points),
					MaxPoints:   sr.MaxPoints,
					ScorePct:    safeDiv(float64(ptrOrZero(sr.Points)), float64(sr.MaxPoints)) * 100,
					Correctness: sr.Correctness,
					TotalTokens: metrics.TotalTokens,
				})
			}
		}
	}
}

func (b *Builder) finalizeModel(modelName string, acc *modelAccumulator) ModelAggregate {
	completion := completionTps(acc)
	prompt := promptTps(acc)
	runCount := max(1, len(acc.runIDs))
	solve := computeSolveStats(acc.solveRows)

	ratios := make([]float64, len(acc.contextRows))
	for i, r := range acc.contextRows {
		ratios[i] = r.Ratio
	}
	avgContextPerTurn := meanContextPerTurn(ratios)
	byHarness := contextPerTurnByHarness(acc.contextRows)
	contextByTurnArr := positionalMeans(acc.seriesRuns)
	capCurve := computeSolveRateByContextCap(acc.capRows)

	var verifyRate *float64
	if acc.verify.mutating > 0 {
		v := 100 * float64(acc.verify.verified) / float64(acc.verify.mutating)
		verifyRate = &v
	}

	var bashCallsPerRun *float64
	if acc.verify.eligible > 0 {
		v := float64(acc.verify.bashCallsSum) / float64(acc.verify.eligible)
		bashCallsPerRun = &v
	}

	var verifyPassesPerRun *float64
	if acc.verify.eligible > 0 {
		v := float64(acc.verify.verifyPassesSum) / float64(acc.verify.eligible)
		verifyPassesPerRun = &v
	}

	var avgFirstTokenSec *float64
	if acc.firstTokenCount > 0 {
		v := float64(acc.firstTokenSumMs) / float64(acc.firstTokenCount) / 1000
		avgFirstTokenSec = &v
	}

	categories := make(map[string]CategoryScore)
	for _, cat := range AllCategories() {
		agg := acc.categories[string(cat)]
		if agg.MaxPoints > 0 {
			pct := float64(agg.Points) / float64(agg.MaxPoints) * 100
			categories[string(cat)] = CategoryScore{Points: agg.Points, MaxPoints: agg.MaxPoints, Pct: &pct}
		} else {
			categories[string(cat)] = CategoryScore{Points: agg.Points, MaxPoints: 0, Pct: nil}
		}
	}

	tiers := make(map[string]DifficultyScore)
	for _, diff := range AllDifficulties() {
		agg := acc.tiers[string(diff)]
		if agg.MaxPoints > 0 {
			pct := float64(agg.Points) / float64(agg.MaxPoints) * 100
			tiers[string(diff)] = DifficultyScore{Points: agg.Points, MaxPoints: agg.MaxPoints, Pct: &pct}
		}
	}

	var latestTimestamp string
	if acc.latestFinishedAt > 0 {
		latestTimestamp = time.UnixMilli(acc.latestFinishedAt).UTC().Format(time.RFC3339)
	}

	return ModelAggregate{
		Model:              modelName,
		Source:             acc.source,
		Runs:               len(acc.runIDs),
		ScorePct:           safeDiv(float64(acc.totalPoints), float64(acc.maxPoints)) * 100,
		SolveAttempts:      solve.solveAttempts,
		SolveCount:         solve.solveCount,
		SolveRatePct:       solve.solveRatePct,
		SolveCiLowPct:      solve.solveCiLowPct,
		SolveCiHighPct:     solve.solveCiHighPct,
		DisciplinePct:      solve.disciplinePct,
		VerifyRatePct:      verifyRate,
		VerifyEligibleRuns: acc.verify.eligible,
		BashCallsPerRun:    bashCallsPerRun,
		VerifyPassesPerRun: verifyPassesPerRun,
		PointsAvg:          float64(acc.totalPoints) / float64(runCount),
		MaxAvg:             float64(acc.maxPoints) / float64(runCount),
		TotalWallSeconds:   float64(acc.totalWallMs) / 1000 / float64(runCount),
		AvgScenarioSeconds: safeDiv(float64(acc.scenarioWallMs), float64(acc.scenarioRuns)) / 1000,
		AvgFirstTokenSec:   avgFirstTokenSec,
		CompletionTps:      completion.value,
		CompletionTpsApprox: completion.approx,
		PromptTps:          prompt.value,
		PromptTpsApprox:    prompt.approx,
		AvgTokensPerScenario: safeDiv(float64(acc.totalTokens), float64(acc.metricScenarioRuns)),
		AvgTokensPerRun:    float64(acc.totalTokens) / float64(runCount),
		PromptTokensAvg:    float64(acc.promptTokens) / float64(runCount),
		CompletionTokensAvg: float64(acc.completionTokens) / float64(runCount),
		ParetoFrontier:     false,
		ToolCallsTotal:     int(safeDiv(float64(acc.toolCalls), float64(runCount))),
		Requests:           int(safeDiv(float64(acc.requests), float64(runCount))),
		Timeouts:           acc.timeouts,
		ExemptScenarios:    acc.exemptScenarios,
		Categories:         categories,
		Tiers:              tiers,
		ScenarioCount:      len(acc.scenarioIDs),
		LatestTimestamp:    latestTimestamp,
		AvgContextPerTurn:  avgContextPerTurn,
		ContextPerTurnByHarness: byHarness,
		ContextByTurn:      contextByTurnArr,
		SolveRateByContextCap: capCurve,
	}
}

func (b *Builder) computeAwards(models []ModelAggregate) Awards {
	var awards Awards

	if len(models) > 0 {
		awards.BestOverall = &models[0]
	}

	// Best efficiency (score per second)
	var bestEff *ModelAggregate
	bestEffRatio := -1.0
	for i := range models {
		if models[i].AvgScenarioSeconds > 0 {
			ratio := models[i].ScorePct / models[i].AvgScenarioSeconds
			if ratio > bestEffRatio {
				bestEffRatio = ratio
				bestEff = &models[i]
			}
		}
	}
	awards.BestEfficiency = bestEff

	// Fastest generation
	var fastestGen *ModelAggregate
	fastestGenTps := -1.0
	for i := range models {
		if models[i].CompletionTps != nil && *models[i].CompletionTps > fastestGenTps {
			fastestGenTps = *models[i].CompletionTps
			fastestGen = &models[i]
		}
	}
	awards.FastestGeneration = fastestGen

	// Fastest prompt
	var fastestPrompt *ModelAggregate
	fastestPromptTps := -1.0
	for i := range models {
		if models[i].PromptTps != nil && *models[i].PromptTps > fastestPromptTps {
			fastestPromptTps = *models[i].PromptTps
			fastestPrompt = &models[i]
		}
	}
	awards.FastestPrompt = fastestPrompt

	return awards
}

func (b *Builder) computeCategories() []string {
	cats := make([]string, 0)
	for _, cat := range AllCategories() {
		cats = append(cats, string(cat))
	}
	return cats
}

func (b *Builder) computeTotals(models []ModelAggregate, runs []model.Run) Totals {
	var totals Totals
	totals.Models = len(models)
	totals.Runs = len(runs)
	for _, m := range models {
		if m.Source == "local" {
			totals.Local++
		} else if m.Source == "remote" {
			totals.Remote++
		}
	}

	// Actual scenario run count from database
	for _, run := range runs {
		_, scenarios, err := b.store.GetRunWithScenarios(run.ID)
		if err == nil {
			totals.ScenarioRuns += len(scenarios)
		}
	}

	return totals
}

// Helper types and functions

type modelAccumulator struct {
	source              string
	runIDs              map[string]struct{}
	totalPoints         int
	maxPoints           int
	totalWallMs         int64
	scenarioWallMs      int64
	scenarioRuns        int
	firstTokenSumMs     int64
	firstTokenCount     int
	promptEvalTokens    int
	promptEvalTimeMs    int64
	completionEvalTokens int
	completionEvalTimeMs int64
	hasPromptTiming     bool
	hasCompletionTiming bool
	promptTokens        int
	completionTokens    int
	totalTokens         int
	metricScenarioRuns  int
	totalRequestTimeMs  int64
	requests            int
	toolCalls           int
	timeouts            int
	exemptScenarios     int
	categories          map[string]categoryAggregate
	tiers               map[string]categoryAggregate
	scenarioIDs         map[string]struct{}
	latestFinishedAt    int64
	solveRows           []solveDimRow
	contextRows         []contextRow
	seriesRuns          [][]requestPoint
	capRows             []capRow
	verify              verifyAcc
}

type categoryAggregate struct {
	Points    int
	MaxPoints int
}

type solveDimRow struct {
	Correctness  int
	Scope        *int
	Pattern      *int
	Verification *int
	Cleanup      *int
}

type contextRow struct {
	Harness string
	Ratio   float64
}

type requestPoint struct {
	PromptTokens int
}

type capRow struct {
	Solved bool
	Peak   int
}

type verifyAcc struct {
	eligible       int
	mutating       int
	verified       int
	bashCallsSum   int
	verifyPassesSum int
}

type modelMetrics struct {
	RequestCount         int              `json:"requestCount"`
	PromptTokens         int              `json:"promptTokens"`
	CompletionTokens     int              `json:"completionTokens"`
	TotalTokens          int              `json:"totalTokens"`
	TotalRequestTimeMs   int64            `json:"totalRequestTimeMs"`
	PromptEvalTokens     int              `json:"promptEvalTokens,omitempty"`
	PromptEvalTimeMs     int64            `json:"promptEvalTimeMs,omitempty"`
	CompletionEvalTokens int              `json:"completionEvalTokens,omitempty"`
	CompletionEvalTimeMs int64            `json:"completionEvalTimeMs,omitempty"`
	Requests             []requestMetrics `json:"requests,omitempty"`
}

type requestMetrics struct {
	PromptTokens     int   `json:"promptTokens"`
	CompletionTokens int   `json:"completionTokens"`
	RequestTimeMs    int64 `json:"requestTimeMs"`
}

func newModelAccumulator() *modelAccumulator {
	return &modelAccumulator{
		runIDs:      make(map[string]struct{}),
		scenarioIDs: make(map[string]struct{}),
		categories:  make(map[string]categoryAggregate),
		tiers:       make(map[string]categoryAggregate),
	}
}

func addMetrics(acc *modelAccumulator, m *modelMetrics) {
	acc.promptTokens += m.PromptTokens
	acc.completionTokens += m.CompletionTokens
	total := m.TotalTokens
	if total == 0 {
		total = m.PromptTokens + m.CompletionTokens
	}
	acc.totalTokens += total
	acc.totalRequestTimeMs += m.TotalRequestTimeMs
	acc.requests += m.RequestCount

	if m.PromptEvalTokens > 0 && m.PromptEvalTimeMs > 0 {
		acc.promptEvalTokens += m.PromptEvalTokens
		acc.promptEvalTimeMs += m.PromptEvalTimeMs
		acc.hasPromptTiming = true
	}

	if m.CompletionEvalTokens > 0 && m.CompletionEvalTimeMs > 0 {
		acc.completionEvalTokens += m.CompletionEvalTokens
		acc.completionEvalTimeMs += m.CompletionEvalTimeMs
		acc.hasCompletionTiming = true
	}
}

type solveStats struct {
	solveAttempts int
	solveCount    int
	solveRatePct  float64
	solveCiLowPct float64
	solveCiHighPct float64
	disciplinePct float64
}

func computeSolveStats(rows []solveDimRow) solveStats {
	solveAttempts := len(rows)
	solveCount := 0
	for _, r := range rows {
		if r.Correctness == 3 {
			solveCount++
		}
	}

	low, high := wilsonInterval(solveCount, solveAttempts)

	disciplineSum := 0.0
	disciplineCount := 0
	for _, r := range rows {
		if r.Scope == nil && r.Pattern == nil && r.Verification == nil && r.Cleanup == nil {
			continue
		}
		dims := ptrOrZero(r.Scope) + ptrOrZero(r.Pattern) + ptrOrZero(r.Verification) + ptrOrZero(r.Cleanup)
		disciplineSum += 100 * float64(dims) / 7
		disciplineCount++
	}

	return solveStats{
		solveAttempts:  solveAttempts,
		solveCount:     solveCount,
		solveRatePct:   safeDiv(float64(solveCount), float64(solveAttempts)) * 100,
		solveCiLowPct:  low,
		solveCiHighPct: high,
		disciplinePct:  safeDiv(disciplineSum, float64(disciplineCount)),
	}
}

func wilsonInterval(successes, total int) (float64, float64) {
	if total <= 0 {
		return 0, 0
	}
	z := 1.96
	p := float64(successes) / float64(total)
	z2 := z * z
	center := (p + z2/(2*float64(total))) / (1 + z2/float64(total))
	halfwidth := (z * math.Sqrt(p*(1-p)/float64(total)+z2/(4*float64(total)*float64(total)))) / (1 + z2/float64(total))
	low := math.Max(0, center-halfwidth) * 100
	high := math.Min(1, center+halfwidth) * 100
	return low, high
}

func meanContextPerTurn(ratios []float64) *float64 {
	if len(ratios) == 0 {
		return nil
	}
	sum := 0.0
	for _, r := range ratios {
		sum += r
	}
	v := sum / float64(len(ratios))
	return &v
}

func contextPerTurnByHarness(rows []contextRow) map[string]float64 {
	groups := make(map[string]struct {
		sum float64
		n   int
	})
	for _, r := range rows {
		h := r.Harness
		if h == "" {
			h = "unknown"
		}
		g := groups[h]
		g.sum += r.Ratio
		g.n++
		groups[h] = g
	}

	withData := make(map[string]struct {
		sum float64
		n   int
	})
	for h, g := range groups {
		if g.n > 0 {
			withData[h] = g
		}
	}
	if len(withData) < 2 {
		return nil
	}

	out := make(map[string]float64)
	for h, g := range withData {
		out[h] = g.sum / float64(g.n)
	}
	return out
}

func positionalMeans(series [][]requestPoint) []ContextByTurn {
	if len(series) == 0 {
		return nil
	}
	maxLen := 0
	for _, s := range series {
		if len(s) > maxLen {
			maxLen = len(s)
		}
	}

	var out []ContextByTurn
	for i := 0; i < maxLen; i++ {
		sum := 0.0
		n := 0
		for _, s := range series {
			if i < len(s) {
				sum += float64(s[i].PromptTokens)
				n++
			}
		}
		if n > 0 {
			out = append(out, ContextByTurn{
				Turn:             i + 1,
				MeanPromptTokens: sum / float64(n),
				Runs:             n,
			})
		}
	}
	return out
}

func computeSolveRateByContextCap(rows []capRow) *SolveRateByContextCap {
	if len(rows) == 0 {
		return nil
	}
	caps := AllContextCaps()
	points := make([]ContextCapPoint, len(caps))
	for i, cap := range caps {
		solved := 0
		for _, r := range rows {
			if r.Solved && r.Peak <= int(cap) {
				solved++
			}
		}
		points[i] = ContextCapPoint{
			Cap:    int(cap),
			Solved: solved,
			Pct:    safeDiv(float64(solved), float64(len(rows))) * 100,
		}
	}
	return &SolveRateByContextCap{
		Attempts: len(rows),
		Points:   points,
	}
}

func peakContextTokens(requests []requestMetrics) int {
	peak := 0
	for _, r := range requests {
		total := r.PromptTokens + r.CompletionTokens
		if total > peak {
			peak = total
		}
	}
	return peak
}

func completionTps(acc *modelAccumulator) (struct {
	value  *float64
	approx bool
}) {
	if acc.hasCompletionTiming && acc.completionEvalTimeMs > 0 {
		v := float64(acc.completionEvalTokens) / (float64(acc.completionEvalTimeMs) / 1000)
		return struct {
			value  *float64
			approx bool
		}{&v, false}
	}
	if acc.completionTokens > 0 && acc.totalRequestTimeMs > 0 {
		v := float64(acc.completionTokens) / (float64(acc.totalRequestTimeMs) / 1000)
		return struct {
			value  *float64
			approx bool
		}{&v, true}
	}
	return struct {
		value  *float64
		approx bool
	}{nil, false}
}

func promptTps(acc *modelAccumulator) (struct {
	value  *float64
	approx bool
}) {
	if acc.hasPromptTiming && acc.promptEvalTimeMs > 0 {
		v := float64(acc.promptEvalTokens) / (float64(acc.promptEvalTimeMs) / 1000)
		return struct {
			value  *float64
			approx bool
		}{&v, false}
	}
	if acc.promptTokens > 0 && acc.totalRequestTimeMs > 0 {
		v := float64(acc.promptTokens) / (float64(acc.totalRequestTimeMs) / 1000)
		return struct {
			value  *float64
			approx bool
		}{&v, true}
	}
	return struct {
		value  *float64
		approx bool
	}{nil, false}
}

func paretoFrontier(models []ModelAggregate) []int {
	type point struct {
		idx   int
		tokens float64
		score float64
	}
	var points []point
	for i, m := range models {
		if m.AvgTokensPerScenario > 0 {
			points = append(points, point{i, m.AvgTokensPerScenario, m.ScorePct})
		}
	}

	var frontier []int
	for i, p := range points {
		dominated := false
		for j, q := range points {
			if i == j {
				continue
			}
			if q.tokens <= p.tokens && q.score >= p.score && (q.tokens < p.tokens || q.score > p.score) {
				dominated = true
				break
			}
		}
		if !dominated {
			frontier = append(frontier, p.idx)
		}
	}
	return frontier
}

func ptrOrZero[T int | int64 | float64](p *T) T {
	if p == nil {
		return 0
	}
	return *p
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func finiteFloat(v int) float64 {
	return float64(v)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
