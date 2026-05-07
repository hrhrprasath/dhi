use crate::pipeline::handlers::Handlers;
use crate::pipeline::{MatchedPipeline, PipelineAction, PipelineManager};
use crate::summarizer::Summarizer;
use anyhow::Result;
use std::collections::HashMap;

#[derive(Clone)]
pub struct PipelineExecutor {
    pipeline_manager: PipelineManager,
    summarizer: Summarizer,
}

impl PipelineExecutor {
    pub fn new(pipeline_config_path: Option<&str>) -> Self {
        let config_path = pipeline_config_path.unwrap_or("config/pipelines.yaml");
        let pipeline_manager = PipelineManager::load_from_file(&std::path::PathBuf::from(
            config_path,
        ))
        .unwrap_or_else(|e| {
            eprintln!("Failed to load pipeline config: {}", e);
            PipelineManager::default()
        });

        Self {
            pipeline_manager,
            summarizer: Summarizer::new(),
        }
    }

    pub fn find_and_execute(&self, user_input: &str, classification: &str) -> Result<String> {
        let pipeline_name = self.classify_to_pipeline(classification);

        if let Some(pipeline) = self.pipeline_manager.get_pipeline(&pipeline_name) {
            self.execute_pipeline(pipeline, user_input)
        } else {
            Ok(format!(
                "No pipeline found for classification: {}",
                classification
            ))
        }
    }

    pub fn execute_from_matched(
        &self,
        matched: &MatchedPipeline,
        user_input: &str,
    ) -> Result<String> {
        self.execute_pipeline(&matched.pipeline, user_input)
    }

    fn classify_to_pipeline(&self, classification: &str) -> String {
        let type_map: HashMap<&str, &str> = [
            ("music", "music"),
            ("coding", "system"),
            ("portfolio", "portfolio"),
            ("stock", "portfolio"),
            ("rss", "rss"),
            ("feed", "rss"),
            ("search", "search"),
            ("expense", "expense"),
            ("health", "health"),
            ("system", "system"),
        ]
        .iter()
        .cloned()
        .collect();

        let lower = classification.to_lowercase();
        type_map
            .get(lower.as_str())
            .unwrap_or(&"system")
            .to_string()
    }

    fn execute_pipeline(
        &self,
        pipeline: &crate::pipeline::Pipeline,
        _user_input: &str,
    ) -> Result<String> {
        let raw_output = match &pipeline.action {
            PipelineAction::Http { url, method } => self.execute_http(url, method),
            PipelineAction::Command { command, args } => self.execute_command(command, args),
            PipelineAction::Internal { handler } => self.execute_internal(handler),
        };

        if pipeline.summarizer.enabled {
            let summary = tokio::runtime::Handle::current().block_on(self.summarizer.summarize(
                &raw_output,
                &pipeline.name,
                Some(&pipeline.summarizer.prompt),
            ));
            Ok(summary)
        } else {
            Ok(raw_output)
        }
    }

    fn execute_http(&self, url: &str, _method: &str) -> String {
        let output = std::process::Command::new("curl")
            .args(["-s", "--max-time", "10", url])
            .output();

        match output {
            Ok(o) => {
                if o.status.success() {
                    String::from_utf8_lossy(&o.stdout).to_string()
                } else {
                    format!("HTTP request failed: {}", o.status)
                }
            }
            Err(e) => format!("Failed to execute HTTP: {}", e),
        }
    }

    fn execute_command(&self, command: &str, args: &[String]) -> String {
        let output = std::process::Command::new(command).args(args).output();

        match output {
            Ok(o) => {
                if o.status.success() {
                    String::from_utf8_lossy(&o.stdout).to_string()
                } else {
                    format!("Command failed: {}", o.status)
                }
            }
            Err(e) => format!("Failed to execute command: {}", e),
        }
    }

    fn execute_internal(&self, handler: &str) -> String {
        match handler {
            "system_metrics" => Handlers::system_metrics(),
            "expense_log" => Handlers::expense_log("0", "misc", "no description"),
            "health_tracker" => Handlers::health_tracker("check", "all"),
            "history_summary" => Handlers::history_summary(Some(10)),
            "daily_brief" => Handlers::daily_brief(),
            _ => format!("Unknown handler: {}", handler),
        }
    }

    pub fn list_pipelines(&self) -> Vec<(String, String)> {
        self.pipeline_manager.list_pipelines()
    }

    pub fn get_pipeline_manager(&self) -> &PipelineManager {
        &self.pipeline_manager
    }
}

impl Default for PipelineExecutor {
    fn default() -> Self {
        Self::new(None)
    }
}
