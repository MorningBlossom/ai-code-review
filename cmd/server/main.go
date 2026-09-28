package main

import (
	"log"
	"net/http"

	"github.com/MorningBlossom/ai-code-review/internal/analyzer"
	"github.com/MorningBlossom/ai-code-review/internal/api"
	"github.com/MorningBlossom/ai-code-review/internal/github"
	"github.com/MorningBlossom/ai-code-review/internal/model"
	"github.com/MorningBlossom/ai-code-review/internal/orchestrator"
	"github.com/MorningBlossom/ai-code-review/internal/publisher"
	"github.com/MorningBlossom/ai-code-review/internal/validator"
)

func main() {
	orchestrator := orchestrator.NewOrchestrator(
		&github.FakeProvider{},
		[]analyzer.Analyzer{
			&analyzer.FakeAnalyzer{},
		},
		&model.FakeModelProvider{},
		&validator.FakeValidator{},
		&publisher.FakePublisher{},
	)
	handler := api.NewHandler(orchestrator)
	http.HandleFunc("/health", handler.Health)
	http.HandleFunc("/reviews", handler.CreateReview)
	log.Println("review API listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
