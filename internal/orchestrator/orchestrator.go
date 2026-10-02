package orchestrator

import (
	"context"
	"fmt"
	"log"
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

	log.Printf(
		"orchestrator started review_id=%s mode=%s org=%s repo=%s pr=%d head=%s",
		request.ReviewID,
		request.ReviewMode,
		request.Organization,
		request.Repository,
		request.PullRequestNumber,
		request.HeadSHA,
	)

	result := review.ReviewResult{
		ReviewID:          request.ReviewID,
		Organization:      request.Organization,
		Repository:        request.Repository,
		PullRequestNumber: request.PullRequestNumber,
		BaseSHA:           request.BaseSHA,
		HeadSHA:           request.HeadSHA,
		Status:            "running",
	}

	log.Printf(
		"orchestrator fetching PR review_id=%s org=%s repo=%s pr=%d",
		request.ReviewID,
		request.Organization,
		request.Repository,
		request.PullRequestNumber,
	)

	pr, err := o.github.GetPullRequest(
		ctx,
		request.InstallationID,
		request.Organization,
		request.Repository,
		request.PullRequestNumber,
	)
	if err != nil {
		log.Printf(
			"orchestrator failed review_id=%s stage=get_pull_request error=%v",
			request.ReviewID,
			err,
		)

		return o.failResult(
			result,
			start,
			fmt.Errorf("get pull request: %w", err),
		)
	}

	log.Printf(
		"orchestrator PR fetched review_id=%s pr=%d draft=%t head=%s base=%s",
		request.ReviewID,
		pr.Number,
		pr.Draft,
		pr.HeadSHA,
		pr.BaseSHA,
	)

	log.Printf(
		"orchestrator fetching changed files review_id=%s pr=%d",
		request.ReviewID,
		request.PullRequestNumber,
	)

	changedFiles, err := o.github.GetChangedFiles(
		ctx,
		request.InstallationID,
		request.Organization,
		request.Repository,
		request.PullRequestNumber,
	)
	if err != nil {
		log.Printf(
			"orchestrator failed review_id=%s stage=get_changed_files error=%v",
			request.ReviewID,
			err,
		)

		return o.failResult(
			result,
			start,
			fmt.Errorf("get changed files: %w", err),
		)
	}

	log.Printf(
		"orchestrator changed files fetched review_id=%s files=%d",
		request.ReviewID,
		len(changedFiles),
	)

	log.Printf(
		"orchestrator building review context review_id=%s",
		request.ReviewID,
	)

	reviewContext, err := o.contextBuilder.Build(
		ctx,
		request,
		pr,
		changedFiles,
	)
	if err != nil {
		log.Printf(
			"orchestrator failed review_id=%s stage=build_context error=%v",
			request.ReviewID,
			err,
		)

		return o.failResult(
			result,
			start,
			fmt.Errorf("build review context: %w", err),
		)
	}

	log.Printf(
		"orchestrator context built review_id=%s",
		request.ReviewID,
	)

	// Run analyzers that operate directly on the review context.
	for _, currentAnalyzer := range o.analyzers {
		log.Printf(
			"analyzer started review_id=%s",
			request.ReviewID,
		)

		analyzerResult := currentAnalyzer.Analyze(
			ctx,
			reviewContext,
		)

		log.Printf(
			"analyzer completed review_id=%s analyzer=%s status=%s findings=%d",
			request.ReviewID,
			analyzerResult.AnalyzerName,
			analyzerResult.Status,
			len(analyzerResult.Findings),
		)

		appendAnalyzerResult(
			&result,
			&reviewContext,
			analyzerResult,
		)

		if analyzerResult.Status == "failed" {
			log.Printf(
				"orchestrator failed review_id=%s stage=analyzer analyzer=%s",
				request.ReviewID,
				analyzerResult.AnalyzerName,
			)

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
			log.Printf(
				"orchestrator failed review_id=%s stage=workspace reason=workspace_builder_missing",
				request.ReviewID,
			)

			return o.failResult(
				result,
				start,
				fmt.Errorf(
					"workspace analyzers configured without workspace builder",
				),
			)
		}

		log.Printf(
			"workspace build started review_id=%s org=%s repo=%s head=%s",
			request.ReviewID,
			request.Organization,
			request.Repository,
			request.HeadSHA,
		)

		ws, err := o.workspaceBuilder.Build(
			ctx,
			request.InstallationID,
			request.Organization,
			request.Repository,
			request.HeadSHA,
		)
		if err != nil {
			log.Printf(
				"orchestrator failed review_id=%s stage=build_workspace error=%v",
				request.ReviewID,
				err,
			)

			return o.failResult(
				result,
				start,
				fmt.Errorf(
					"build repository workspace: %w",
					err,
				),
			)
		}

		log.Printf(
			"workspace build completed review_id=%s",
			request.ReviewID,
		)

		defer func() {
			if err := ws.Close(); err != nil {
				log.Printf(
					"workspace close failed review_id=%s error=%v",
					request.ReviewID,
					err,
				)
			}
		}()

		for _, currentAnalyzer := range o.workspaceAnalyzers {
			log.Printf(
				"workspace analyzer started review_id=%s",
				request.ReviewID,
			)

			analyzerResult := currentAnalyzer.AnalyzeWorkspace(
				ctx,
				ws,
			)

			log.Printf(
				"workspace analyzer completed review_id=%s analyzer=%s status=%s findings=%d",
				request.ReviewID,
				analyzerResult.AnalyzerName,
				analyzerResult.Status,
				len(analyzerResult.Findings),
			)

			appendAnalyzerResult(
				&result,
				&reviewContext,
				analyzerResult,
			)

			if analyzerResult.Status == "failed" {
				log.Printf(
					"orchestrator failed review_id=%s stage=workspace_analyzer analyzer=%s",
					request.ReviewID,
					analyzerResult.AnalyzerName,
				)

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

		log.Printf(
			"workspace analyzers completed review_id=%s findings=%d",
			request.ReviewID,
			len(result.Findings),
		)
	}

	// Run the model review.
	log.Printf(
		"model review started review_id=%s",
		request.ReviewID,
	)

	modelFindings, err := o.model.Review(
		ctx,
		reviewContext,
	)
	if err != nil {
		log.Printf(
			"orchestrator failed review_id=%s stage=model_review error=%v",
			request.ReviewID,
			err,
		)

		return o.failResult(
			result,
			start,
			fmt.Errorf("model review: %w", err),
		)
	}

	log.Printf(
		"model review completed review_id=%s findings=%d",
		request.ReviewID,
		len(modelFindings),
	)

	result.Findings = append(
		result.Findings,
		modelFindings...,
	)

	// Validate and deduplicate all findings.
	log.Printf(
		"finding validation started review_id=%s findings=%d",
		request.ReviewID,
		len(result.Findings),
	)

	validatedFindings, err := o.validator.ValidateWithContext(
		ctx,
		request,
		reviewContext,
		result.Findings,
	)
	if err != nil {
		log.Printf(
			"orchestrator failed review_id=%s stage=validate_findings error=%v",
			request.ReviewID,
			err,
		)

		return o.failResult(
			result,
			start,
			fmt.Errorf("validate findings: %w", err),
		)
	}

	result.Findings = validatedFindings

	log.Printf(
		"finding validation completed review_id=%s findings=%d",
		request.ReviewID,
		len(result.Findings),
	)

	result.Status = "completed"
	result.DurationMillis = time.Since(start).Milliseconds()

	log.Printf(
		"publisher started review_id=%s findings=%d",
		request.ReviewID,
		len(result.Findings),
	)

	if err := o.publisher.Publish(
		ctx,
		request,
		result,
	); err != nil {
		log.Printf(
			"orchestrator failed review_id=%s stage=publish error=%v",
			request.ReviewID,
			err,
		)

		return o.failResult(
			result,
			start,
			fmt.Errorf("publish review: %w", err),
		)
	}

	log.Printf(
		"publisher completed review_id=%s",
		request.ReviewID,
	)

	log.Printf(
		"orchestrator completed review_id=%s status=%s duration_ms=%d findings=%d",
		request.ReviewID,
		result.Status,
		result.DurationMillis,
		len(result.Findings),
	)

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

	log.Printf(
		"orchestrator failed review_id=%s status=%s duration_ms=%d error=%v",
		result.ReviewID,
		result.Status,
		result.DurationMillis,
		err,
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
