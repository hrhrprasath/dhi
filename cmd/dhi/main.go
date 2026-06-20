package main

import (
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"

	"dhi_harness/internal/ai"
	"dhi_harness/internal/config"
	"dhi_harness/internal/dhroot"
	"dhi_harness/internal/intent"
	"dhi_harness/internal/learn"
	"dhi_harness/internal/pipeline"
	"dhi_harness/internal/rss"
	"dhi_harness/internal/services"
	"dhi_harness/internal/settings"
	"dhi_harness/internal/summarizer"
	"dhi_harness/internal/task"
	"dhi_harness/internal/todo"
	"dhi_harness/internal/topic"
	"dhi_harness/tui"

	"charm.land/bubbletea/v2"
)

var version = "dev"

func main() {
	if err := dhroot.EnsureDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "Init error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load(dhroot.ConfigPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	logger := services.NewLogger()

	logger.Info("SYSTEM", fmt.Sprintf("DHI Harness v%s booting...", version))
	logger.Info("CONFIG", fmt.Sprintf("AI provider: %s (%s)", cfg.AI.Provider, cfg.AI.BaseURL))

	if err := logger.SetFile(filepath.Join(dhroot.DataDir(), "dhi.log")); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: log file: %v\n", err)
	}

	taskMgr := task.NewManager(dhroot.DataDir(), nil)

	router := intent.NewRouter(cfg.AI, logger, dhroot.DataDir(), dhroot.PipelinePath())
	learningEngine := learn.NewEngine(dhroot.DataDir())
	router.SetWeightProvider(learningEngine)

	pipeMgr := pipeline.NewManager(dhroot.PipelinePath())
	summ := summarizer.New(cfg.AI)
	exec := pipeline.NewExecutor(pipeMgr, summ, taskMgr, dhroot.DataDir(), dhroot.ScriptsDir())

	logger.Info("SYSTEM", fmt.Sprintf("Loaded %d pipelines", pipeMgr.PipelineCount()))
	logger.Info("LEARN", fmt.Sprintf("Learning engine loaded with %d pipeline stats", len(learningEngine.Stats())))

	aiClient := ai.NewClient(cfg.AI)
	settingsMgr := settings.NewManager(dhroot.DataDir(), aiClient, cfg.AI)
	if err := settingsMgr.FetchModels(); err != nil {
		logger.Warn("SETTINGS", fmt.Sprintf("Failed to fetch models: %v", err))
	}
	if cur := settingsMgr.CurrentModel(); cur != nil {
		router.SetModel(cur.Name)
		exec.SetModel(cur.Name)
		learningEngine.SetModel(cur.Name)
	}

	topicMgr := topic.NewManager(dhroot.DataDir())
	sysSvc := services.NewSystemService()
	todoMgr := todo.NewManager(dhroot.DataDir())
	rssMgr := rss.NewManager(dhroot.DataDir())
	rssMgr.SetSkipShorts(cfg.RSS.SkipShorts)

	// Pre-warm agent-browser daemon so first search isn't slowed by Chrome cold start
	go func() {
		if _, err := osexec.LookPath("agent-browser"); err == nil {
			_ = osexec.Command("bash", "-c", "agent-browser batch 'open about:blank' 'wait 100' 2>/dev/null").Run()
		}
	}()

	model := tui.NewModel(logger, taskMgr, router, exec, topicMgr, sysSvc, todoMgr, rssMgr, learningEngine, settingsMgr)

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
