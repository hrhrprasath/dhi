use crate::pipeline::handlers::Handlers;
use crate::pipeline::{MatchedPipeline, PipelineManager};
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

    pub fn find_and_execute(
        &self,
        event: &str,
        params: &Option<serde_json::Value>,
    ) -> Result<String> {
        let pipeline_name = self.event_to_pipeline(event);

        if let Some(pipeline) = self.pipeline_manager.get_pipeline(&pipeline_name) {
            let extracted_params = self.extract_params_from_json(params);

            if let Err(e) = self.validate_params(&extracted_params, &pipeline.params) {
                return Ok(format!("Parameter validation failed: {}", e));
            }

            let mut params_with_defaults = extracted_params;
            PipelineManager::apply_defaults(&mut params_with_defaults, &pipeline.params);

            if let Err(e) = self.validate_params(&params_with_defaults, &pipeline.params) {
                return Ok(format!("Parameter validation failed: {}", e));
            }

            self.execute_pipeline_with_params(pipeline, &params_with_defaults)
        } else {
            self.execute_unknown_fallback()
        }
    }

    pub fn execute_from_matched(
        &self,
        matched: &MatchedPipeline,
        _user_input: &str,
    ) -> Result<String> {
        self.execute_pipeline_sync(&matched.pipeline)
    }

    fn event_to_pipeline(&self, event: &str) -> String {
        let upper = event.to_uppercase();

        if upper == "UNKNOWN" {
            return "ollama_search".to_string();
        }

        if let Some(stripped) = upper.strip_prefix("EXECUTE_") {
            let name = stripped.to_lowercase();
            if self.pipeline_manager.get_pipeline(&name).is_some() {
                return name;
            }
        }

        "ollama_search".to_string()
    }

    fn extract_params_from_json(
        &self,
        params: &Option<serde_json::Value>,
    ) -> HashMap<String, String> {
        let mut result = HashMap::new();

        if let Some(val) = params {
            if let Some(obj) = val.as_object() {
                for (k, v) in obj {
                    result.insert(k.clone(), v.to_string().trim_matches('"').to_string());
                }
            }
        }

        result
    }

    fn validate_params(
        &self,
        params: &HashMap<String, String>,
        param_defs: &[crate::pipeline::ParamDefinition],
    ) -> Result<(), String> {
        let valid_names: std::collections::HashSet<String> =
            param_defs.iter().map(|d| d.name.clone()).collect();

        for (key, _) in params {
            if !valid_names.contains(key) {
                return Err(format!(
                    "Invalid parameter '{}'. Expected one of: {:?}",
                    key, valid_names
                ));
            }
        }

        for def in param_defs {
            if def.required && !params.contains_key(&def.name) {
                return Err(format!("Missing required parameter: {}", def.name));
            }
        }

        Ok(())
    }

    fn execute_unknown_fallback(&self) -> Result<String> {
        Ok("Unable to determine action: event unknown or required params missing".to_string())
    }

    fn execute_pipeline_with_params(
        &self,
        pipeline: &crate::pipeline::Pipeline,
        params: &HashMap<String, String>,
    ) -> Result<String> {
        use crate::pipeline::PipelineAction;

        if let PipelineAction::Http { url, method, auth } = &pipeline.action {
            let resolved_url = self.interpolate_params(url, params);
            let resolved_method = self.interpolate_params(method, params);
            let auth_header = auth.as_ref().map(|a| {
                let h = self.interpolate_params(&a.header, params);
                let v = self.interpolate_params(&a.value, params);
                (h, v)
            });
            let output = self.execute_http(&resolved_url, &resolved_method, auth_header);
            if pipeline.summarizer.is_enabled() && !output.is_empty() {
                let summary = tokio::task::block_in_place(|| {
                    tokio::runtime::Handle::current().block_on(self.summarizer.summarize(
                        &output,
                        &pipeline.name,
                        Some(&pipeline.summarizer.prompt),
                    ))
                });
                return Ok(summary);
            }
            return Ok(output);
        }

        if let PipelineAction::Exec { command, args } = &pipeline.action {
            let resolved_command = self.interpolate_params(command, params);
            let resolved_args: Vec<String> = args
                .iter()
                .map(|a| self.interpolate_params(a, params))
                .collect();
            let output = self.execute_exec(&resolved_command, &resolved_args);
            if pipeline.summarizer.is_enabled() && !output.is_empty() {
                let summary = tokio::task::block_in_place(|| {
                    tokio::runtime::Handle::current().block_on(self.summarizer.summarize(
                        &output,
                        &pipeline.name,
                        Some(&pipeline.summarizer.prompt),
                    ))
                });
                return Ok(summary);
            }
            return Ok(output);
        }

        if let PipelineAction::Bash { script } = &pipeline.action {
            let resolved_script = self.interpolate_params(script, params);
            let output = self.execute_bash(&resolved_script);
            if pipeline.summarizer.is_enabled() && !output.is_empty() {
                let summary = tokio::task::block_in_place(|| {
                    tokio::runtime::Handle::current().block_on(self.summarizer.summarize(
                        &output,
                        &pipeline.name,
                        Some(&pipeline.summarizer.prompt),
                    ))
                });
                return Ok(summary);
            }
            return Ok(output);
        }

        if let PipelineAction::Open { target } = &pipeline.action {
            let resolved_target = self.interpolate_params(target, params);
            let output = self.execute_open(&resolved_target);
            return Ok(output);
        }

        if let PipelineAction::Internal { handler } = &pipeline.action {
            return self.execute_internal_with_params(handler, params);
        }

        Ok("Unknown action type".to_string())
    }

    fn interpolate_params(&self, template: &str, params: &HashMap<String, String>) -> String {
        let mut result = template.to_string();
        for (key, value) in params {
            let placeholder = format!("{{{}}}", key);
            result = result.replace(&placeholder, value);
        }
        result
    }

    fn execute_internal_with_params(
        &self,
        handler: &str,
        params: &HashMap<String, String>,
    ) -> Result<String> {
        match handler {
            "system_metrics" => Ok(Handlers::system_metrics()),
            "expense_log" => {
                let amount = params.get("amount").cloned().unwrap_or_default();
                let category = params
                    .get("category")
                    .cloned()
                    .unwrap_or_else(|| "misc".to_string());
                let description = params.get("description").cloned().unwrap_or_default();
                Ok(Handlers::expense_log(&amount, &category, &description))
            }
            "health_tracker" => {
                let action = params
                    .get("action")
                    .cloned()
                    .unwrap_or_else(|| "check".to_string());
                let metric = params.get("metric").cloned().unwrap_or_default();
                Ok(Handlers::health_tracker(&action, &metric))
            }
            "memory_grep" => {
                let query = params.get("query").cloned().unwrap_or_default();
                let date_filter = params
                    .get("date_filter")
                    .cloned()
                    .unwrap_or_else(|| "all".to_string());
                let depth = params
                    .get("depth")
                    .cloned()
                    .unwrap_or_else(|| "1".to_string());
                Ok(Handlers::memory_grep(&query, &date_filter, &depth))
            }
            "conversation_continue" => {
                let topic = params.get("topic").cloned().unwrap_or_default();
                let last_context = params.get("last_context").cloned().unwrap_or_default();
                Ok(Handlers::conversation_continue(&topic, &last_context))
            }
            _ => Ok(format!("Unknown handler: {}", handler)),
        }
    }

    fn execute_pipeline_sync(&self, pipeline: &crate::pipeline::Pipeline) -> Result<String> {
        use crate::pipeline::PipelineAction;

        let raw_output = match &pipeline.action {
            PipelineAction::Http { url, method, auth } => {
                let auth_header = auth.as_ref().map(|a| (a.header.clone(), a.value.clone()));
                self.execute_http(url, method, auth_header)
            }
            PipelineAction::Exec { command, args } => self.execute_exec(command, args),
            PipelineAction::Bash { script } => self.execute_bash(script),
            PipelineAction::Open { target } => self.execute_open(target),
            PipelineAction::Internal { handler } => self.execute_internal(handler),
        };

        if pipeline.summarizer.is_enabled() && !raw_output.is_empty() {
            let summary = tokio::task::block_in_place(|| {
                tokio::runtime::Handle::current().block_on(self.summarizer.summarize(
                    &raw_output,
                    &pipeline.name,
                    Some(&pipeline.summarizer.prompt),
                ))
            });
            Ok(summary)
        } else {
            Ok(raw_output)
        }
    }

    fn execute_http(&self, url: &str, method: &str, auth: Option<(String, String)>) -> String {
        let mut cmd = std::process::Command::new("curl");
        cmd.args(["-s", "--max-time", "10", "-X", method]);

        if let Some((header, value)) = auth {
            cmd.args(["-H", &format!("{}: {}", header, value)]);
        }

        cmd.arg(url);

        let output = cmd.output();

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

    fn execute_exec(&self, command: &str, args: &[String]) -> String {
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

    fn execute_bash(&self, script: &str) -> String {
        let output = std::process::Command::new("bash")
            .args(["-c", script])
            .output();

        match output {
            Ok(o) => {
                if o.status.success() {
                    String::from_utf8_lossy(&o.stdout).to_string()
                } else {
                    format!(
                        "Script failed: {}\n{}",
                        o.status,
                        String::from_utf8_lossy(&o.stderr)
                    )
                }
            }
            Err(e) => format!("Failed to execute script: {}", e),
        }
    }

    fn execute_open(&self, target: &str) -> String {
        if target.starts_with("http://") || target.starts_with("https://") {
            let output = std::process::Command::new("open").arg(target).output();

            match output {
                Ok(o) if o.status.success() => format!("Opened URL: {}", target),
                Ok(o) => format!("Failed to open URL: {}", o.status),
                Err(e) => format!("Failed to open URL: {}", e),
            }
        } else {
            let output = std::process::Command::new("open")
                .args(["-a", target])
                .output();

            match output {
                Ok(o) if o.status.success() => format!("Opened app: {}", target),
                Ok(o) => format!("Failed to open app: {}", o.status),
                Err(e) => format!("Failed to open app: {}", e),
            }
        }
    }

    fn execute_internal(&self, handler: &str) -> String {
        match handler {
            "system_metrics" => Handlers::system_metrics(),
            "expense_log" => Handlers::expense_log("0", "misc", "no description"),
            "health_tracker" => Handlers::health_tracker("check", "all"),
            "memory_grep" => Handlers::memory_grep("", "all", "1"),
            "conversation_continue" => Handlers::conversation_continue("", ""),
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_execute_expense() {
        let executor = PipelineExecutor::new(Some("config/pipelines.yaml"));
        let params = serde_json::json!({"amount": "50.00", "category": "food"});
        let result = executor.find_and_execute("EXECUTE_EXPENSE", &Some(params));
        println!("Expense result: {:?}", result);
        assert!(result.is_ok());
    }

    #[test]
    fn test_execute_health() {
        let executor = PipelineExecutor::new(Some("config/pipelines.yaml"));
        let result = executor.find_and_execute("EXECUTE_HEALTH", &None);
        println!("Health result: {:?}", result);
        assert!(result.is_ok());
    }

    #[test]
    fn test_execute_history() {
        let executor = PipelineExecutor::new(Some("config/pipelines.yaml"));
        let params = serde_json::json!({"query": "test", "depth": "1"});
        let result = executor.find_and_execute("EXECUTE_HISTORY", &Some(params));
        println!("History result: {:?}", result);
        assert!(result.is_ok());
    }

    #[test]
    fn test_all_pipeline_names() {
        let executor = PipelineExecutor::new(Some("config/pipelines.yaml"));
        let pipelines = executor.list_pipelines();
        println!("Available pipelines: {:?}", pipelines);
        assert!(pipelines.len() >= 7);
    }
}
