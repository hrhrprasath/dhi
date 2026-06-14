package main

import (
	"fmt"
	"os"

	"dhi_harness/internal/config"
	"dhi_harness/internal/intent"
	"dhi_harness/internal/learn"
	"dhi_harness/internal/pipeline"
	"dhi_harness/internal/services"
	"dhi_harness/internal/settings"
	"dhi_harness/internal/summarizer"
	"dhi_harness/internal/task"
	"dhi_harness/internal/todo"
	"dhi_harness/internal/topic"
	"dhi_harness/tui"

	"charm.land/bubbletea/v2"
)

func main() {
	endpoint := config.OllamaEndpoint()
	logger := services.NewLogger()

	logger.Info("SYSTEM", fmt.Sprintf("DHI Harness v%s booting...", "0.1.0"))
	logger.Info("CONFIG", fmt.Sprintf("LLM endpoint: %s", endpoint))

	if err := logger.SetFile(".data/dhi.log"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: log file: %v\n", err)
	}

	taskMgr := task.NewManager(".data", nil)

	router := intent.NewRouter(endpoint, logger)
	learningEngine := learn.NewEngine(".data")
	router.SetWeightProvider(learningEngine)

	pipeMgr := pipeline.NewManager("config/pipelines.yaml")
	summ := summarizer.New(endpoint)
	exec := pipeline.NewExecutor(pipeMgr, summ, taskMgr)

	logger.Info("SYSTEM", fmt.Sprintf("Loaded %d pipelines", pipeMgr.PipelineCount()))
	logger.Info("LEARN", fmt.Sprintf("Learning engine loaded with %d pipeline stats", len(learningEngine.Stats())))

	settingsMgr := settings.NewManager(".data", endpoint)
	if err := settingsMgr.FetchModels(); err != nil {
		logger.Warn("SETTINGS", fmt.Sprintf("Failed to fetch models: %v", err))
	}
	if cur := settingsMgr.CurrentModel(); cur != nil {
		router.SetModel(cur.Name)
		router.SetContextSize(settingsMgr.ContextLength())
		exec.SetModel(cur.Name)
		exec.SetContextSize(settingsMgr.ContextLength())
		learningEngine.SetModel(cur.Name)
	}

	topicMgr := topic.NewManager(".data")
	sysSvc := services.NewSystemService()
	todoMgr := todo.NewManager(".data")

	model := tui.NewModel(logger, taskMgr, router, exec, topicMgr, sysSvc, todoMgr, learningEngine, settingsMgr)

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
