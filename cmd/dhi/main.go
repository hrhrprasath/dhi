package main

import (
	"fmt"
	"os"

	"dhi_harness/internal/config"
	"dhi_harness/internal/conversation"
	"dhi_harness/internal/intent"
	"dhi_harness/internal/pipeline"
	"dhi_harness/internal/queue"
	"dhi_harness/internal/services"
	"dhi_harness/internal/summarizer"
	"dhi_harness/internal/todo"
	"dhi_harness/tui"

	"charm.land/bubbletea/v2"
)

func main() {
	cfg := config.Load()
	logger := services.NewLogger()

	logger.Info("SYSTEM", fmt.Sprintf("DHI Harness v%s booting...", "0.1.0"))
	logger.Info("CONFIG", fmt.Sprintf("LLM: %s | Model: %s", cfg.LLM.Endpoint, cfg.LLM.Model))

	if err := logger.SetFile(".data/dhi.log"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: log file: %v\n", err)
	}

	qm := queue.NewManager(".data")
	convMgr := conversation.NewManager(".data")

	router := intent.NewRouter(cfg.LLM.Model, cfg.LLM.Endpoint, logger)

	pipeMgr := pipeline.NewManager("config/pipelines.yaml")
	summ := summarizer.New(cfg.LLM.Model, cfg.LLM.Endpoint)
	exec := pipeline.NewExecutor(pipeMgr, summ)

	logger.Info("SYSTEM", fmt.Sprintf("Loaded %d pipelines", pipeMgr.PipelineCount()))

	sysSvc := services.NewSystemService()
	todoMgr := todo.NewManager(".data")

	model := tui.NewModel(cfg, logger, qm, router, exec, convMgr, sysSvc, todoMgr)

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
