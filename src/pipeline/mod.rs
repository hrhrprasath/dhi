use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::PathBuf;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PipelineConfig {
    pub pipelines: Vec<Pipeline>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Pipeline {
    pub name: String,
    pub description: String,
    pub params: Vec<ParamDefinition>,
    pub action: PipelineAction,
    pub summarizer: SummarizerConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ParamDefinition {
    pub name: String,
    pub required: bool,
    #[serde(default)]
    pub default: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum PipelineAction {
    #[serde(rename = "http")]
    Http {
        url: String,
        method: String,
        #[serde(default)]
        auth: Option<HttpAuth>,
    },
    #[serde(rename = "exec")]
    Exec {
        command: String,
        #[serde(default)]
        args: Vec<String>,
    },
    #[serde(rename = "bash")]
    Bash { script: String },
    #[serde(rename = "open")]
    Open { target: String },
    #[serde(rename = "internal")]
    Internal { handler: String },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HttpAuth {
    pub header: String,
    pub value: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SummarizerConfig {
    #[serde(default)]
    pub prompt: String,
}

impl SummarizerConfig {
    pub fn is_enabled(&self) -> bool {
        !self.prompt.is_empty()
    }
}

#[derive(Debug, Clone)]
pub struct MatchedPipeline {
    pub pipeline: Pipeline,
    pub params: HashMap<String, String>,
}

pub struct PipelineManager {
    pipelines: Vec<Pipeline>,
}

impl Clone for PipelineManager {
    fn clone(&self) -> Self {
        Self {
            pipelines: self.pipelines.clone(),
        }
    }
}

impl PipelineManager {
    pub fn load_from_file(path: &PathBuf) -> anyhow::Result<Self> {
        let content = std::fs::read_to_string(path)?;
        let config: PipelineConfig = serde_yaml::from_str(&content)?;

        Ok(Self {
            pipelines: config.pipelines,
        })
    }

    pub fn get_pipeline_names(&self) -> Vec<String> {
        self.pipelines.iter().map(|p| p.name.clone()).collect()
    }

    pub fn get_pipeline_descriptions(&self) -> Vec<(String, String)> {
        self.pipelines
            .iter()
            .map(|p| (p.name.clone(), p.description.clone()))
            .collect()
    }

    pub fn find_pipeline(&self, name: &str) -> Option<&Pipeline> {
        self.pipelines
            .iter()
            .find(|p| p.name.to_lowercase() == name.to_lowercase())
    }

    pub fn apply_defaults(params: &mut HashMap<String, String>, param_defs: &[ParamDefinition]) {
        for def in param_defs {
            if let Some(default) = &def.default {
                params
                    .entry(def.name.clone())
                    .or_insert_with(|| default.clone());
            }
        }
    }

    pub fn get_pipeline(&self, name: &str) -> Option<&Pipeline> {
        self.pipelines.iter().find(|p| p.name == name)
    }

    pub fn list_pipelines(&self) -> Vec<(String, String)> {
        self.pipelines
            .iter()
            .map(|p| (p.name.clone(), p.description.clone()))
            .collect()
    }
}

impl Default for PipelineManager {
    fn default() -> Self {
        Self {
            pipelines: Vec::new(),
        }
    }
}

pub mod executor;
pub use executor::PipelineExecutor;
pub mod handlers;
