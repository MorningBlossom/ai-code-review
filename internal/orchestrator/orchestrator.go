package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/MorningBlossom/ai-code-review/internal/analyzer"
	"github.com/MorningBlossom/ai-code-review/internal/contextbuilder"
	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/model"
	"github.com/MorningBlossom/ai-code-review/internal/publisher"
	"github.com/MorningBlossom/ai-code-review/internal/review"
	"github.com/MorningBlossom/ai-code-review/internal/validator"
	"github.com/MorningBlossom/ai-code-review/internal/workspace"
)

type Orchestrator interface {
	Review(
		ctx context.Context,
		request review.ReviewRequest,
	) (review.ReviewResult, error)
}

type ReviewOrchestrator struct {
	github           github.Provider
	contextBuilder   *contextbuilder.Builder
	workspaceBuilder workspace.Builder

	analyzers          []analyzer.Analyzer
	workspaceAnalyzers []analyzer.WorkspaceAnalyzer

	model     model.ModelProvider
	validator validator.Validator
	publisher publisher.Publisher
}

func NewOrchestrator(
	githubProvider github.Provider,
	contextBuilder *contextbuilder.Builder,
	workspaceBuilder workspace.Builder,
	analyzers []analyzer.Analyzer,
	workspaceAnalyzers []analyzer.WorkspaceAnalyzer,
	modelProvider model.ModelProvider,
	findingValidator validator.Validator,
	reviewPublisher publisher.Publisher,
) *ReviewOrchestrator {
	return &ReviewOrchestrator{
		github:             githubProvider,
		contextBuilder:     contextBuilder,
		workspaceBuilder:   workspaceBuilder,
		analyzers:          analyzers,
		workspaceAnalyzers: workspaceAnalyzers,
		model:              modelProvider,
		validator:          findingValidator,
		publisher:          reviewPublisher,
	}
}

func (o *ReviewOrchestrator) Review(
	ctx context.Context,
	request review.ReviewRequest,
) (review.ReviewResult, error) {
	start := time.Now()

	result := review.ReviewResult{
		ReviewID:          request.ReviewID,
		Organization:      request.Organization,
		Repository:        request.Repository,
		PullRequestNumber: request.PullRequestNumber,
		BaseSHA:           request.BaseSHA,
		HeadSHA:           request.HeadSHA,
		Status:            "running",
	}

	pr, err := o.github.GetPullRequest(
		ctx,
		request.InstallationID,
		request.Organization,
		request.Repository,
		request.PullRequestNumber,
	)
	if err != nil {
		return o.failResult(
			result,
			start,
			fmt.Errorf("get pull request: %w", err),
		)
	}

	changedFiles, err := o.github.GetChangedFiles(
		ctx,
		request.InstallationID,
		request.Organization,
		request.Repository,
		request.PullRequestNumber,
	)
	if err != nil {
		return o.failResult(
			result,
			start,
			fmt.Errorf("get changed files: %w", err),
		)
	}

	reviewContext, err := o.contextBuilder.Build(
		ctx,
		request,
		pr,
		changedFiles,
	)
	if err != nil {
		return o.failResult(
			result,
			start,
			fmt.Errorf("build review context: %w", err),
		)
	}

	// Run analyzers that operate directly on the review context.
	for _, currentAnalyzer := range o.analyzers {
		analyzerResult := currentAnalyzer.Analyze(
			ctx,
			reviewContext,
		)

		appendAnalyzerResult(&result, &reviewContext, analyzerResult)

		if analyzerResult.Status == "failed" {
			return o.failResult(
				result,
				start,
				fmt.Errorf(
					"analyzer %s failed",
					analyzerResult.AnalyzerName,
				),
			)
		}
	}

	// Build a complete repository workspace for repository-level analyzers.
	if len(o.workspaceAnalyzers) > 0 {
		if o.workspaceBuilder == nil {
			return o.failResult(
				result,
				start,
				fmt.Errorf(
					"workspace analyzers configured without workspace builder",
				),
			)
		}

		ws, err := o.workspaceBuilder.Build(
			ctx,
			request.InstallationID,
			request.Organization,
			request.Repository,
			request.HeadSHA,
		)
		if err != nil {
			return o.failResult(
				result,
				start,
				fmt.Errorf(
					"build repository workspace: %w",
					err,
				),
			)
		}

		defer ws.Close()

		for _, currentAnalyzer := range o.workspaceAnalyzers {
			analyzerResult := currentAnalyzer.AnalyzeWorkspace(
				ctx,
				ws,
			)

			appendAnalyzerResult(&result, &reviewContext, analyzerResult)

			if analyzerResult.Status == "failed" {
				return o.failResult(
					result,
					start,
					fmt.Errorf(
						"workspace analyzer %s failed",
						analyzerResult.AnalyzerName,
					),
				)
			}
		}
	}

	// Run the model review.
	modelFindings, err := o.model.Review(
		ctx,
		reviewContext,
	)
	if err != nil {
		return o.failResult(
			result,
			start,
			fmt.Errorf("model review: %w", err),
		)
	}

	result.Findings = append(
		result.Findings,
		modelFindings...,
	)

	// Validate and deduplicate all findings.
	validatedFindings, err := o.validator.ValidateWithContext(
		ctx,
		request,
		reviewContext,
		result.Findings,
	)
	if err != nil {
		return o.failResult(
			result,
			start,
			fmt.Errorf("validate findings: %w", err),
		)
	}

	result.Findings = validatedFindings

	result.Status = "completed"
	result.DurationMillis = time.Since(start).Milliseconds()

	if err := o.publisher.Publish(
		ctx,
		request,
		result,
	); err != nil {
		return o.failResult(
			result,
			start,
			fmt.Errorf("publish review: %w", err),
		)
	}

	return result, nil
}

func (o *ReviewOrchestrator) failResult(
	result review.ReviewResult,
	start time.Time,
	err error,
) (review.ReviewResult, error) {
	result.Status = "failed"
	result.DurationMillis = time.Since(start).Milliseconds()
	result.Errors = append(
		result.Errors,
		err.Error(),
	)

	return result, err
}

func appendAnalyzerResult(
	result *review.ReviewResult,
	reviewContext *review.ReviewContext,
	analyzerResult review.AnalyzerResult,
) {
	result.AnalyzerSummary = append(
		result.AnalyzerSummary,
		analyzerResult,
	)

	result.Findings = append(
		result.Findings,
		analyzerResult.Findings...,
	)

	reviewContext.AnalyzerFindings = append(
		reviewContext.AnalyzerFindings,
		analyzerResult.Findings...,
	)
}
