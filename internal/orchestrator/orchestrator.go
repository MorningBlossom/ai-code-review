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
)

type Orchestrator interface {
	Review(
		ctx context.Context,
		request review.ReviewRequest,
	) (review.ReviewResult, error)
}

type ReviewOrchestrator struct {
	github         github.Provider
	contextBuilder *contextbuilder.Builder
	analyzers      []analyzer.Analyzer
	model          model.ModelProvider
	validator      validator.Validator
	publisher      publisher.Publisher
}

func NewOrchestrator(
	githubProvider github.Provider,
	contextBuilder *contextbuilder.Builder,
	analyzers []analyzer.Analyzer,
	modelProvider model.ModelProvider,
	findingValidator validator.Validator,
	reviewPublisher publisher.Publisher,
) *ReviewOrchestrator {
	return &ReviewOrchestrator{
		github:         githubProvider,
		contextBuilder: contextBuilder,
		analyzers:      analyzers,
		model:          modelProvider,
		validator:      findingValidator,
		publisher:      reviewPublisher,
	}
}

func (o *ReviewOrchestrator) Review(
	ctx context.Context,
	request review.ReviewRequest) (review.ReviewResult, error) {
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

	// Get PullRequest
	pullRequest, err := o.github.GetPullRequest(
		ctx,
		request.InstallationID,
		request.Organization,
		request.Repository,
		request.PullRequestNumber,
	)

	if err != nil {
		result.Status = "failed"
		result.DurationMillis = time.Since(start).Milliseconds()
		result.Errors = append(result.Errors, fmt.Sprintf("get pull request: %v", err))

		return result, err
	}

	//	Get Changed Files
	changedFiles, err := o.github.GetChangedFiles(
		ctx,
		request.InstallationID,
		request.Organization,
		request.Repository,
		request.PullRequestNumber,
	)
	if err != nil {
		result.Status = "failed"
		result.DurationMillis = time.Since(start).Milliseconds()
		result.Errors = append(result.Errors, fmt.Sprintf("get changed files: %v", err))
		return result, err
	}

	reviewContext, err := o.contextBuilder.Build(
		ctx,
		request,
		pullRequest,
		changedFiles,
	)
	if err != nil {
		result.Status = "failed"
		result.Errors = append(
			result.Errors,
			err.Error(),
		)

		return result, err
	}

	for _, analyzer := range o.analyzers {
		analyzerResult := analyzer.Analyze(ctx, reviewContext)
		result.AnalyzerSummary = append(result.AnalyzerSummary, analyzerResult)
		reviewContext.AnalyzerFindings = append(reviewContext.AnalyzerFindings, analyzerResult.Findings...)
	}

	// Run Model Review
	modelFindings, err := o.model.Review(ctx, reviewContext)
	if err != nil {
		result.Status = "failed"
		result.DurationMillis = time.Since(start).Milliseconds()
		result.Errors = append(result.Errors, fmt.Sprintf("model review: %v", err))

		return result, err
	}

	// combine analyzer + model findings
	allFindings := append(
		[]review.ReviewFinding{},
		reviewContext.AnalyzerFindings...,
	)

	allFindings = append(allFindings, modelFindings...)

	// validate findings
	validFindings, err := o.validator.Validate(ctx, request, allFindings)
	if err != nil {
		result.Status = "failed"
		result.DurationMillis = time.Since(start).Milliseconds()
		result.Errors = append(result.Errors, fmt.Sprintf("validate findings: %v", err))

		return result, err
	}
	result.Findings = validFindings

	// publish review
	if err := o.publisher.Publish(ctx, request, result); err != nil {
		result.Status = "failed"
		result.DurationMillis = time.Since(start).Milliseconds()
		result.Errors = append(result.Errors, fmt.Sprintf("publish review: %v", err))

		return result, err
	}

	result.Status = "completed"
	result.DurationMillis = time.Since(start).Milliseconds()

	return result, nil
}
