use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::PathBuf;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PipelineConfig {
    pub pipelines: Vec<Pipeline>,
    #[serde(default)]
    pub internal: Vec<InternalPipeline>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Pipeline {
    pub name: String,
    #[serde(rename = "type")]
    pub pipeline_type: String,
    pub description: String,
    pub triggers: Vec<String>,
    pub params: Vec<ParamDefinition>,
    #[serde(rename = "output_format")]
    pub output_format: String,
    pub action: PipelineAction,
    pub summarizer: SummarizerConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InternalPipeline {
    pub name: String,
    #[serde(rename = "type")]
    pub pipeline_type: String,
    pub description: String,
    #[serde(default)]
    pub schedule: String,
    pub action: PipelineAction,
    pub summarizer: SummarizerConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ParamDefinition {
    pub name: String,
    #[serde(rename = "type")]
    pub param_type: String,
    pub required: bool,
    #[serde(default)]
    pub default: Option<String>,
    pub description: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum PipelineAction {
    #[serde(rename = "http")]
    Http { url: String, method: String },
    #[serde(rename = "command")]
    Command {
        command: String,
        #[serde(default)]
        args: Vec<String>,
    },
    #[serde(rename = "internal")]
    Internal { handler: String },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SummarizerConfig {
    pub enabled: bool,
    #[serde(default)]
    pub prompt: String,
}

#[derive(Debug, Clone)]
pub struct MatchedPipeline {
    pub pipeline: Pipeline,
    pub params: HashMap<String, String>,
}

pub struct PipelineManager {
    pipelines: Vec<Pipeline>,
    internal: Vec<InternalPipeline>,
    trigger_index: HashMap<String, usize>,
}

impl Clone for PipelineManager {
    fn clone(&self) -> Self {
        Self {
            pipelines: self.pipelines.clone(),
            internal: self.internal.clone(),
            trigger_index: self.trigger_index.clone(),
        }
    }
}

impl PipelineManager {
    pub fn load_from_file(path: &PathBuf) -> anyhow::Result<Self> {
        let content = std::fs::read_to_string(path)?;
        let config: PipelineConfig = serde_yaml::from_str(&content)?;

        let mut trigger_index = HashMap::new();
        for (idx, pipeline) in config.pipelines.iter().enumerate() {
            for trigger in &pipeline.triggers {
                trigger_index.insert(trigger.clone(), idx);
            }
        }

        Ok(Self {
            pipelines: config.pipelines,
            internal: config.internal,
            trigger_index,
        })
    }

    pub fn find_pipeline(&self, user_input: &str) -> Option<MatchedPipeline> {
        let input_lower = user_input.to_lowercase();

        for (trigger, &idx) in &self.trigger_index {
            if input_lower.contains(&trigger.to_lowercase()) {
                let pipeline = &self.pipelines[idx];
                let params = self.extract_params(user_input, &pipeline.params);
                return Some(MatchedPipeline {
                    pipeline: pipeline.clone(),
                    params,
                });
            }
        }

        None
    }

    fn extract_params(
        &self,
        user_input: &str,
        param_defs: &[ParamDefinition],
    ) -> HashMap<String, String> {
        let mut params = HashMap::new();
        let input_lower = user_input.to_lowercase();

        for def in param_defs {
            if def.name == "query" || def.name == "symbol" || def.name == "source" {
                let words: Vec<&str> = user_input.split_whitespace().collect();
                if let Some(val) = words
                    .iter()
                    .rposition(|w| w.chars().any(|c| c.is_uppercase()))
                {
                    params.insert(def.name.clone(), words[val].to_string());
                }
            } else if def.name == "action" {
                let action = if input_lower.contains("check") || input_lower.contains("info") {
                    "check"
                } else if input_lower.contains("buy") {
                    "buy"
                } else if input_lower.contains("sell") {
                    "sell"
                } else {
                    def.default.as_deref().unwrap_or("check")
                };
                params.insert(def.name.clone(), action.to_string());
            } else if def.param_type == "integer" || def.param_type == "float" {
                if let Some(num) = extract_number(user_input) {
                    params.insert(def.name.clone(), num);
                } else if def.required {
                    params.insert(def.name.clone(), def.default.clone().unwrap_or_default());
                }
            } else if def.required {
                params.insert(def.name.clone(), def.default.clone().unwrap_or_default());
            }
        }

        params
    }

    pub fn get_pipeline(&self, name: &str) -> Option<&Pipeline> {
        self.pipelines.iter().find(|p| p.name == name)
    }

    pub fn get_internal(&self, name: &str) -> Option<&InternalPipeline> {
        self.internal.iter().find(|p| p.name == name)
    }

    pub fn list_pipelines(&self) -> Vec<(String, String)> {
        self.pipelines
            .iter()
            .map(|p| (p.name.clone(), p.description.clone()))
            .collect()
    }
}

fn extract_number(input: &str) -> Option<String> {
    let words: Vec<&str> = input.split_whitespace().collect();
    for word in words.iter().rev() {
        let cleaned = word.trim_end_matches(|c| c == '.' || c == ',' || c == '$' || c == '₹');
        if cleaned.parse::<f64>().is_ok() {
            return Some(cleaned.to_string());
        }
    }
    None
}

impl Default for PipelineManager {
    fn default() -> Self {
        Self {
            pipelines: Vec::new(),
            internal: Vec::new(),
            trigger_index: HashMap::new(),
        }
    }
}

pub mod executor;
pub use executor::PipelineExecutor;
pub mod handlers;
